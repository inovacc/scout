#!/usr/bin/env bash
# Verify scout binary is available for the MCP server.

if command -v scout &>/dev/null; then
  version=$(scout version 2>/dev/null || echo "unknown")
  echo "scout plugin: scout binary found (${version})"
else
  echo "scout plugin: scout binary not found on PATH"
  echo "  Install with: go install github.com/inovacc/scout/cmd/scout@latest"
fi

exit 0
