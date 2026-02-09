#!/usr/bin/env bash
# seed-localstack.sh — Populates LocalStack with test data for sacha development.
#
# Data volumes exceed default page sizes to exercise lazy loading / pagination:
#   S3 ListObjectsV2     — page 1000 → 1200+ objects in sacha-data
#   CloudWatch LogGroups — page 50   → 60 log groups
#   DynamoDB ListTables  — page 100  → 120 tables
#   DynamoDB Scan        — page 25   → 80 items in Users table
#   Lambda ListFunctions — page 50   → 65 functions
#   SQS ListQueues       — page 50   → 60 queues
#   SSM DescribeParams   — page 50   → 65 parameters

set -euo pipefail

ENDPOINT="${AWS_ENDPOINT_URL:-http://localhost:4566}"
REGION="${AWS_REGION:-us-east-1}"

aws() {
  command aws --endpoint-url "$ENDPOINT" --region "$REGION" --no-cli-pager "$@"
}

# progress BAR_CURRENT BAR_TOTAL LABEL — inline progress indicator
progress() {
  local current=$1 total=$2 label=$3
  printf "\r  %s %d/%d" "$label" "$current" "$total"
  if [ "$current" -eq "$total" ]; then printf "\n"; fi
}

echo "=== Seeding S3 ==="

# Create 5 buckets
for bucket in sacha-logs sacha-data sacha-assets sacha-backups sacha-config; do
  aws s3api create-bucket --bucket "$bucket" > /dev/null 2>&1 || true
done
echo "  5 buckets created"

# sacha-logs: nested folder structure with JSON/text files
for folder in app/web app/api app/worker system; do
  for i in $(seq 1 5); do
    echo "{\"level\":\"info\",\"msg\":\"log entry $i\",\"service\":\"$folder\"}" | \
      aws s3 cp - "s3://sacha-logs/$folder/log-$(printf '%03d' "$i").json" --quiet
  done
done
echo "access log data" | aws s3 cp - "s3://sacha-logs/access.log" --quiet
echo "error log data" | aws s3 cp - "s3://sacha-logs/error.log" --quiet
echo "  sacha-logs: 22 objects"

# sacha-data: 1200+ objects across folders (triggers pagination)
count=0
total=1200
for folder in users orders products analytics; do
  for i in $(seq 1 300); do
    echo "{\"id\":$i,\"folder\":\"$folder\"}" | \
      aws s3 cp - "s3://sacha-data/$folder/item-$(printf '%04d' "$i").json" --quiet
    count=$((count + 1))
    progress "$count" "$total" "sacha-data:"
  done
done

# sacha-assets: images/media folder structure
for folder in images/icons images/banners videos documents; do
  for i in $(seq 1 5); do
    echo "binary-placeholder-$i" | \
      aws s3 cp - "s3://sacha-assets/$folder/file-$(printf '%03d' "$i").bin" --quiet
  done
done
echo "  sacha-assets: 20 objects"

# sacha-backups: date-partitioned folders
for month in 01 02 03 04 05 06; do
  for day in 01 15; do
    echo "{\"backup\":true,\"date\":\"2025-$month-$day\"}" | \
      aws s3 cp - "s3://sacha-backups/2025/$month/$day/backup.json" --quiet
  done
done
echo "  sacha-backups: 12 objects"

# sacha-config: small config files
echo '{"app":"sacha","version":"1.0"}' | aws s3 cp - "s3://sacha-config/app.json" --quiet
echo '{"database":{"host":"localhost","port":5432}}' | aws s3 cp - "s3://sacha-config/database.json" --quiet
echo '{"logging":{"level":"info"}}' | aws s3 cp - "s3://sacha-config/logging.json" --quiet
echo "  sacha-config: 3 objects"

echo "=== Seeding CloudWatch Logs ==="

# 4 primary log groups with 10+ events each
for group in /app/web /app/api /app/worker /aws/lambda/my-function; do
  aws logs create-log-group --log-group-name "$group" 2>/dev/null || true
  aws logs create-log-stream --log-group-name "$group" --log-stream-name "stream-1" 2>/dev/null || true

  ts=$(($(date +%s) * 1000 - 600000))
  events=""
  for i in $(seq 1 12); do
    ev="{\"timestamp\":$((ts + i * 1000)),\"message\":\"{\\\"level\\\":\\\"info\\\",\\\"msg\\\":\\\"request $i\\\",\\\"service\\\":\\\"${group##*/}\\\",\\\"duration_ms\\\":$((RANDOM % 500))}\"}"
    if [ -n "$events" ]; then
      events="$events,$ev"
    else
      events="$ev"
    fi
  done
  aws logs put-log-events \
    --log-group-name "$group" \
    --log-stream-name "stream-1" \
    --log-events "[$events]" > /dev/null
