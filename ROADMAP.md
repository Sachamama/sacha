# Sacha Roadmap

Planned features and new AWS services, building on Sacha's existing TUI patterns.

**Current services:** CloudWatch Logs, S3, DynamoDB, Lambda, SSM Parameter Store, SQS, EC2

---

## Completed

### Phase 1: CloudWatch Logs Enhancements — [#40](https://github.com/Sachamama/sacha/pull/40)

Set retention policy (`R`), delete log groups (`d`), show creation date in details pane.

### Phase 2: SSM Parameter Store Browser — [#42](https://github.com/Sachamama/sacha/pull/42)

Hierarchical path navigation, parameter details with decryption, expanded popup, copy (`y`), lazy-load pagination, scroll memory.

### Phase 3: SQS Queue Browser — [#43](https://github.com/Sachamama/sacha/pull/43)

Queue list with message stats, non-destructive message peeking, message body viewer with JSON formatting, queue attributes popup, copy (`y`), search/filter.

### Phase 4a: EC2 Instance Browser — [#49](https://github.com/Sachamama/sacha/pull/49)

Instance list with color-coded state, full metadata details, expanded popup with tags, copy instance ID (`y`), search/filter, lazy-load pagination.

---

## Phase 5: Feature Consistency

Align all services to a consistent baseline before adding new ones.

### 5a: Copy Support for CloudWatch Logs

CloudWatch Logs is the only service missing the `y` key binding. Add copy support for log group names (and log group ARNs in the details pane).

### 5b: "Load All" for Paginated Lists

S3 is the only service with an `A` key to load all remaining pages at once. Extend this pattern to services where bulk visibility matters: DynamoDB tables, Lambda functions, EC2 instances.

### 5c: Refresh Support

Add a `ctrl+r` or `g` keybinding across all services to reload the current view from AWS. Currently, stale data requires restarting the app.

---

## Phase 6: Secrets Manager Browser

**Complexity:** Low — simple flat list, structurally similar to SSM.

- **Secret list** — left pane with secret names, lazy-load pagination
- **Secret details** — right pane shows description, rotation config, last accessed/changed dates, tags
- **Retrieve value** — `enter/space` to fetch and display secret value in expanded popup (requires `GetSecretValue`)
- **Value masking** — secret values shown masked by default, with a toggle to reveal
- **Copy** — `y` to copy secret name or ARN
- **Search/filter** — `/` to filter by name or description
- **Version history** — show version stages (AWSCURRENT, AWSPREVIOUS) in the detail view

### Client API

- `ListSecrets` with pagination
- `GetSecretValue` for on-demand retrieval
- `DescribeSecret` for metadata

---

## Phase 7: SNS Topic Browser

**Complexity:** Low — topic list with subscription drill-down.

- **Topic list** — left pane with topic names/ARNs, lazy-load pagination
- **Topic details** — right pane shows display name, subscription count, policy summary, encryption, FIFO flag
- **Subscriptions view** — `enter` to list subscriptions for a topic (protocol, endpoint, status)
- **Expanded popup** — `space` to view full topic attributes
- **Copy** — `y` to copy topic ARN
- **Search/filter** — `/` to filter by name

### Client API

- `ListTopics` with pagination
- `GetTopicAttributes` for metadata
- `ListSubscriptionsByTopic` for drill-down

---

## Phase 8: CloudFormation Stack Browser

**Complexity:** Medium — stacks with nested resource and event views.

- **Stack list** — left pane with stack names and color-coded status (CREATE_COMPLETE green, ROLLBACK red, IN_PROGRESS yellow, DELETE gray)
- **Stack details** — right pane shows status reason, description, creation/update timestamps, outputs, parameters
- **Resources view** — `enter` to list stack resources with logical/physical IDs, type, and status
- **Events view** — `e` to show stack events chronologically (useful for debugging failed deployments)
- **Expanded popup** — `space` to view full stack details, outputs, and parameters
- **Copy** — `y` to copy stack name or ARN
- **Search/filter** — `/` to filter by name or status
- **Scroll memory** — restore position when navigating back from resources/events

### Client API

- `ListStacks` (excluding DELETE_COMPLETE by default) with pagination
- `DescribeStacks` for full details
- `ListStackResources` for resource drill-down
- `DescribeStackEvents` for event history

