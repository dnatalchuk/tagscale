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

- Go 1.21+
- PostgreSQL 12+
- AWS credentials with Cost Explorer access
- Docker (optional)

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

4. Run database migrations:
```bash
make migrate
```

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

## Usage

Build the CLI tool:

```bash
make build-cli
```

The binary will be created at `bin/tagscale-cli`.

## Configuration

### Environment Variables

- `DATABASE_URL`: PostgreSQL connection string
- `AWS_REGION`: AWS region for Cost Explorer
- `SLACK_TOKEN`: Slack bot token for notifications
- `EMAIL_SMTP_*`: Email configuration for notifications
- `DATA_COLLECTION_INTERVAL`: How often to collect cost data (minutes)
- `ANALYSIS_INTERVAL`: How often to run cost analysis (minutes)

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
3. **Tag Analysis**: Extracts team info from existing tags
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
