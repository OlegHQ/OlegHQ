.PHONY: sync sync-dry build test

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
