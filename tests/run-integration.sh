#!/usr/bin/env bash
set -euo pipefail

# Run against an isolated rc.47 or rc.49 source tree. The test uses the host's
# real router, moejs, SQLite and billing session; Google is local only.
test_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
host_dir="${1:?Usage: tests/run-integration.sh /path/to/new-api [rc.47|rc.49]}"
host_version="${2:-rc.49}"
case "$host_version" in
    rc.47|rc.49) ;;
    *) printf 'Unsupported test host version: %s\n' "$host_version" >&2; exit 2 ;;
esac
host_dir="$(cd "$host_dir" && pwd)"
test_temp="$(mktemp -d "${TMPDIR:-/tmp}/gemini-decisions-integration.XXXXXXXX")"
test_target="$host_dir/router/$(basename "$test_temp" | tr '.-' '__')_test.go"
overlay_file="$test_temp/overlay.json"
cleanup() {
    status=$?
    trap - EXIT
    if [[ -f "$overlay_file" && ! -L "$overlay_file" ]]; then
        rm -- "$overlay_file"
    fi
    rmdir -- "$test_temp"
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
if [[ -e "$test_target" || -L "$test_target" ]]; then
    printf 'Refusing to overlay existing integration test: %s\n' "$test_target" >&2
    exit 1
fi
# Go's overlay makes the integration test visible without modifying the host.
python3 - "$test_target" "$test_dir/host_integration_test.go" "$overlay_file" <<'PY'
import json
import sys

with open(sys.argv[3], "x", encoding="utf-8") as output:
    json.dump({"Replace": {sys.argv[1]: sys.argv[2]}}, output)
PY
export GEMINI_DECISIONS_PLUGIN_SOURCE="$test_dir/../plugin.js"
export GEMINI_DECISIONS_HOST_VERSION="$host_version"
cd "$host_dir"
go test -overlay="$overlay_file" ./router -run '^TestGeminiDecisionsIntegration$' -count=1 -v -timeout=180s
