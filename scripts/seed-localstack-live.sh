#!/usr/bin/env bash
# seed-localstack-live.sh — Continuously sends log events to LocalStack CloudWatch log groups.
#
# Prerequisites: make local-up && make local-seed
#
# Usage:
#   make local-seed-live          # Terminal 1: generate events
#   make local-run                # Terminal 2: tail them in the TUI
#
# Press Ctrl+C to stop.

set -euo pipefail

ENDPOINT="${AWS_ENDPOINT_URL:-http://localhost:4566}"
REGION="${AWS_REGION:-us-east-1}"

aws() {
  command aws --endpoint-url "$ENDPOINT" --region "$REGION" "$@"
}

# Log groups seeded by seed-localstack.sh
# Note: avoid "GROUPS" — it's a reserved bash variable (user's Unix group IDs).
LOG_GROUPS=("/app/web" "/app/api" "/app/worker" "/aws/lambda/my-function")
STREAM="stream-1"

# Sample data pools
LEVELS=("info" "info" "info" "warn" "error" "debug")
PATHS_WEB=("GET /dashboard" "GET /users" "POST /login" "GET /settings" "PUT /users/123" "DELETE /sessions/abc" "GET /health")
PATHS_API=("GET /api/v1/users" "POST /api/v1/orders" "GET /api/v1/products" "PUT /api/v1/cart" "GET /api/v1/search?q=test" "DELETE /api/v1/cache")
WORKER_MSGS=("processing job" "job completed" "retrying failed task" "queue drained" "batch processed" "sending notification" "compressing archive")
LAMBDA_MSGS=("invocation started" "cold start detected" "execution completed" "timeout warning" "memory usage high" "response sent")
STATUS_CODES=(200 200 200 200 201 204 301 400 404 500 502)

# rand_element ARRAY_NAME — prints a random element from the named array.
# Compatible with bash 3.2 (no nameref needed).
rand_element() {
  local tmp="$1[@]"
  local vals=("${!tmp}")
  echo "${vals[$((RANDOM % ${#vals[@]}))]}"
}

rand_range() {
  echo $(( $1 + RANDOM % ($2 - $1 + 1) ))
}

# Fetch the current sequence token for a log stream (needed by put-log-events).
get_sequence_token() {
  local group=$1
  local token
  token=$(aws logs describe-log-streams \
    --log-group-name "$group" \
    --log-stream-name-prefix "$STREAM" \
    --query 'logStreams[0].uploadSequenceToken' \
    --output text 2>/dev/null || echo "")
  # "None" is returned when no token exists yet (first write)
  if [[ "$token" == "None" || -z "$token" ]]; then
    echo ""
  else
    echo "$token"
  fi
}

put_events() {
  local group=$1
  local events=$2
  local token
  token=$(get_sequence_token "$group")

  if [[ -n "$token" ]]; then
    aws logs put-log-events \
      --log-group-name "$group" \
      --log-stream-name "$STREAM" \
      --log-events "$events" \
      --sequence-token "$token" > /dev/null 2>&1
  else
    aws logs put-log-events \
      --log-group-name "$group" \
      --log-stream-name "$STREAM" \
      --log-events "$events" > /dev/null 2>&1
  fi
}

build_event() {
  local group=$1
  local ts=$2
  local level msg status duration request_id

  level=$(rand_element LEVELS)
  status=$(rand_element STATUS_CODES)
  duration=$(rand_range 1 800)
  request_id=$(printf '%04x-%04x' $((RANDOM)) $((RANDOM)))

  case "$group" in
    /app/web)
      msg=$(rand_element PATHS_WEB)
      ;;
    /app/api)
      msg=$(rand_element PATHS_API)
      ;;
    /app/worker)
      msg=$(rand_element WORKER_MSGS)
      ;;
    /aws/lambda/*)
      msg=$(rand_element LAMBDA_MSGS)
      ;;
  esac

  local payload="{\\\"level\\\":\\\"$level\\\",\\\"msg\\\":\\\"$msg\\\",\\\"status\\\":$status,\\\"duration_ms\\\":$duration,\\\"request_id\\\":\\\"$request_id\\\"}"
  echo "{\"timestamp\":$ts,\"message\":\"$payload\"}"
}

iteration=0

echo "Sending live log events to LocalStack (Ctrl+C to stop)..."
echo "  Groups: ${LOG_GROUPS[*]}"
echo ""

while true; do
  iteration=$((iteration + 1))
  now_ms=$(( $(date +%s) * 1000 ))

  # Pick a random subset of groups (1 to all 4)
  count=$(rand_range 1 ${#LOG_GROUPS[@]})
  # Shuffle: swap each element with a random position
  shuffled=("${LOG_GROUPS[@]}")
  for ((i = ${#shuffled[@]} - 1; i > 0; i--)); do
    j=$((RANDOM % (i + 1)))
    tmp="${shuffled[$i]}"
    shuffled[$i]="${shuffled[$j]}"
    shuffled[$j]="$tmp"
  done
  selected=("${shuffled[@]:0:$count}")

  for group in "${selected[@]}"; do
    # Send 1-3 events per group
    num_events=$(rand_range 1 3)
    events=""
    for ((e = 0; e < num_events; e++)); do
      # Offset each event by a few ms so timestamps are unique
      ev=$(build_event "$group" $((now_ms + e)))
      if [[ -n "$events" ]]; then
        events="$events,$ev"
      else
        events="$ev"
      fi
    done

    put_events "$group" "[$events]"
  done

  # Brief status
  printf "\r[#%d] Sent events to %d group(s): %s" "$iteration" "$count" "${selected[*]}"

  # Sleep 2-3 seconds
  sleep $(rand_range 2 3)
done
