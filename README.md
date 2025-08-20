# TagScale 🚀

> # TagScale - Smart Cloud Cost Attribution

TagScale helps engineering teams understand where their cloud costs go, even with messy or incomplete tagging.

## Features

- **Zero-effort cost attribution**: Works with missing or inconsistent tags
- **Smart inference**: Uses resource names, patterns, and heuristics to determine ownership
- **Clean dashboards**: Simple, focused interface for small teams
- **Automated notifications**: Slack and email digests for cost visibility
- **AWS Cost Explorer integration**: Pull real cost data automatically

## Quick Start

### Prerequisites

- Go 1.23+ (Go toolchain 1.24+ recommended)
- PostgreSQL 12+ (only required when using database persistence or running the server)
- AWS credentials with Cost Explorer access
- Docker (optional)
- Node.js 18+ (for running the frontend)

### Installation

1. Clone the repository:
```bash
git clone https://github.com/your-org/tagscale.git
cd tagscale
```

2. Install dependencies:
```bash
make deps
```

3. Set up environment variables:
```bash
cp .env.example .env
# Edit .env with your configuration
```

4. (Optional) Run database migrations manually:
```bash
make migrate
```

Migrations will also run automatically when starting the server or when using `--migrate` with the CLI.

5. Start the application:
```bash
make run
```

### Docker Setup

```bash
# Start all services
make docker-run

# Stop all services
make docker-stop
```

## Frontend Development

1. Install dependencies:
   ```bash
   cd frontend
   npm install
   ```
2. Start the development server:
   ```bash
   npm start
   ```

To build the frontend Docker image:
```bash
docker build -t tagscale-frontend ./frontend
```

## Usage

Build the CLI tool:

```bash
make build-cli
```

The binary will be created at `bin/tagscale-cli`.

Check the installed version:

```bash
./bin/tagscale-cli version
./bin/tagscale-cli version --output json
```

By default the CLI stores all data in an in-memory SQLite database. Use the `--db-path`
flag to persist results to a local SQLite file, or the `--db` flag to use the database
configured by `DATABASE_URL`. Use `--migrate` to run any pending database migrations.
Migrations are skipped by default for faster startup.

Use `--quiet` (`-q`) to suppress progress messages. When combined with JSON output, success messages are omitted as well. Use `--silent` to suppress all CLI output, or `--verbose` (`-v`) for more detailed progress. These flags are mutually exclusive.

To apply migrations explicitly, include `--migrate` with your command, for example:

```bash
./bin/tagscale-cli scan --db-path /tmp/tagscale.db --migrate
```

If `scan` and `summary` are run as separate commands without `--db` or `--db-path`,
each invocation starts with a fresh in-memory database and the scan results are lost
before the summary runs. Use `--db`, `--db-path`, or run both commands in a single
session to retain the collected data.

### Example: In-memory scan and summary

Run a quick scan for the last 7 days without saving results to a database and then
print a summary for the same period:

```bash
./bin/tagscale-cli scan --range 7
./bin/tagscale-cli summary --range 7
```

You can override the AWS region and shared config profile, adjust the AWS request timeout (must be greater than 0), or emit JSON instead of
the default table output (supported values: `table`, `json`):

```bash
./bin/tagscale-cli scan --range 7 --region us-west-2 --profile my-profile --timeout 60
./bin/tagscale-cli summary --range 2024-01-01:2024-01-07 --output json
```

The `--range` flag accepted by both `scan` and `summary` commands supports either a
number of days (e.g. `7` for the last seven days) or an explicit date or date
range using the format `YYYY-MM-DD` or `YYYY-MM-DD:YYYY-MM-DD`.

To focus on specific dimensions, the `summary` command supports grouping and limiting.
Accepted values for `--group-by` are `service` (default), `account`, `region`, and `team`.
The `--limit` flag restricts the number of results and must be a positive integer:

```bash
./bin/tagscale-cli summary --group-by account --limit 3
```

Sample output grouped by account:

```text
🔍 Running TagScale scan...
✅ Cost data collected.

💰 Total Cost: $123.45
🏷️ Untagged Cost: $45.67 (37.0%)

Top Accounts:
 • 123456789012                $100.00
 • 210987654321                $20.00

Insights:
 • 37.0% of your costs are untagged. Consider implementing tagging policies.
```

## Configuration

### Environment Variables

- `DATABASE_URL` (optional): PostgreSQL connection string used with `--db` or when running the server
- `AWS_REGION`: AWS region for Cost Explorer
- `AWS_PROFILE` (optional): Shared config profile name used for AWS credentials
- `AWS_REQUEST_TIMEOUT`: Timeout in seconds for AWS API calls (default used by the CLI `--timeout` flag; must be > 0)
- `SLACK_TOKEN`: Slack bot token for notifications
- `EMAIL_SMTP_*`: Email configuration for notifications
- `DATA_COLLECTION_INTERVAL`: How often to collect cost data (minutes)
- `ANALYSIS_INTERVAL`: How often to run cost analysis (minutes)
- `API_KEYS` (optional): Comma-separated API keys required for API requests. Send as `Authorization: Bearer <API_KEY>`
- `CORS_ORIGINS`: Comma-separated list of allowed origins for API requests
- `REACT_APP_API_URL`: base URL for API calls made by the React app
- `REACT_APP_API_KEY` (optional): API key for the React frontend. When set, the app sends requests with `Authorization: Bearer <key>`

### AWS Permissions

Your AWS credentials need the following permissions:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "ce:GetCostAndUsage",
                "ce:GetRightsizingRecommendation",
                "ce:GetReservationCoverage",
                "ce:GetReservationPurchaseRecommendation",
                "ce:GetReservationUtilization",
                "ce:GetSavingsPlansUtilization",
                "ce:GetUsageReport"
            ],
            "Resource": "*"
        }
    ]
}
```

## API Endpoints

All `/api/v1` routes require one of the `API_KEYS` if any are configured. Include it in requests as:

```
Authorization: Bearer <API_KEY>
```

### Cost Management
- `GET /api/v1/costs/summary` - Get cost summary by time period
- `GET /api/v1/costs/top` - Get top costs by service/account/region
- `POST /api/v1/costs/collect` - Trigger cost data collection

### Analysis
- `GET /api/v1/analysis/latest` - Get latest cost analysis
- `POST /api/v1/analysis/run` - Trigger cost analysis

### Dashboard
- `GET /api/v1/dashboard/overview` - Get dashboard overview data
- `GET /api/v1/dashboard/trends` - Get cost trends over time

## Development

### Running Tests
```bash
make test
```

### Code Formatting
```bash
make fmt
```

### Linting
```bash
make lint
```

### Live Reload
```bash
# Install air for live reload
go install github.com/cosmtrek/air@latest
make dev
```

## Team Inference

TagScale uses several heuristics to infer team ownership:

1. **Resource Name Patterns**: Matches prefixes like `web-*`, `api-*`, `data-*`
2. **Service Mapping**: Maps AWS services to likely teams
3. **Tag Analysis**: Parses tags returned by Cost Explorer (e.g. `Team=backend`)
   and stores them with each cost record for ownership inference
4. **Custom Rules**: Define your own mapping rules

## Notifications

### Slack Integration
1. Create a Slack app with bot permissions
2. Add the bot to your desired channel
3. Set `SLACK_TOKEN` and `SLACK_CHANNEL` in your environment

### Email Notifications
Configure SMTP settings in your environment variables for daily email digests.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request
