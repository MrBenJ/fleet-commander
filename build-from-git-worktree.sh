#!/usr/bin/env bash
set -euo pipefail

if [ $# -lt 1 ]; then
    echo "Usage: $0 <worktree-name>"
    echo "Builds the fleet binary from a git worktree and copies it here."
    echo ""
    echo "Available worktrees:"
    git worktree list
    exit 1
fi

NAME="$1"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Find the worktree path by matching the name against git worktree list
WORKTREE=$(git worktree list --porcelain | awk -v name="$NAME" '
    /^worktree / { path = substr($0, 10) }
    /^branch /   { branch = substr($0, 8); sub(/.*\//, "", branch) }
    /^$/ { if (path ~ name || branch ~ name) { print path; exit } }
    END {}
')

if [ -z "$WORKTREE" ]; then
    echo "Error: no worktree matching '$NAME'"
    echo ""
    echo "Available worktrees:"
    git worktree list
    exit 1
fi

if [ ! -d "$WORKTREE/cmd/fleet" ]; then
    echo "Error: '$WORKTREE' doesn't have cmd/fleet/"
    exit 1
fi

echo "Building fleet from $WORKTREE..."
(cd "$WORKTREE" && go build -o "$SCRIPT_DIR/fleet" ./cmd/fleet/)
echo "Done — binary at $SCRIPT_DIR/fleet"
