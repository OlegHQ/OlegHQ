# GitHub Agent Documentation

## Overview

GitHub Agent syncs GitHub pull requests and issues to a local repository, keeping contribution data up to date automatically.

## Setup

### Prerequisites

- Go 1.x or later
- Git configured with GitHub access
- `smdctl` installed for systemd service management (optional, for scheduled tasks)

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd ghagent
```

2. Install dependencies:
```bash
go mod download
```

3. Verify setup:
```bash
make test
```

## Usage

### Manual Sync

Run a one-time sync to update all PRs and issues:

```bash
make sync
```

Preview changes without pushing to remote:

```bash
make sync-dry
```

### Build Binary

Build a standalone executable:

```bash
make build
# Binary will be in bin/sync
```

## Scheduled Sync (Automated)

### Deploy Daily Sync Task

The project includes a systemd timer configuration that runs the sync daily at **8:00 AM Zagreb timezone**.

#### Deploy the scheduled task:

```bash
make deploy
```

This command will:
- Remove any existing deployment
- Create a systemd user service
- Set up a timer to run at 8 AM Europe/Zagreb daily
- Enable and start the timer

#### Redeploy (update configuration):

```bash
make deploy
```

The deploy command handles both initial deployment and updates.

### Monitor Scheduled Tasks

View scheduled tasks:
```bash
smdctl tasks ghagent-sync
```

Check next run time:
```bash
systemctl --user list-timers smdctl-ghagent-sync-task-daily-sync.timer
```

View logs from scheduled sync runs:
```bash
smdctl logs -f ghagent-sync-task-daily-sync
```

View service status:
```bash
smdctl status ghagent-sync
```

### Stop Scheduled Sync

Remove the scheduled task:
```bash
smdctl rm ghagent-sync
```

## Configuration

### Sync Configuration

The sync is configured in the Makefile with the following parameters:

- **Repository**: `github.com/oleghq/oleghq`
- **PR Scope**: `both` (includes all PRs)

To modify these settings, edit the `Makefile`:

```makefile
sync:
	go run ./cmd/sync --repo github.com/oleghq/oleghq --prs-scope both
```

### Scheduled Task Configuration

The scheduled task is defined in `smdctl.yml`:

```yaml
name: ghagent-sync
description: GitHub Agent sync service with daily scheduled task
command: /usr/bin/true
workdir: /home/snowbear/projects/ghagent
restart: no

tasks:
  - name: daily-sync
    description: Run GitHub sync daily at 8 AM Zagreb time
    command: /usr/bin/make
    args: [sync]
    schedule:
      on_calendar: "*-*-* 08:00:00 Europe/Zagreb"
      persistent: true
```

**Persistent**: If the system is off at 8 AM, the sync will run when the system starts.

To change the schedule time, edit the `on_calendar` field and redeploy with `make deploy`.

## File Locations (Systemd User Mode)

- **Service file**: `~/.config/systemd/user/smdctl-ghagent-sync.service`
- **Timer file**: `~/.config/systemd/user/smdctl-ghagent-sync-task-daily-sync.timer`
- **Task service**: `~/.config/systemd/user/smdctl-ghagent-sync-task-daily-sync.service`
- **Environment**: `~/.config/smdctl/env/ghagent-sync-task-daily-sync.env`
- **Logs**: `~/.config/smdctl/logs/ghagent-sync-task-daily-sync.log`

## Troubleshooting

### Scheduled task not running

1. Check if timer is active:
```bash
systemctl --user list-timers | grep ghagent-sync
```

2. View recent logs:
```bash
smdctl logs -n 100 ghagent-sync-task-daily-sync
```

3. Check timer status:
```bash
systemctl --user status smdctl-ghagent-sync-task-daily-sync.timer
```

4. Manually trigger the task to test:
```bash
systemctl --user start smdctl-ghagent-sync-task-daily-sync.service
```

### Sync fails

Check the logs for error details:
```bash
smdctl logs ghagent-sync-task-daily-sync
```

Common issues:
- GitHub authentication: Ensure Git is configured with proper credentials
- Network connectivity: Check internet connection
- Repository access: Verify you have access to the target repository

### View dry-run to debug

Test sync without making changes:
```bash
make sync-dry
```

## Development

### Run Tests

```bash
make test
```

### Project Structure

```
ghagent/
├── cmd/
│   └── sync/           # Main sync command
├── internal/
│   ├── ghcli/         # GitHub CLI wrapper
│   ├── model/         # Data models
│   ├── render/        # Template rendering
│   └── sync/          # Sync logic
├── Makefile           # Build and deployment commands
├── smdctl.yml         # Systemd service configuration
└── DOCS.md           # This file
```

## Quick Reference

| Command | Description |
|---------|-------------|
| `make sync` | Run sync now |
| `make sync-dry` | Preview changes without pushing |
| `make build` | Build binary |
| `make test` | Run tests |
| `make deploy` | Deploy/redeploy scheduled task |
| `smdctl tasks ghagent-sync` | View scheduled tasks |
| `smdctl logs -f ghagent-sync-task-daily-sync` | Follow sync logs |
| `smdctl status ghagent-sync` | Check service status |
| `smdctl rm ghagent-sync` | Remove scheduled task |
