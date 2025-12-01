# AWS TUI

A fast, intuitive Terminal User Interface for managing AWS resources. Stop memorizing CLI commands and start being productive.

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/license-MIT-green)

## Features

- **Multi-resource support**: EC2, EKS, ECS, S3, RDS, Lambda
- **S3 Browser**: Navigate bucket contents, view objects, download files
- **Profile switching**: Quickly switch between AWS profiles
- **Region switching**: Change regions on the fly
- **YAML configuration**: Customize everything via config file
- **Keyboard-driven**: Vim-style navigation (hjkl)
- **Fast**: Built with Go, instant response times
- **Searchable**: Filter resources by name

## Installation

### From Source

```bash
git clone https://github.com/yourusername/aws-tui.git
cd aws-tui
go build -o aws-tui ./cmd/aws-tui
```

### Using Go Install

```bash
go install github.com/yourusername/aws-tui/cmd/aws-tui@latest
```

## Configuration

Create a config file at `~/.aws-tui/config.yaml`:

```yaml
# AWS profiles (from ~/.aws/credentials)
profiles:
  - name: default
    region: us-east-1
    description: "Default Account"
  - name: production
    region: us-west-2
    description: "Production Environment"

default_profile: default

# Enable/disable resource types
resources:
  ec2:
    enabled: true
  eks:
    enabled: true
  ecs:
    enabled: true
  s3:
    enabled: true
  rds:
    enabled: true
  lambda:
    enabled: true

# UI settings
ui:
  theme: dark
  refresh_interval: 30
  show_counts: true
```

See `config.example.yaml` for full configuration options.

## Usage

```bash
# Run with default config
aws-tui

# Specify config file
aws-tui --config /path/to/config.yaml

# Override profile
aws-tui --profile production

# Override region
aws-tui --region eu-west-1
```

## Keyboard Shortcuts

### Navigation
| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `←` / `h` | Previous resource type |
| `→` / `l` | Next resource type |
| `Tab` | Next resource type |
| `Shift+Tab` | Previous resource type |
| `Enter` | View details |
| `Esc` | Go back |

### Actions
| Key | Action |
|-----|--------|
| `r` | Refresh |
| `p` | Switch profile |
| `R` | Switch region |
| `s` | Start instance (EC2) |
| `S` | Stop instance (EC2) |
| `c` | Copy resource ID |
| `/` | Search/Filter |

### S3 Browser (when viewing S3 buckets)
| Key | Action |
|-----|--------|
| `Enter` | Browse bucket / Open folder / View object |
| `Backspace` | Go up one level |
| `/` | Search objects with regex |
| `Esc` | Clear search filter / Go back |
| `v` | View object content (text files) |
| `d` | Download object |
| `c` | Copy object key / Generate presigned URL |

### General
| Key | Action |
|-----|--------|
| `?` | Help |
| `q` | Quit |

## Screenshots

```
☁️  AWS TUI                           Profile: production | Region: us-west-2

[EC2] [EKS] [ECS] [S3] [RDS] [Lambda]

┌─────────────────────────────────────────────────────────────────────────────┐
│ EC2 Instances (12)                                                          │
├─────────────────────────────────────────────────────────────────────────────┤
│ > web-server-01        i-0abc123... | t3.medium | running                   │
│   web-server-02        i-0def456... | t3.medium | running                   │
│   api-server-01        i-0ghi789... | t3.large  | running                   │
│   worker-01            i-0jkl012... | c5.xlarge | stopped                   │
│   bastion              i-0mno345... | t3.micro  | running                   │
└─────────────────────────────────────────────────────────────────────────────┘

Loaded EC2 Instances
↑/k up • ↓/j down • enter select • r refresh • p profile • ? help • q quit
```

## Prerequisites

- Go 1.21 or later
- AWS credentials configured (`~/.aws/credentials` or environment variables)
- Appropriate IAM permissions for the resources you want to view/manage

### Required IAM Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeInstances",
        "ec2:StartInstances",
        "ec2:StopInstances",
        "ec2:RebootInstances",
        "eks:ListClusters",
        "eks:DescribeCluster",
        "eks:ListNodegroups",
        "eks:DescribeNodegroup",
        "ecs:ListClusters",
        "ecs:DescribeClusters",
        "ecs:ListServices",
        "ecs:DescribeServices",
        "s3:ListAllMyBuckets",
        "rds:DescribeDBInstances",
        "lambda:ListFunctions"
      ],
      "Resource": "*"
    }
  ]
}
```

## Roadmap

- [ ] CloudWatch metrics integration
- [ ] CloudWatch Logs viewer
- [ ] Cost Explorer integration
- [ ] Resource tagging
- [ ] Multi-account support (AWS Organizations)
- [ ] Custom actions via config
- [ ] SSH tunneling for EC2
- [ ] Port forwarding for RDS

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) for details.
