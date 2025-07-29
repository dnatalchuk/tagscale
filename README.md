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

Migrations will also run automatically when starting the server or when using `--db` with the CLI.

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

By default the CLI stores all data in-memory. Use the `--db` flag to persist results
to the database configured by `DATABASE_URL`. When this flag is used the CLI
automatically runs any pending migrations.

### Example: In-memory scan and summary

Run a quick scan for the last 7 days without saving results to a database and then
print a summary:

```bash
./bin/tagscale-cli scan --days 7
./bin/tagscale-cli summary
```

Sample output:

```text
🔍 Running TagScale scan...
✅ Cost data collected.

💰 Total Cost: $123.45
🏷️ Untagged Cost: $45.67 (37.0%)

Top Services:
 • AmazonEC2                    $100.00
 • AmazonS3                     $20.00

Insights:
 • 37.0% of your costs are untagged. Consider implementing tagging policies.
```

## Configuration

### Environment Variables

 - `DATABASE_URL` (optional): PostgreSQL connection string used with `--db` or when running the server
- `AWS_REGION`: AWS region for Cost Explorer
- `SLACK_TOKEN`: Slack bot token for notifications
- `EMAIL_SMTP_*`: Email configuration for notifications
- `DATA_COLLECTION_INTERVAL`: How often to collect cost data (minutes)
- `ANALYSIS_INTERVAL`: How often to run cost analysis (minutes)
- `API_KEY` (optional): API key required for API requests. Send as `Authorization: Bearer <API_KEY>`
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

All `/api/v1` routes require the `API_KEY` if one is configured. Include it in requests as:

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