---

## Phase 9: Route 53 Browser

**Complexity:** Medium — hosted zones with record set drill-down.

- **Hosted zones list** — left pane with zone names, record count, public/private indicator
- **Zone details** — right pane shows zone ID, record count, comment, associated VPCs (private zones)
- **Records view** — `enter` to list DNS records with name, type, TTL, and values
- **Expanded popup** — `space` to view full record details (alias targets, routing policies, health checks)
- **Copy** — `y` to copy zone ID or record value
- **Search/filter** — `/` to filter zones by name; records by name or type

### Client API

- `ListHostedZones` with pagination
- `ListResourceRecordSets` for record drill-down
- `GetHostedZone` for zone metadata

---

## Phase 10: ECS Browser

**Complexity:** High — deeply nested hierarchy requiring multi-level navigation.

- **Cluster list** — top-level view with cluster names, active service/task counts, status
- **Services view** — `enter` on a cluster to list services with desired/running/pending counts, launch type
- **Tasks view** — `enter` on a service to list tasks with status, started time, health, launch type
- **Container view** — `enter` on a task to show container details (image, status, ports, health, last status)
- **Expanded popup** — `space` at any level for full details
- **Scroll memory** — scroll stack for the full cluster > service > task > container hierarchy
- **Copy** — `y` to copy ARN at any level
- **Search/filter** — `/` at each level

### Client API

- `ListClusters` + `DescribeClusters` for cluster details
- `ListServices` + `DescribeServices` per cluster
- `ListTasks` + `DescribeTasks` per service
- `DescribeContainerInstances` for EC2 launch type

### Design Notes

ECS is the deepest hierarchy in Sacha. Reuse the `scrollStack` pattern from S3/SSM but extend it to 4 levels. Consider showing breadcrumbs (e.g., `cluster > service > task`) in the header to indicate navigation depth.

---

## Phase 11: EC2 Sub-Resources

**Complexity:** Medium — extends the existing EC2 service with additional resource types.

- **EBS Volumes** — list with state, size, type, attached instance, IOPS
- **Elastic IPs** — list with allocation ID, association, public IP, domain
- **Security Groups** — list with group ID, name, VPC, inbound/outbound rule counts; expand to view rules
- **Key Pairs** — list with name, type, fingerprint

These can be added as sub-views within the EC2 service (e.g., a resource-type picker before the list view) or as separate services registered independently. The sub-view approach keeps related resources grouped but adds navigation complexity.

---

## Future Considerations

Services that may be added later based on user demand:

| Service | Notes | Complexity |
|---------|-------|------------|
| IAM | Global service (not regional), many sub-resource types (users, roles, policies, groups) — would need special handling for the region switcher | High |
| Step Functions | State machine list + execution history + visual execution status | Medium |
| EventBridge | Event buses + rules + targets | Medium |
| RDS | DB instances + clusters, similar pattern to EC2 | Medium |
| CodeBuild | Build project list + build history | Low |
| CloudWatch Metrics | Metric namespaces + dimensions + inline sparkline graphs — would be a novel UI pattern | High |

---

## Cross-Cutting Improvements

Improvements that apply across all services, independent of new service work.

### Error Handling & Resilience

- **Retry with backoff** — add automatic retry for transient AWS errors (throttling, timeouts) in client methods
- **Error display** — show AWS API errors inline in the status bar rather than crashing; allow dismissal and retry
- **Credential expiry** — detect expired credentials and prompt the user to refresh rather than showing cryptic errors

### UX Polish

- **Breadcrumb header** — for services with hierarchical navigation (S3, SSM, ECS), show the current path in the header
- **Configurable page size** — allow users to set page sizes per service in `config.json`
- **Keyboard shortcut help overlay** — `?` to show a full-screen cheat sheet of available keys for the current view
- **Theme support** — let users choose or define color themes in config (e.g., light mode, high-contrast)

### Performance

- **Background prefetch** — when the cursor is near the bottom of the list, prefetch the next page before the threshold is hit
- **Cache recent results** — cache list results with a short TTL so navigating back doesn't re-fetch

### Testing

- **UI integration tests** — test Bubble Tea models by sending key messages and asserting on model state transitions
- **Mock AWS client interface** — extract interfaces from concrete clients to simplify test setup across all services
