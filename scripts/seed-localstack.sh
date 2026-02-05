#!/usr/bin/env bash
# seed-localstack.sh — Populates LocalStack with test data for sacha development.
#
# Data volumes exceed default page sizes to exercise lazy loading / pagination:
#   S3 ListObjectsV2     — page 1000 → 1200+ objects in sacha-data
#   CloudWatch LogGroups — page 50   → 60 log groups
#   DynamoDB ListTables  — page 100  → 120 tables
#   DynamoDB Scan        — page 25   → 80 items in Users table
#   Lambda ListFunctions — page 50   → 65 functions

set -euo pipefail

ENDPOINT="${AWS_ENDPOINT_URL:-http://localhost:4566}"
REGION="${AWS_REGION:-us-east-1}"

aws() {
  command aws --endpoint-url "$ENDPOINT" --region "$REGION" "$@"
}

echo "=== Seeding S3 ==="

# Create 5 buckets
for bucket in sacha-logs sacha-data sacha-assets sacha-backups sacha-config; do
  aws s3api create-bucket --bucket "$bucket" 2>/dev/null || true
done

# sacha-logs: nested folder structure with JSON/text files
for folder in app/web app/api app/worker system; do
  for i in $(seq 1 5); do
    echo "{\"level\":\"info\",\"msg\":\"log entry $i\",\"service\":\"$folder\"}" | \
      aws s3 cp - "s3://sacha-logs/$folder/log-$(printf '%03d' "$i").json"
  done
done
echo "access log data" | aws s3 cp - "s3://sacha-logs/access.log"
echo "error log data" | aws s3 cp - "s3://sacha-logs/error.log"

# sacha-data: 1200+ objects across folders (triggers pagination)
for folder in users orders products analytics; do
  for i in $(seq 1 300); do
    echo "{\"id\":$i,\"folder\":\"$folder\"}" | \
      aws s3 cp - "s3://sacha-data/$folder/item-$(printf '%04d' "$i").json" --quiet
  done
done
echo "Seeded 1200 objects in sacha-data"

# sacha-assets: images/media folder structure
for folder in images/icons images/banners videos documents; do
  for i in $(seq 1 5); do
    echo "binary-placeholder-$i" | \
      aws s3 cp - "s3://sacha-assets/$folder/file-$(printf '%03d' "$i").bin"
  done
done

# sacha-backups: date-partitioned folders
for month in 01 02 03 04 05 06; do
  for day in 01 15; do
    echo "{\"backup\":true,\"date\":\"2025-$month-$day\"}" | \
      aws s3 cp - "s3://sacha-backups/2025/$month/$day/backup.json"
  done
done

# sacha-config: small config files
echo '{"app":"sacha","version":"1.0"}' | aws s3 cp - "s3://sacha-config/app.json"
echo '{"database":{"host":"localhost","port":5432}}' | aws s3 cp - "s3://sacha-config/database.json"
echo '{"logging":{"level":"info"}}' | aws s3 cp - "s3://sacha-config/logging.json"

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
done
echo "Seeded 60 log groups"

echo "=== Seeding DynamoDB ==="

# Users table — 80 items (hash key), tests 4+ scan pages (page size 25)
aws dynamodb create-table \
  --table-name Users \
  --attribute-definitions AttributeName=userId,AttributeType=S \
  --key-schema AttributeName=userId,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST 2>/dev/null || true

for i in $(seq 1 80); do
  aws dynamodb put-item --table-name Users --item \
    "{\"userId\":{\"S\":\"user-$(printf '%03d' "$i")\"},\"name\":{\"S\":\"User $i\"},\"email\":{\"S\":\"user$i@example.com\"},\"age\":{\"N\":\"$((20 + RANDOM % 50))\"},\"active\":{\"BOOL\":$([ $((i % 5)) -eq 0 ] && echo false || echo true)}}" \
    > /dev/null
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
  --billing-mode PAY_PER_REQUEST 2>/dev/null || true

for i in $(seq 1 50); do
  aws dynamodb put-item --table-name Orders --item \
    "{\"orderId\":{\"S\":\"order-$(printf '%03d' "$i")\"},\"createdAt\":{\"S\":\"2025-01-$(printf '%02d' $((i % 28 + 1)))T10:00:00Z\"},\"total\":{\"N\":\"$((RANDOM % 10000))\"},\"status\":{\"S\":\"$([ $((i % 3)) -eq 0 ] && echo shipped || echo pending)\"}}" \
    > /dev/null
done

# Sessions table — 30 items
aws dynamodb create-table \
  --table-name Sessions \
  --attribute-definitions AttributeName=sessionId,AttributeType=S \
  --key-schema AttributeName=sessionId,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST 2>/dev/null || true

for i in $(seq 1 30); do
  aws dynamodb put-item --table-name Sessions --item \
    "{\"sessionId\":{\"S\":\"sess-$(printf '%03d' "$i")\"},\"userId\":{\"S\":\"user-$(printf '%03d' $((i % 80 + 1)))\"},\"ttl\":{\"N\":\"$(($(date +%s) + 3600))\"}}" \
    > /dev/null
done

# 117 additional empty tables for list pagination (page size 100)
for i in $(seq 1 117); do
  aws dynamodb create-table \
    --table-name "table-$(printf '%03d' "$i")" \
    --attribute-definitions AttributeName=id,AttributeType=S \
    --key-schema AttributeName=id,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST 2>/dev/null || true
done
echo "Seeded 120 DynamoDB tables"

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
  2>/dev/null || true

aws lambda create-function \
  --function-name data-processor \
  --runtime python3.12 \
  --handler lambda_function.handler \
  --role arn:aws:iam::000000000000:role/lambda-role \
  --zip-file "fileb://$TMPDIR/function.zip" \
  --environment "Variables={BATCH_SIZE=100,QUEUE_URL=https://sqs.example.com/queue}" \
  --timeout 300 --memory-size 512 \
  2>/dev/null || true

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
  2>/dev/null || true

aws lambda create-function \
  --function-name notification-sender \
  --runtime nodejs20.x \
  --handler index.handler \
  --role arn:aws:iam::000000000000:role/lambda-role \
  --zip-file "fileb://$TMPDIR/node-function.zip" \
  --timeout 30 --memory-size 256 \
  2>/dev/null || true

aws lambda create-function \
  --function-name health-checker \
  --runtime python3.12 \
  --handler lambda_function.handler \
  --role arn:aws:iam::000000000000:role/lambda-role \
  --zip-file "fileb://$TMPDIR/function.zip" \
  --timeout 10 --memory-size 128 \
  2>/dev/null || true

# 60 additional functions for list pagination (page size 50)
for i in $(seq 1 60); do
  aws lambda create-function \
    --function-name "fn-$(printf '%02d' "$i")" \
    --runtime python3.12 \
    --handler lambda_function.handler \
    --role arn:aws:iam::000000000000:role/lambda-role \
    --zip-file "fileb://$TMPDIR/function.zip" \
    2>/dev/null || true
done
echo "Seeded 65 Lambda functions"

rm -rf "$TMPDIR"

echo ""
echo "=== Seeding complete ==="
echo "Run sacha against LocalStack:"
echo "  make local-run"
echo "  # or: ./bin/sacha --endpoint http://localhost:4566 --region us-east-1"
