#!/usr/bin/env bash
set -e

# Export AWS credentials so Terraform can read them
echo "Exporting AWS credentials..."
eval $(aws configure export-credentials --format env)

# Start Temporal server if not already running
if ! docker compose ps temporal 2>/dev/null | grep -q "running"; then
  echo "Starting Temporal server..."
  docker compose up -d
  echo "Waiting for Temporal to be ready..."
  sleep 5
fi

# Open 3 terminal tabs: worker, API, UI
if [[ "$OSTYPE" == "darwin"* ]]; then
  osascript \
    -e 'tell application "Terminal"' \
    -e '  set w to do script "cd '"$(pwd)"' && go run worker/main.go"' \
    -e '  do script "cd '"$(pwd)"' && go run api/main.go" in (do script "")' \
    -e '  do script "cd '"$(pwd)"'/ui && npm run dev" in (do script "")' \
    -e 'end tell'
else
  # Linux fallback: open separate gnome-terminal tabs
  gnome-terminal \
    --tab -- bash -c "go run worker/main.go; exec bash" \
    --tab -- bash -c "go run api/main.go; exec bash" \
    --tab -- bash -c "cd ui && npm run dev; exec bash"
fi

echo ""
echo "Started:"
echo "  Worker  → Terminal tab 1"
echo "  API     → http://127.0.0.1:8090"
echo "  UI      → http://localhost:5173"
echo "  Temporal UI → http://localhost:8080"
