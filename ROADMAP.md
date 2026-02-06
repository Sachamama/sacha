# Sacha Roadmap

Planned features and new AWS services, building on Sacha's existing TUI patterns.

## Phase 1: CloudWatch Logs Enhancements

Quick wins that extend the existing CloudWatch Logs service with high-value operations.

### Set Retention Policy (`R` key)

Add a retention picker to set retention on selected log groups. Show standard options: 1d, 3d, 5d, 7d, 14d, 30d, 60d, 90d, 1y, never.

- AWS API: `PutRetentionPolicy`
- Reuses existing multi-select pattern
- Files: `internal/logs/client.go`, `internal/ui/logs/model.go`

### Delete Log Groups (`D` key)

Delete selected log groups with a confirmation prompt. Multi-select already works.

- AWS API: `DeleteLogGroup`
- Needs confirmation overlay (y/n)
- Files: `internal/logs/client.go`, `internal/ui/logs/model.go`

### Show Creation Date

Display log group creation date in the right-pane details. `DescribeLogGroups` already returns `creationTime` — just format and display it.

- No new API calls needed
- Files: `internal/ui/logs/views.go`

## Phase 2: SSM Parameter Store Browser

Browse parameters by path hierarchy, reusing S3's folder-navigation and scroll-stack patterns.

- Left pane: path tree (`/app/prod/db-host`, `/app/prod/db-port`, ...)
- Right pane: parameter value, type, version, last modified
- `y` to copy value
- `enter` to navigate into path prefix
- `esc/backspace` to go back up

### AWS APIs

- `GetParametersByPath` — list parameters under a prefix
- `GetParameter` — fetch single parameter value
- `DescribeParameters` — metadata and filtering

### Architecture

- `internal/ssm/client.go` + `internal/ssm/types.go`
- `internal/ui/ssm/service.go` + `internal/ui/ssm/model.go`
- Scroll memory via `scrollStack` (same as S3)

## Phase 3: SQS Queue Browser

Browse queues with message count stats. Peek messages and optionally tail incoming messages using the CloudWatch tailing pattern.

- Left pane: queue list with message counts
- Right pane: queue attributes (type, visibility timeout, redrive policy)
- `enter` to peek messages (`ReceiveMessage` with visibility timeout 0)
- Potential tail mode for watching incoming messages

### AWS APIs

- `ListQueues` — list all queues
- `GetQueueAttributes` — message counts, configuration
- `ReceiveMessage` — peek at messages

### Architecture

- `internal/sqs/client.go` + `internal/sqs/types.go`
- `internal/ui/sqs/service.go` + `internal/ui/sqs/model.go`
- Optional `Tailing()` interface for live message watching

## Phase 4: EC2 Browser

Start with instances-only, expand to sub-resources later.

### 4a: EC2 Instances

- Browse instances with state, type, IP, name tag
- Start/stop instances with confirmation
- View full instance details in expandable popup

### 4b: EC2 Sub-Resources (future)

- EBS volumes (find unattached)
- Elastic IPs (find unassociated)
- Security groups (view rules, find unused)
- Key pairs (find unused)

### AWS APIs

- `DescribeInstances`, `StartInstances`, `StopInstances`
- `DescribeVolumes`, `DescribeAddresses`, `DescribeSecurityGroups`, `DescribeKeyPairs`

## Future Considerations

Services evaluated but deferred due to complexity or niche use:

| Service | Notes |
|---------|-------|
| CloudFormation | Stacks + resources + events view. "Find stack by resource" is a unique feature |
| Secrets Manager | Simple list + view, but sensitive (secrets display needs masking) |
| ECS | Very valuable but deeply nested hierarchy (cluster > service > task > container) |
| IAM | Global service (not regional), many sub-resource types |
| Multi-Account Switcher | Cross-cutting concern needing STS assume-role integration |