done
echo "  4 primary log groups (12 events each)"

# 56 additional log groups for pagination (page size 50)
for i in $(seq 1 56); do
  name="/service/svc-$(printf '%02d' "$i")"
  aws logs create-log-group --log-group-name "$name" 2>/dev/null || true
  aws logs create-log-stream --log-group-name "$name" --log-stream-name "stream-1" 2>/dev/null || true

  ts=$(($(date +%s) * 1000 - 60000))
  events=""
  for j in 1 2 3; do
    ev="{\"timestamp\":$((ts + j * 1000)),\"message\":\"{\\\"level\\\":\\\"info\\\",\\\"msg\\\":\\\"event $j\\\",\\\"service\\\":\\\"svc-$(printf '%02d' "$i")\\\"}\"}"
    if [ -n "$events" ]; then
      events="$events,$ev"
    else
      events="$ev"
    fi
  done
  aws logs put-log-events \
    --log-group-name "$name" \
    --log-stream-name "stream-1" \
    --log-events "[$events]" > /dev/null
  progress "$i" 56 "pagination groups:"
done
echo "  60 log groups total"

echo "=== Seeding DynamoDB ==="

# Users table — 80 items (hash key), tests 4+ scan pages (page size 25)
aws dynamodb create-table \
  --table-name Users \
  --attribute-definitions AttributeName=userId,AttributeType=S \
  --key-schema AttributeName=userId,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST > /dev/null 2>&1 || true

for i in $(seq 1 80); do
  aws dynamodb put-item --table-name Users --item \
    "{\"userId\":{\"S\":\"user-$(printf '%03d' "$i")\"},\"name\":{\"S\":\"User $i\"},\"email\":{\"S\":\"user$i@example.com\"},\"age\":{\"N\":\"$((20 + RANDOM % 50))\"},\"active\":{\"BOOL\":$([ $((i % 5)) -eq 0 ] && echo false || echo true)}}" \
    > /dev/null
  progress "$i" 80 "Users items:"
done

# Orders table — 50 items (hash+range key), tests 2+ scan pages
aws dynamodb create-table \
  --table-name Orders \
  --attribute-definitions \
    AttributeName=orderId,AttributeType=S \
    AttributeName=createdAt,AttributeType=S \
  --key-schema \
    AttributeName=orderId,KeyType=HASH \
    AttributeName=createdAt,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST > /dev/null 2>&1 || true

for i in $(seq 1 50); do
  aws dynamodb put-item --table-name Orders --item \
    "{\"orderId\":{\"S\":\"order-$(printf '%03d' "$i")\"},\"createdAt\":{\"S\":\"2025-01-$(printf '%02d' $((i % 28 + 1)))T10:00:00Z\"},\"total\":{\"N\":\"$((RANDOM % 10000))\"},\"status\":{\"S\":\"$([ $((i % 3)) -eq 0 ] && echo shipped || echo pending)\"}}" \
    > /dev/null
  progress "$i" 50 "Orders items:"
done

# Sessions table — 30 items
aws dynamodb create-table \
  --table-name Sessions \
  --attribute-definitions AttributeName=sessionId,AttributeType=S \
  --key-schema AttributeName=sessionId,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST > /dev/null 2>&1 || true

for i in $(seq 1 30); do
  aws dynamodb put-item --table-name Sessions --item \
    "{\"sessionId\":{\"S\":\"sess-$(printf '%03d' "$i")\"},\"userId\":{\"S\":\"user-$(printf '%03d' $((i % 80 + 1)))\"},\"ttl\":{\"N\":\"$(($(date +%s) + 3600))\"}}" \
    > /dev/null
  progress "$i" 30 "Sessions items:"
done

# 117 additional empty tables for list pagination (page size 100)
for i in $(seq 1 117); do
  aws dynamodb create-table \
    --table-name "table-$(printf '%03d' "$i")" \
    --attribute-definitions AttributeName=id,AttributeType=S \
    --key-schema AttributeName=id,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST > /dev/null 2>&1 || true
  progress "$i" 117 "pagination tables:"
