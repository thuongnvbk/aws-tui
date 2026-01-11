# AGENTS.md - SRE Guide for AWS TUI

This document provides guidance for AI agents and SREs working with the AWS TUI codebase.

## Project Overview

AWS TUI is a Go-based Terminal User Interface for managing AWS resources. It provides a keyboard-driven, Vim-style interface for common SRE operations without memorizing CLI commands.

**Key Technologies:**
- Go 1.23+
- Bubble Tea (TUI framework)
- AWS SDK v2

**Supported AWS Services:** EC2, EKS, ECS, S3, RDS, Lambda, MSK (Kafka)

## Repository Structure

```
aws-tui/
├── cmd/aws-tui/main.go      # Application entry point
├── internal/
│   ├── aws/                  # AWS service integrations
│   │   ├── client.go        # AWS SDK client initialization
│   │   ├── ec2.go           # EC2 operations (start/stop/reboot)
│   │   ├── eks.go           # EKS cluster management
│   │   ├── ecs.go           # ECS cluster/service viewing
│   │   ├── msk.go           # MSK/Kafka operations
│   │   ├── resources.go     # RDS, Lambda, S3 bucket listing
│   │   └── s3_objects.go    # S3 object browsing
│   ├── config/config.go     # YAML configuration management
│   └── ui/                   # Terminal UI components
│       ├── model.go         # Bubble Tea model & state
│       ├── view.go          # View rendering
│       ├── s3_browser.go    # S3 browser UI
│       └── s3_handlers.go   # S3 event handling
├── config.example.yaml       # Configuration template
└── Makefile                  # Build targets
```

## Quick Start for SREs

### Prerequisites
1. AWS credentials configured (`~/.aws/credentials` or environment variables)
2. Go 1.21+ installed
3. Appropriate IAM permissions for target resources

### Build & Run
```bash
make build          # Build binary to ./build/aws-tui
make run            # Build and run immediately
make install        # Install to $GOPATH/bin
```

### Configuration
Copy and customize the configuration:
```bash
mkdir -p ~/.aws-tui
cp config.example.yaml ~/.aws-tui/config.yaml
```

## Common SRE Workflows

### EC2 Instance Management
- **List instances**: Navigate to EC2 tab, view running/stopped instances
- **Start/Stop**: Press `s` (start) or `S` (stop) on selected instance
- **Reboot**: Press `r` on selected instance in detail view
- **View details**: Press `Enter` to see instance metadata, IPs, security groups

### EKS Cluster Operations
- **View clusters**: Navigate to EKS tab
- **Node groups**: Press `Enter` to view node group scaling info
- **Tags**: View cluster tags and configuration

### S3 Object Investigation
- **Browse buckets**: Navigate to S3 tab
- **Search objects**: Press `/` for regex search
- **Preview files**: Press `v` on text files (JSON, YAML, logs)
- **Download**: Press `d` to download locally
- **Presigned URLs**: Press `u` for shareable URLs (15-min expiry)

### MSK/Kafka Troubleshooting
- **View clusters**: Check broker nodes, authentication settings
- **Topics**: View Kafka topic list and partition counts
- **SCRAM secrets**: View authentication credentials

## Key Files for Modifications

| Task | Files to Modify |
|------|-----------------|
| Add new AWS service | `internal/aws/` + `internal/ui/model.go` |
| Modify UI rendering | `internal/ui/view.go` |
| Change key bindings | `internal/ui/model.go` (Update function) |
| Add configuration options | `internal/config/config.go` |
| EC2 operations | `internal/aws/ec2.go` |
| S3 features | `internal/aws/s3_objects.go`, `internal/ui/s3_*.go` |

## Testing & Validation

### Before Deploying Changes
```bash
make test           # Run unit tests
make build          # Verify compilation
./build/aws-tui     # Manual testing
```

### Testing AWS Integrations
1. Use a non-production AWS profile
2. Verify IAM permissions match required actions
3. Test with limited resources first (filter by tags/regions)

