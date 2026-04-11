#!/usr/bin/env bash
set -euo pipefail

remove_volumes=0
keep_runtime_keys=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --remove-volumes)
      remove_volumes=1
      ;;
    --keep-runtime-keys)
      keep_runtime_keys=1
      ;;
    -h|--help)
      cat <<'USAGE'
Usage: ./down.sh [--remove-volumes] [--keep-runtime-keys]
USAGE
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
  shift
done

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../../.." && pwd)"
compose_file="${repo_root}/docker-compose.LocalE2E.yml"

compose_args=(-f "${compose_file}" down)
if [[ "${remove_volumes}" -eq 1 ]]; then
  compose_args+=(--volumes)
fi

docker compose "${compose_args[@]}"

runtime_ssh_dir="${repo_root}/.local-e2e/.runtime/ssh"
if [[ "${keep_runtime_keys}" -eq 0 ]]; then
  if [[ -d "${runtime_ssh_dir}" ]]; then
    rm -rf "${runtime_ssh_dir}"
  fi
  echo "Local E2E stand is down. Runtime SSH artifacts removed."
else
  echo "Local E2E stand is down. Runtime SSH artifacts were preserved."
fi
