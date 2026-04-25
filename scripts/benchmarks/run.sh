#!/usr/bin/env bash
set -euo pipefail

SCENARIO="${1:-all}"
OUTPUT_ROOT="${2:-}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HARNESS_PATH="${REPO_ROOT}/tests/benchmarks/e2e/harness.py"

if [[ -n "${OUTPUT_ROOT}" ]]; then
  python "${HARNESS_PATH}" "${SCENARIO}" --output-root "${OUTPUT_ROOT}"
else
  python "${HARNESS_PATH}" "${SCENARIO}"
fi