## Incident Response Patterns

### Quick EC2 Troubleshooting
1. Launch TUI: `aws-tui --profile production`
2. Navigate to EC2 tab
3. Identify unhealthy instances (check state, status checks)
4. View instance details for IPs and security groups
5. Use action to restart if needed

### S3 Log Investigation
1. Navigate to S3 → target bucket
2. Use `/` search with regex patterns for timestamps/errors
3. Preview log files inline with `v`
4. Download relevant files with `d`

### EKS/ECS Service Health
1. Navigate to EKS or ECS tab
2. Check running task counts vs desired
3. View node group scaling status
4. Identify capacity issues

## Security Considerations

### IAM Permissions Required
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:Describe*",
        "ec2:StartInstances",
        "ec2:StopInstances",
        "ec2:RebootInstances",
        "eks:Describe*",
        "eks:List*",
        "ecs:Describe*",
        "ecs:List*",
        "s3:ListBucket",
        "s3:GetObject",
        "rds:Describe*",
        "lambda:List*",
        "lambda:GetFunction",
        "kafka:Describe*",
        "kafka:List*",
        "secretsmanager:GetSecretValue"
      ],
      "Resource": "*"
    }
  ]
}
```

### Credential Safety
- Never commit AWS credentials to repository
- Use IAM roles when running on EC2/ECS
- Prefer temporary credentials (STS assume-role)
- Audit actions via CloudTrail

## Debugging the Application

### Enable Debug Logging
In `~/.aws-tui/config.yaml`:
```yaml
logging:
  enabled: true
  level: debug
  file: ~/.aws-tui/debug.log
```

### Common Issues

| Issue | Solution |
|-------|----------|
| "No credentials found" | Check `~/.aws/credentials` or environment vars |
| "Access denied" | Verify IAM permissions for the operation |
| "Region not found" | Ensure region is valid in config |
| UI rendering issues | Try different terminal emulator |
| Slow S3 listing | Enable pagination, use search filters |

## Architecture Notes for Agents

### State Management
- Uses Bubble Tea's Elm-like architecture
- State contained in `ui.Model` struct
- Updates via message passing (non-blocking)
- Async operations use Go routines with Tea commands

### AWS Client Pattern
```go
// internal/aws/client.go - Central client initialization
type Client struct {
    cfg       aws.Config
    ec2Client *ec2.Client
    // ... other service clients
}
```

### Adding New Features
1. Define data structures in `internal/aws/`
2. Implement AWS API calls
3. Add UI state to `internal/ui/model.go`
4. Implement view rendering in `internal/ui/view.go`
5. Handle keyboard events in `Update()` function
6. Update configuration if needed

### Code Style
- Follow Go idioms and conventions
- Use context for cancellation
- Handle errors explicitly
- Keep UI logic separate from AWS logic

## Performance Considerations

- S3 regex patterns are cached (max 50 patterns)
- Content preview limited to 100KB by default
- Pagination used for large resource lists
- Async loading with spinner indicators

## Monitoring & Observability

### Recommended Metrics to Track
- AWS API call latency
- Error rates per service
- Resource counts per region
- User action frequency

### Integration Points
- CloudWatch for AWS metrics
- Application logs for TUI operations
- AWS Config for resource compliance

## Contributing Guidelines

1. Create feature branch from main
2. Follow existing code patterns
3. Add tests for new AWS operations
4. Update README.md for user-facing changes
5. Test with multiple AWS profiles/regions
6. Verify cross-platform compatibility (Linux, macOS, Windows)

## Related Documentation

- [README.md](README.md) - User documentation and installation
- [S3_BROWSER_IMPLEMENTATION.md](S3_BROWSER_IMPLEMENTATION.md) - S3 feature details
- [S3_SEARCH_FEATURE.md](S3_SEARCH_FEATURE.md) - Regex search implementation
- [config.example.yaml](config.example.yaml) - Full configuration reference
