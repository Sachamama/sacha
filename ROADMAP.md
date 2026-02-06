# Sacha Roadmap

Planned features and new AWS services, building on Sacha's existing TUI patterns.

## Phase 1: CloudWatch Logs Enhancements ✅ Completed

---

### ✅ Set Retention Policy (`R` key)

All three items shipped in [#40](https://github.com/Sachamama/sacha/pull/40):

- **Set Retention Policy (`R` key)** — retention picker with standard options, applied to multi-selected groups via `PutRetentionPolicy`
- **Delete Log Groups (`D` key)** — delete with confirmation overlay, multi-select support via `DeleteLogGroup`
- **Show Creation Date** — displayed in the right-pane details panel, formatted from `creationTime`

---

## Phase 2: SSM Parameter Store Browser _(completed)_

- AWS API: `DeleteLogGroup`
- Needs confirmation overlay (y/n)
- Files: `internal/logs/client.go`, `internal/ui/logs/model.go`
- Completed in PR #40

### ✅ Show Creation Date

### Files


- No new API calls needed
- Files: `internal/ui/logs/views.go`
- Completed in PR #40

---

## Phase 3: SQS Queue Browser

Browse queues with message count stats. Peek messages and optionally tail incoming messages.

- Left pane: queue list with approximate message counts (visible, in-flight, delayed)
- Right pane: queue attributes (type FIFO/Standard, visibility timeout, redrive policy, encryption)
- `enter` to peek messages (`ReceiveMessage` with visibility timeout 0)
- `space` to expand queue details in popup
- `y` to copy queue URL

### AWS APIs

- `ListQueues` — list all queues (paginated via `NextToken`)
- `GetQueueAttributes` — message counts, configuration
- `ReceiveMessage` — peek at messages (visibility timeout 0)

### Architecture

- `internal/sqs/client.go` + `internal/sqs/types.go`
- `internal/ui/sqs/service.go` + `internal/ui/sqs/model.go` + views + messages

---

## Phase 4: EC2 Instance Browser

Start with instances only. Sub-resources deferred to Phase 4b.

### 4a: EC2 Instances

- Browse instances with state (color-coded), type, IP, name tag
- `enter/space` to expand instance details
- `y` to copy instance ID
- `DescribeInstances` (paginated)

### 4b: EC2 Sub-Resources _(deferred)_

- EBS volumes, Elastic IPs, Security groups, Key pairs

---

## Future Considerations

| Service | Notes | Complexity |
|---------|-------|------------|
| CloudFormation | Stacks + resources + events view | Medium |
| Secrets Manager | Simple list + view, needs value masking | Low |
| ECS | Deeply nested hierarchy (cluster > service > task > container) | High |
| IAM | Global service (not regional), many sub-resource types | High |
| SNS | Topic list + subscriptions | Low |
| Route 53 | Hosted zones + records | Medium |
