#!/usr/bin/env bash
set -euo pipefail

no_build=0
no_detach=0
no_force_recreate=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-build)
      no_build=1
      ;;
    --no-detach)
      no_detach=1
      ;;
    --no-force-recreate)
      no_force_recreate=1
      ;;
    -h|--help)
      cat <<'USAGE'
Usage: ./up.sh [--no-build] [--no-detach] [--no-force-recreate]
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

runtime_ssh_dir="${repo_root}/.local-e2e/.runtime/ssh"
private_key_path="${runtime_ssh_dir}/reconciler_local_e2e_key"
public_key_path="${runtime_ssh_dir}/reconciler_local_e2e_key.pub"

mkdir -p "${runtime_ssh_dir}"
rm -f "${private_key_path}" "${public_key_path}"

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/imp-local-e2e-key-XXXXXX")"
trap 'rm -rf "${tmp_dir}"' EXIT

tmp_private_key="${tmp_dir}/id_ed25519"
ssh-keygen -q -t ed25519 -N "" -C "imp-local-e2e-runtime" -f "${tmp_private_key}"

cp "${tmp_private_key}" "${private_key_path}"
chmod 600 "${private_key_path}"
ssh-keygen -y -f "${tmp_private_key}" > "${public_key_path}"

compose_args=(-f "${compose_file}" up)
if [[ "${no_build}" -eq 0 ]]; then
  compose_args+=(--build)
fi
if [[ "${no_detach}" -eq 0 ]]; then
  compose_args+=(-d)
fi
if [[ "${no_force_recreate}" -eq 0 ]]; then
  compose_args+=(--force-recreate)
fi

docker compose "${compose_args[@]}"
echo "Local E2E stand is up. Runtime SSH keys were generated in: ${runtime_ssh_dir}"
