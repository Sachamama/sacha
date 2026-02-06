# Sacha Full Demo — 20-25s showcase with deeper feature exploration
#
# Prerequisites:
#   make local-up && make local-seed && make local-seed-live &
#   make build
#
# Record:
#   cd demos && vhs sacha-full.tape
#
# Output: sacha-full.mp4 (1920x1080, ~22s)

Output sacha-full.mp4

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

# === CloudWatch Logs (0-5s) ===
# Navigate log groups
Sleep 300ms
Type "j"
Sleep 100ms
Type "j"
Sleep 100ms
Type "j"
Sleep 300ms

# Select two groups
Type " "
Sleep 200ms
Type "j"
Sleep 100ms
Type " "
Sleep 300ms

# Start tailing
Type "t"
Sleep 1500ms

# Switch to tail panel
Tab
Sleep 400ms

# Navigate tail events
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 400ms

# Stop tailing
Type "x"
Sleep 400ms

# === S3 (5-11s) ===
Type "s"
Sleep 400ms
Type "s3"
Sleep 200ms
Enter
Sleep 600ms

# Navigate to a bucket
Type "j"
Sleep 150ms
Type "j"
Sleep 300ms

# Open bucket
Enter
Sleep 600ms

# Navigate inside bucket
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 300ms

# Open a folder
Enter
Sleep 600ms

# Navigate files
Type "j"
Sleep 150ms
Type "j"
Sleep 300ms

# Go back
Escape
Sleep 400ms
Escape
Sleep 400ms

# === DynamoDB (11-17s) ===
Type "s"
Sleep 400ms
Type "dyn"
Sleep 200ms
Enter
Sleep 600ms

# Navigate tables
Type "j"
Sleep 150ms
Type "j"
Sleep 300ms

# Open table to scan items
Enter
Sleep 800ms

# Navigate items
Type "j"
Sleep 150ms
Type "j"
Sleep 300ms

# Expand an item
Enter
Sleep 800ms

# Close popup
Escape
Sleep 300ms

# Go back to tables
Escape
Sleep 400ms

# === Lambda (17-22s) ===
Type "s"
Sleep 400ms
Type "lam"
Sleep 200ms
Enter
Sleep 600ms

# Filter functions
Type "/"
Sleep 200ms
Type "api"
Sleep 400ms
Enter
Sleep 400ms

# Expand function details
Enter
Sleep 1000ms

# Close and quit
Escape
Sleep 400ms
