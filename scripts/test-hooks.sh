#!/bin/sh
# Tests the git hooks in a throwaway repository, so a mistake in a hook shows here and not as a blocked commit.
# Run:   sh scripts/test-hooks.sh        (CI runs it too)

here=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

cd "$work" || exit 1
git init -q .
git config user.email "test@example.com"
git config user.name "Hook Test"
git config commit.gpgsign false
git config core.autocrlf false
cp -R "$here/.githooks" .githooks
chmod +x .githooks/*
git config core.hooksPath .githooks
git commit -q --allow-empty --no-verify -m "chore: start"

passed=0
failed=0
# expect <pass|fail> <description> <command...>: the command must succeed (pass) or be refused (fail).
expect() {
  want=$1
  what=$2
  shift 2
  if "$@" >/dev/null 2>&1; then got=pass; else got=fail; fi
  if [ "$got" = "$want" ]; then
    passed=$((passed + 1))
  else
    failed=$((failed + 1))
    printf 'FAIL  %s (expected it to %s, it did not)\n' "$what" "$want"
  fi
}

clean() {
  git reset -q 2>/dev/null
  git clean -fdq -e .githooks
}

message() { git commit -q --allow-empty -m "$1"; }
stage() { git add -f -- "$@"; }

# ---- commit messages -------------------------------------------------------------------------------------------

expect pass "a conventional message"            message "feat(go-api): add API keys"
expect pass "a type without a scope"            message "docs: describe the API"
expect pass "a breaking change mark"            message "feat(api)!: rename a route"
expect pass "a dependency bump"                 message "chore(deps): bump gin from 1.9 to 1.10"
expect pass "a merge commit"                    message "Merge pull request #12 from x/y"
expect pass "a revert made by git"              message "Revert \"feat: add keys\""
expect fail "no type"                           message "Added some things"
expect fail "an unknown type"                   message "feature: add keys"
expect fail "no space after the colon"          message "feat:add keys"
expect fail "an empty description"              message "feat: "
expect fail "a first line over 100 characters"  message "feat: $(printf 'x%.0s' $(seq 1 100))"
expect fail "a Co-authored-by line"             message "feat: add keys

Co-Authored-By: Someone <someone@example.com>"
expect fail "a Signed-off-by line"              message "feat: add keys

Signed-off-by: Someone <someone@example.com>"
expect fail "the name of an AI assistant"       message "feat: add keys with help from Claude"
expect fail "a generated-with line"             message "feat: add keys

Generated with a tool"
expect fail "a session link"                    message "feat: add keys

session_01abcdef"

# ---- what is committed -----------------------------------------------------------------------------------------

check() { sh .githooks/pre-commit; }

clean; echo "KEY=1" > .env; stage .env
expect fail "a .env file" check
clean; echo "KEY=" > .env.example; stage .env.example
expect pass "a .env.example file" check
clean; echo x > server.pem; stage server.pem
expect fail "a .pem file" check

# Built from pieces so that this very file does not look like it holds a secret.
fake_groq="gsk""_abcdefghijklmnopqrstuvwxyz0123456789"
fake_pem="-----BEGIN ""PRIVATE KEY-----"
fake_ink="ink""_$(printf 'a%.0s' $(seq 1 43))"
clean; echo "GROQ_API_KEY=$fake_groq" > settings.txt; stage settings.txt
expect fail "a Groq key in a file" check
clean; echo "$fake_pem" > note.txt; stage note.txt
expect fail "a private key in a file" check
clean; echo "token: $fake_ink" > note.txt; stage note.txt
expect fail "an Inkling API key in a file" check
clean; echo "Authorization: Bearer ink_YOUR_KEY and ink_AbCd1234..." > note.md; stage note.md
expect pass "an example key in the docs" check
clean; echo "GROQ_API_KEY=your-key-here" > note.md; stage note.md
expect pass "a placeholder in the docs" check

clean; head -c 6000000 /dev/zero > big.bin; stage big.bin
expect fail "a 6 MB file" check
clean; head -c 1000000 /dev/zero > ok.bin; stage ok.bin
expect pass "a 1 MB file" check

clean; mkdir -p go-api/migrations; echo "select 1;" > go-api/migrations/000001_x.up.sql; stage go-api/migrations/000001_x.up.sql
expect fail "a migration without its undo" check
echo "select 2;" > go-api/migrations/000001_x.down.sql; stage go-api/migrations/000001_x.down.sql
expect pass "a migration with its undo" check

if command -v gofmt >/dev/null 2>&1; then
  clean; mkdir -p go-api; printf 'package a\n\nfunc  f( ) {}\n' > go-api/a.go; stage go-api/a.go
  expect fail "an unformatted Go file" check
  clean; mkdir -p go-api; printf 'package a\n\nfunc f() {}\n' > go-api/a.go; stage go-api/a.go
  expect pass "a formatted Go file" check
  # what is staged counts, not what is on disk
  printf 'package a\n\nfunc  f( ) {}\n' > go-api/a.go
  expect pass "a formatted Go file that was messed up again after staging" check
fi

clean; mkdir -p frontend; echo "export const a = 1;" > frontend/a.ts; stage frontend/a.ts
expect pass "frontend code when it cannot be linted here (skipped)" check

clean; echo "hello" > README.md; stage README.md
expect pass "an ordinary change" check

printf '%s checks passed, %s failed\n' "$passed" "$failed"
[ "$failed" -eq 0 ]