done
echo "  120 tables total (Users: 80, Orders: 50, Sessions: 30)"

echo "=== Seeding Lambda ==="

# Create a minimal zip for Lambda deployments
TMPDIR=$(mktemp -d)
echo 'def handler(event, context): return {"statusCode": 200}' > "$TMPDIR/lambda_function.py"
(cd "$TMPDIR" && zip -q function.zip lambda_function.py)

# 5 primary functions with varied runtimes
aws lambda create-function \
  --function-name my-api-handler \
  --runtime python3.12 \
  --handler lambda_function.handler \
  --role arn:aws:iam::000000000000:role/lambda-role \
  --zip-file "fileb://$TMPDIR/function.zip" \
  --environment "Variables={API_URL=https://api.example.com,LOG_LEVEL=info}" \
  > /dev/null 2>&1 || true

aws lambda create-function \
  --function-name data-processor \
  --runtime python3.12 \
  --handler lambda_function.handler \
  --role arn:aws:iam::000000000000:role/lambda-role \
  --zip-file "fileb://$TMPDIR/function.zip" \
  --environment "Variables={BATCH_SIZE=100,QUEUE_URL=https://sqs.example.com/queue}" \
  --timeout 300 --memory-size 512 \
  > /dev/null 2>&1 || true

# Node.js runtime
echo 'exports.handler = async (event) => ({ statusCode: 200 });' > "$TMPDIR/index.js"
(cd "$TMPDIR" && zip -q node-function.zip index.js)

aws lambda create-function \
  --function-name event-listener \
  --runtime nodejs20.x \
  --handler index.handler \
  --role arn:aws:iam::000000000000:role/lambda-role \
  --zip-file "fileb://$TMPDIR/node-function.zip" \
  --environment "Variables={EVENT_BUS=default}" \
  > /dev/null 2>&1 || true

aws lambda create-function \
  --function-name notification-sender \
  --runtime nodejs20.x \
  --handler index.handler \
  --role arn:aws:iam::000000000000:role/lambda-role \
  --zip-file "fileb://$TMPDIR/node-function.zip" \
  --timeout 30 --memory-size 256 \
  > /dev/null 2>&1 || true

aws lambda create-function \
  --function-name health-checker \
  --runtime python3.12 \
  --handler lambda_function.handler \
  --role arn:aws:iam::000000000000:role/lambda-role \
  --zip-file "fileb://$TMPDIR/function.zip" \
  --timeout 10 --memory-size 128 \
  > /dev/null 2>&1 || true

echo "  5 primary functions"

# 60 additional functions for list pagination (page size 50)
for i in $(seq 1 60); do
  aws lambda create-function \
    --function-name "fn-$(printf '%02d' "$i")" \
    --runtime python3.12 \
    --handler lambda_function.handler \
    --role arn:aws:iam::000000000000:role/lambda-role \
    --zip-file "fileb://$TMPDIR/function.zip" \
    > /dev/null 2>&1 || true
  progress "$i" 60 "pagination functions:"
done
echo "  65 functions total"

rm -rf "$TMPDIR"

echo "=== Seeding SQS ==="

# 5 primary queues with messages
for queue in order-processing notification-dispatch email-outbox event-ingestion audit-trail; do
  aws sqs create-queue --queue-name "$queue" > /dev/null 2>&1 || true
done

# 1 FIFO queue
aws sqs create-queue --queue-name "payment-processing.fifo" \
  --attributes '{"FifoQueue":"true","ContentBasedDeduplication":"true"}' \
  > /dev/null 2>&1 || true

# 1 queue with dead-letter configuration
DLQ_ARN=$(aws sqs get-queue-attributes \
  --queue-url "$ENDPOINT/000000000000/audit-trail" \
  --attribute-names QueueArn --query 'Attributes.QueueArn' --output text 2>/dev/null || echo "")
if [ -n "$DLQ_ARN" ] && [ "$DLQ_ARN" != "None" ]; then
  aws sqs create-queue --queue-name "retry-queue" \
    --attributes "{\"RedrivePolicy\":\"{\\\"deadLetterTargetArn\\\":\\\"$DLQ_ARN\\\",\\\"maxReceiveCount\\\":\\\"3\\\"}\"}" \
    > /dev/null 2>&1 || true
else
  aws sqs create-queue --queue-name "retry-queue" > /dev/null 2>&1 || true
fi
echo "  7 primary queues created"

