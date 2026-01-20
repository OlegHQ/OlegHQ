.PHONY: sync sync-dry build test deploy

# Run the sync and push changes to the remote
sync:
	go run ./cmd/sync --repo github.com/oleghq/oleghq --prs-scope both

# Dry-run: preview changes without pushing
sync-dry:
	go run ./cmd/sync --dry-run --repo github.com/oleghq/oleghq --prs-scope both

# Build the binary
build:
	go build -o bin/sync ./cmd/sync

# Run tests
test:
	go test ./...

# Deploy/redeploy the scheduled sync task
deploy: build
	@echo "Deploying ghagent-sync scheduled task..."
	@smdctl rm -f ghagent-sync 2>/dev/null || true
	smdctl run -f smdctl.yml
	@echo ""
	@echo "Deployment complete!"
	@echo "View status: smdctl status ghagent-sync"
	@echo "View tasks: smdctl tasks ghagent-sync"
	@echo "View logs: smdctl logs -f ghagent-sync-task-daily-sync"
