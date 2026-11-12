#!/usr/bin/env bash
set -euo pipefail

# Run against an isolated new-api v1.0.0-rc.49 source tree. The test uses the
# host's real router, moejs, SQLite and billing session; Google is local only.
test_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
host_dir="${1:?Usage: tests/run-integration.sh /path/to/new-api-v1.0.0-rc.49}"
host_dir="$(cd "$host_dir" && pwd)"
test_temp="$(mktemp -d "${TMPDIR:-/tmp}/gemini-decisions-test.XXXXXXXX")"
test_target="$host_dir/router/codex_$(basename "$test_temp" | tr '.-' '__')_test.go"
trap 'rm -f "$test_target"; rmdir "$test_temp"' EXIT
test ! -e "$test_target"
cp "$test_dir/host_integration_test.go" "$test_target"
export GEMINI_DECISIONS_PLUGIN_SOURCE="$test_dir/../plugin.js"
cd "$host_dir"
go test ./router -run '^TestGeminiDecisionsIntegration$' -count=1 -v -timeout=180s