# Send messages to primary queues so peek works
QUEUE_BASE_URL="$ENDPOINT/000000000000"
for i in $(seq 1 10); do
  aws sqs send-message --queue-url "$QUEUE_BASE_URL/order-processing" \
    --message-body "{\"orderId\":\"ord-$(printf '%03d' "$i")\",\"amount\":$((RANDOM % 10000)),\"status\":\"pending\",\"customer\":\"cust-$(printf '%02d' $((i % 20 + 1)))\"}" \
    > /dev/null 2>&1
  aws sqs send-message --queue-url "$QUEUE_BASE_URL/notification-dispatch" \
    --message-body "{\"type\":\"email\",\"to\":\"user$i@example.com\",\"subject\":\"Order update\",\"template\":\"order-confirmation\"}" \
    > /dev/null 2>&1
  aws sqs send-message --queue-url "$QUEUE_BASE_URL/event-ingestion" \
    --message-body "{\"eventType\":\"page_view\",\"userId\":\"user-$(printf '%03d' "$i")\",\"page\":\"/products/$i\",\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" \
    > /dev/null 2>&1
done
echo "  30 messages sent to primary queues"

# 53 additional queues for list pagination (page size 50)
for i in $(seq 1 53); do
  aws sqs create-queue --queue-name "queue-$(printf '%02d' "$i")" > /dev/null 2>&1 || true
  progress "$i" 53 "pagination queues:"
done
echo "  60 queues total"

echo "=== Seeding SSM ==="

# Application config parameters under /app/
aws ssm put-parameter --name "/app/name" --value "sacha" --type String > /dev/null 2>&1 || true
aws ssm put-parameter --name "/app/version" --value "1.0.0" --type String > /dev/null 2>&1 || true
aws ssm put-parameter --name "/app/environment" --value "development" --type String > /dev/null 2>&1 || true
aws ssm put-parameter --name "/app/log-level" --value "info" --type String > /dev/null 2>&1 || true
aws ssm put-parameter --name "/app/feature-flags" --value "dark-mode,notifications,beta-ui" --type StringList > /dev/null 2>&1 || true

# Database config under /database/
aws ssm put-parameter --name "/database/host" --value "db.example.internal" --type String > /dev/null 2>&1 || true
aws ssm put-parameter --name "/database/port" --value "5432" --type String > /dev/null 2>&1 || true
aws ssm put-parameter --name "/database/name" --value "sacha_prod" --type String > /dev/null 2>&1 || true
aws ssm put-parameter --name "/database/username" --value "sacha_app" --type String > /dev/null 2>&1 || true
aws ssm put-parameter --name "/database/password" --value "s3cret-db-pass!" --type SecureString > /dev/null 2>&1 || true
aws ssm put-parameter --name "/database/max-connections" --value "100" --type String > /dev/null 2>&1 || true
aws ssm put-parameter --name "/database/ssl-mode" --value "require" --type String > /dev/null 2>&1 || true

# API keys under /secrets/
aws ssm put-parameter --name "/secrets/api-key" --value "sk-test-abc123def456" --type SecureString > /dev/null 2>&1 || true
aws ssm put-parameter --name "/secrets/jwt-secret" --value "super-secret-jwt-signing-key" --type SecureString > /dev/null 2>&1 || true
aws ssm put-parameter --name "/secrets/encryption-key" --value "aes256-key-placeholder" --type SecureString > /dev/null 2>&1 || true

# Service endpoints under /services/
for svc in users orders payments notifications analytics search; do
  aws ssm put-parameter --name "/services/$svc/url" --value "https://$svc.internal.example.com" --type String > /dev/null 2>&1 || true
  aws ssm put-parameter --name "/services/$svc/timeout" --value "$((RANDOM % 30 + 5))" --type String > /dev/null 2>&1 || true
  aws ssm put-parameter --name "/services/$svc/retries" --value "$((RANDOM % 5 + 1))" --type String > /dev/null 2>&1 || true
done
echo "  38 primary parameters created"

# 27 additional parameters under /config/ for pagination (page size 50)
for i in $(seq 1 27); do
  aws ssm put-parameter \
    --name "/config/param-$(printf '%02d' "$i")" \
    --value "value-$i" \
    --type String > /dev/null 2>&1 || true
  progress "$i" 27 "pagination params:"
done
echo "  65 parameters total"

echo ""
echo "=== Seeding complete ==="
echo "Run sacha against LocalStack:"
echo "  make local-run"
