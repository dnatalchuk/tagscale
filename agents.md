# `agents.md` — TagScale Background Agents

> **TagScale** helps small to medium-sized teams gain real-time FinOps visibility with minimal setup. These agents form the backbone of the system—handling cost collection, attribution, notifications, and (soon) automated remediation like tagging gaps.

## 🎯 Product Philosophy

- **CLI-first**: Designed to work great in a terminal—fast, scriptable, clean output  
- **Zero-config**: Sensible defaults and optional persistence  
- **Extendable**: Dashboard and write features plug into the same agent backbone  
- **Actionable**: From insight to impact—future agents will apply missing tags  

---

## 🚦 Agents Overview

| Agent Name          | Responsibility                            | Triggered From                 | Planned Write Support |
|---------------------|--------------------------------------------|--------------------------------|------------------------|
| `CostCollectorAgent` | Pull real AWS cost data                   | CLI, API, server               | ❌                     |
| `InferenceAgent`     | Infer ownership from metadata             | CLI, API, server               | ✅                     |
| `DashboardAgent`     | Prepare visual summaries & trends         | API, server                    | ❌                     |
| `NotificationAgent`  | Send Slack/email digests                  | Server                         | ❌                     |
| `TagWriteAgent`      | Apply missing tags to AWS resources       | CLI, API                       | ✅ *(planned)*         |
| `AuthAgent`          | Secure API requests with token auth       | API middleware                 | ✅ *(planned)*         |

---

## 🔍 `CostCollectorAgent`

**Purpose**: Automatically gathers daily cost and usage data from AWS Cost Explorer.

**Capabilities**:
- Collects cost by service, account, resource
- Supports time-based queries (e.g. last 7 days)
- De-duplicates and persists cost entries when `--db` is used

**Triggers**:
- CLI: `tagscale-cli scan --days 7`
- API: `POST /api/v1/costs/collect`
- Server: Via `DATA_COLLECTION_INTERVAL`

---

## 🧠 `InferenceAgent`

**Purpose**: Uses smart heuristics to infer team and service ownership.

**Heuristics Used**:
- Resource name patterns (`web-*`, `api-*`, `dev-*`)
- Service-to-team mappings (e.g. `AmazonEKS` → `platform`)
- Native tag analysis (`Team`, `Owner`, etc.)
- Custom rules (extensible)

**Triggers**:
- CLI: `tagscale-cli summary`
- API: `POST /api/v1/analysis/run`
- Server: Via `ANALYSIS_INTERVAL`

---

## 🧾 `TagWriteAgent` (Coming Soon)

**Purpose**: Auto-applies inferred tags to untagged AWS resources.

**Actions**:
- Applies tags via AWS SDK (opt-in IAM permissions)
- Supports dry-run mode to preview changes
- Logs and tracks all mutations

**Planned Use Cases**:
- Auto-fix cost attribution gaps
- Baseline tagging enforcement

**Trigger Modes**:
- CLI: `tagscale-cli fix-tags`
- API: Future endpoint: `POST /api/v1/tags/apply`

---

## 📈 `DashboardAgent`

**Purpose**: Prepares summarized data for frontend dashboards and charts.

**Data Sources**:
- Cost breakdown by team, service, account
- Daily/weekly/monthly trends
- Untagged cost percentage over time

**Triggers**:
- Server: periodic job
- API endpoints:
  - `GET /api/v1/dashboard/overview`
  - `GET /api/v1/dashboard/trends`

---

## 🔔 `NotificationAgent`

**Purpose**: Sends automated digests via Slack or email to improve cost visibility.

**Features**:
- Slack daily summaries per channel
- Email cost digests with visual charts (planned)
- Future: alerting on anomalies or spikes

**Configurable via**:
- `SLACK_TOKEN`, `SLACK_CHANNEL`
- `EMAIL_SMTP_*`

---

## 🔐 `AuthAgent` (Planned)

**Purpose**: Secures API access using token-based authentication.

**Supports**:
- API key validation (`Authorization: Bearer <API_KEY>`)
- Role-based access (future)
- OAuth provider integration (future)

---

## 🧪 Running Agents Locally

You can simulate full workflows with the CLI:

```bash
# Collect cost data and store in memory
./bin/tagscale-cli scan --days 7

# Run inference
./bin/tagscale-cli summary

# (Future) Apply missing tags
./bin/tagscale-cli fix-tags --dry-run
