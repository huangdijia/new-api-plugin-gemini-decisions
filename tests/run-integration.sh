#!/usr/bin/env bash
set -euo pipefail

# Run against an isolated new-api v1.0.0-rc.49 source tree. The test uses the
# host's real router, moejs, SQLite and billing session; Google is local only.
test_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
host_dir="${1:?Usage: tests/run-integration.sh /path/to/new-api-v1.0.0-rc.49}"
host_dir="$(cd "$host_dir" && pwd)"
test_temp="$(mktemp -d "$host_dir/router/gemini-decisions-test.XXXXXXXX")"
test_target="$host_dir/router/$(basename "$test_temp" | tr '.-' '__')_test.go"
test_source="$test_temp/owned-integration-source"
cleanup() {
    status=$?
    trap - EXIT
    # Only unlink our own inode; a collision or replacement belongs to others.
    if [[ ! -L "$test_target" && "$test_target" -ef "$test_source" ]]; then
        rm -f -- "$test_target"
    fi
    # POSIX ln treats an existing directory as a destination directory. Account
    # for a directory appearing between the check and ln without deleting it.
    if [[ ! -L "$test_target" && ! -L "$test_target/owned-integration-source" &&
          "$test_target/owned-integration-source" -ef "$test_source" ]]; then
        rm -f -- "$test_target/owned-integration-source"
    fi
    rm -f -- "$test_source"
    rmdir -- "$test_temp"
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
# Keep both paths on the same filesystem. ln atomically refuses to overwrite
# an existing file, unlike a separate existence check followed by cp.
cp "$test_dir/host_integration_test.go" "$test_source"
if [[ -e "$test_target" || -L "$test_target" ]]; then
    printf 'Refusing to overwrite existing integration test: %s\n' "$test_target" >&2
    exit 1
fi
ln -n "$test_source" "$test_target"
if [[ -L "$test_target" || ! "$test_target" -ef "$test_source" ]]; then
    printf 'Integration test destination changed during creation: %s\n' "$test_target" >&2
    exit 1
fi
export GEMINI_DECISIONS_PLUGIN_SOURCE="$test_dir/../plugin.js"
cd "$host_dir"
go test ./router -run '^TestGeminiDecisionsIntegration$' -count=1 -v -timeout=180s
