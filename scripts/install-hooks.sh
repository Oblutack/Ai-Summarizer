#!/bin/sh
# Turns on the project's git hooks (.githooks/) for this clone. Run once:   sh scripts/install-hooks.sh
set -e
cd "$(git rev-parse --show-toplevel)"
git config core.hooksPath .githooks
chmod +x .githooks/* 2>/dev/null || true
echo "Git hooks are on: .githooks/pre-commit checks what you commit, .githooks/commit-msg checks the message."
echo "Turn them off again with:   git config --unset core.hooksPath"
