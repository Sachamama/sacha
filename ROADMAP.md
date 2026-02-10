# Sacha Roadmap

Planned features and new AWS services, building on Sacha's existing TUI patterns.

**Current services:** CloudWatch Logs, S3, DynamoDB, Lambda, SSM Parameter Store, SQS, EC2

---

## Phase 1: CloudWatch Logs Enhancements _(completed)_

All three items shipped in [#40](https://github.com/Sachamama/sacha/pull/40):

- **Set Retention Policy (`R` key)** — retention picker with standard options, applied to multi-selected groups via `PutRetentionPolicy`
- **Delete Log Groups (`D` key)** — delete with confirmation overlay, multi-select support via `DeleteLogGroup`
- **Show Creation Date** — displayed in the right-pane details panel, formatted from `creationTime`

---

## Phase 2: SSM Parameter Store Browser _(completed)_

Browse parameters by path hierarchy with folder-style navigation.

- **Path navigation** — `enter` to drill into path prefixes, `esc/backspace/h` to go back
- **Details pane** — parameter value (with decryption), type, version, last modified, ARN
- **Expanded popup** — `enter/space` on a parameter to view full details in a scrollable overlay
- **Copy** — `y` to copy parameter value or path
- **Lazy-load pagination** — loads more parameters near the bottom of the list
- **Scroll memory** — cursor position restored on back navigation via `scrollStack`

### Files

- `internal/ssm/client.go` — `GetParametersByPath`, `GetParameter`, `ListTopLevelPaths`
- `internal/ssm/types.go` — `Parameter` domain type
- `internal/ssm/client_test.go` — 16 tests covering pagination, error handling, path grouping
- `internal/ui/ssm/service.go` — `SSMService` implementing `awsx.Service`
- `internal/ui/ssm/model.go` — Bubble Tea model with scroll stack, lazy-load, expanded popup
- `internal/ui/ssm/views.go` — two-pane layout, parameter list, details, popup overlay
- `internal/ui/ssm/messages.go` — `parametersLoadedMsg`, `moreParametersLoadedMsg`, `parameterDetailMsg`

---

## Phase 3: SQS Queue Browser _(completed)_

Browse queues with message count stats and peek messages.

- **Queue list** — left pane shows queue names with lazy-load pagination
- **Queue details** — right pane shows message counts (visible, in-flight, delayed), type (FIFO/Standard), visibility timeout, redrive policy, encryption
- **Peek messages** — `enter` to peek messages via `ReceiveMessage` with visibility timeout 0 (non-destructive)
- **Message view** — navigate peeked messages, view body (auto-formatted JSON), expand in popup
- **Expand queue** — `space` to expand full queue attributes in scrollable popup
- **Copy** — `y` to copy queue URL; `y` in message view copies message body
- **Search/filter** — `/` to filter queues by name

### Files

- `internal/sqs/client.go` — `ListQueues`, `GetQueueAttributes`, `PeekMessages`
- `internal/sqs/types.go` — `Queue`, `QueueAttributes`, `Message` domain types
- `internal/sqs/client_test.go` — 14 tests covering pagination, attribute parsing, message peeking, error handling
- `internal/ui/sqs/service.go` — `SQSService` implementing `awsx.Service`
- `internal/ui/sqs/model.go` — Bubble Tea model with queue list, message peek view, expanded popups
- `internal/ui/sqs/views.go` — two-pane layout, queue list, details, message list, popup overlays
- `internal/ui/sqs/messages.go` — `queuesLoadedMsg`, `moreQueuesLoadedMsg`, `queueAttributesMsg`, `messagesLoadedMsg`

---

## Phase 4: EC2 Instance Browser _(completed — 4a)_

### 4a: EC2 Instances _(completed)_

Browse instances with state, type, IP, and name tag.

- **Instance list** — left pane shows instances with color-coded state, type, and name tag
- **Instance details** — right pane shows full instance metadata (IPs, VPC, subnet, AZ, AMI, key pair, security groups, IAM profile)
- **Expanded popup** — `enter/space` to expand instance in scrollable overlay with all details and tags
- **Copy** — `y` to copy instance ID
- **Search/filter** — `/` to filter by name, instance ID, type, state, or IP
- **Lazy-load pagination** — loads more instances near the bottom of the list
- **Color-coded states** — running (green), stopped (red), pending/stopping (yellow), terminated (gray)

### Files

- `internal/ec2/client.go` — `ListInstances` via `DescribeInstances` with pagination
- `internal/ec2/types.go` — `Instance`, `SecurityGroup` domain types
- `internal/ec2/client_test.go` — tests covering pagination, field mapping, error handling
- `internal/ui/ec2/service.go` — `EC2Service` implementing `awsx.Service`
- `internal/ui/ec2/model.go` — Bubble Tea model with instance list, lazy-load, expanded popup
- `internal/ui/ec2/views.go` — two-pane layout, instance list with state colors, details, popup overlay
- `internal/ui/ec2/messages.go` — `instancesLoadedMsg`, `moreInstancesLoadedMsg`

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
