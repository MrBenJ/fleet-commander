#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 1 ]; then
    echo "Usage: $0 <worktree-path>"
    echo "Builds the fleet binary from a worktree and copies it here."
    exit 1
fi

WORKTREE="$1"

if [ ! -d "$WORKTREE/cmd/fleet" ]; then
    echo "Error: '$WORKTREE' doesn't look like a fleet-commander worktree (no cmd/fleet/)"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "Building fleet from $WORKTREE..."
(cd "$WORKTREE" && go build -o "$SCRIPT_DIR/fleet" ./cmd/fleet/)
echo "Done — binary at $SCRIPT_DIR/fleet"
