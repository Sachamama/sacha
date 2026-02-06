# Sacha Quick Demo — 6-10s showcase of all 4 AWS services
#
# Prerequisites:
#   make local-up && make local-seed && make build
#
# Record:
#   cd demos && vhs sacha-quick.tape
#
# Output: sacha-quick.mp4 (1920x1080, ~8s)

Output sacha-quick.mp4

Set Shell bash
Set FontSize 16
Set Width 1920
Set Height 1080
Set Padding 20
Set TypingSpeed 40ms
Set Theme "Catppuccin Mocha"
Set WindowBar Colorful
Set WindowBarSize 40

# Launch sacha against LocalStack
Hide
Type "AWS_ENDPOINT_URL=http://localhost:4566 AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1 ./bin/sacha --service cloudwatch-logs"
Enter
Sleep 2s
Show

# --- CloudWatch Logs (0-2s) ---
# Show log groups list, navigate down
Sleep 400ms
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 600ms

# --- Switch to S3 (2-4s) ---
Type "s"
Sleep 400ms
Type "s3"
Sleep 200ms
Enter
Sleep 600ms

# Navigate S3 buckets
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 600ms

# --- Switch to DynamoDB (4-6s) ---
Type "s"
Sleep 400ms
Type "dyn"
Sleep 200ms
Enter
Sleep 600ms

# Navigate DynamoDB tables
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 600ms

# --- Switch to Lambda (6-8s) ---
Type "s"
Sleep 400ms
Type "lam"
Sleep 200ms
Enter
Sleep 600ms

# Navigate Lambda functions
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 400ms

# Expand a function detail
Enter
Sleep 800ms

# Close and end
Escape
Sleep 300ms
