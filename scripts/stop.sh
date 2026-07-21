#!/usr/bin/env bash
set -e

echo "Stopping worker..."
pkill -f "go run worker/main.go" 2>/dev/null || true
pkill -f "go-build.*main" 2>/dev/null || true

echo "Stopping API..."
pkill -f "go run api/main.go" 2>/dev/null || true
pkill -f "go-build.*api" 2>/dev/null || true

echo "Stopping UI..."
pkill -f "vite" 2>/dev/null || true

if [[ "${1}" == "--temporal" ]]; then
  echo "Stopping Temporal..."
  docker compose down
fi

echo "Done."
