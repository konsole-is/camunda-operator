#!/usr/bin/env bash
# Runs a command against the fleet manager on the harness cluster, through a
# port-forward. make fleet-journeys passes the Playwright call. The command
# runs in fleet-manager/, so the screenshots land in fleet-manager/ui-journeys.
#
# Usage: journeys.sh <command...>
#
# Inputs:
#   FLEET_JOURNEYS_PORT  the local port of the port-forward (default 18080)
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.."
port=${FLEET_JOURNEYS_PORT:-18080}

rm -rf ui-journeys test-results
kubectl port-forward -n camunda-operator-system svc/camunda-operator-fleet-manager "127.0.0.1:$port:3000" >/dev/null 2>&1 &
forward=$!
trap 'kill "$forward" 2>/dev/null || true' EXIT

for _ in $(seq 30); do
  curl -fsS -o /dev/null "http://127.0.0.1:$port/api/healthz" && break
  sleep 1
done
curl -fsS -o /dev/null "http://127.0.0.1:$port/api/healthz" || {
  echo "FAIL: the fleet manager did not answer on 127.0.0.1:$port within 30 seconds" >&2
  exit 1
}

FLEET_JOURNEYS_BASE_URL="http://127.0.0.1:$port" \
  FLEET_JOURNEYS_CLUSTER=fleet-smoke/camundacluster-sample \
  "$@"
