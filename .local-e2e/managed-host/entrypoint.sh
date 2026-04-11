#!/usr/bin/env bash
set -euo pipefail

AUTHORIZED_KEY_PATH="${AUTHORIZED_KEY_PATH:-/run/dev-ssh/reconciler_local_e2e_key.pub}"
DOCKERD_START_TIMEOUT_SECONDS="${DOCKERD_START_TIMEOUT_SECONDS:-60}"

mkdir -p /var/run/sshd /root/.ssh /var/log
chmod 700 /root/.ssh

if [[ ! -f "${AUTHORIZED_KEY_PATH}" ]]; then
  echo "authorized key was not found at ${AUTHORIZED_KEY_PATH}" >&2
  exit 1
fi

cp "${AUTHORIZED_KEY_PATH}" /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys

ssh-keygen -A

dockerd --host=unix:///var/run/docker.sock --storage-driver=overlay2 >/var/log/dockerd.log 2>&1 &
DOCKERD_PID=$!

cleanup() {
  if kill -0 "${DOCKERD_PID}" >/dev/null 2>&1; then
    kill "${DOCKERD_PID}" >/dev/null 2>&1 || true
    wait "${DOCKERD_PID}" >/dev/null 2>&1 || true
  fi
}

trap cleanup EXIT INT TERM

for ((i=1; i<=DOCKERD_START_TIMEOUT_SECONDS; i++)); do
  if docker info >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! docker info >/dev/null 2>&1; then
  echo "dockerd did not become ready in ${DOCKERD_START_TIMEOUT_SECONDS} seconds" >&2
  exit 1
fi

exec /usr/sbin/sshd -D -e
