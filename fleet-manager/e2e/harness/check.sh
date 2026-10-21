#!/usr/bin/env bash
# Checks the harness: the fake plugin fills the outputs of both requests, a
# commit pushed to the in-cluster repository becomes a live object, and the
# fleet manager trusts the private CA of the Git server through the chart
# value fleetManager.extraCACerts. apply-loop.sh must run.
#
# Usage: check.sh
#
# Inputs:
#   FLEET_HARNESS_WORK  the directory that up.sh wrote ca.crt into (default
#                       ${RUNNER_TEMP}/fleet-harness)
set -euo pipefail

work=${FLEET_HARNESS_WORK:-${RUNNER_TEMP:?set FLEET_HARNESS_WORK or RUNNER_TEMP}/fleet-harness}
access_url=https://camundacluster-sample-fleet-smoke.fleet-harness.test

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

wait_output() {
  local request=$1 key=$2 want=$3
  kubectl wait -n fleet-smoke "pluginrequest/$request" --for=condition=OutputsReady --timeout=180s ||
    fail "the PluginRequest $request did not reach OutputsReady within 180 seconds"
  kubectl wait -n fleet-smoke "pluginrequest/$request" --for=jsonpath="{.status.outputs.$key}=$want" --timeout=30s ||
    fail "the PluginRequest $request has no output $key=$want"
  echo "ok: the PluginRequest $request has the output $key=$want"
}

echo "==> The fake plugin fills the outputs of the requests"
wait_output camundacluster-sample-access externalUrl "$access_url"
wait_output camundacluster-sample-backup-storage objectStorageConfig fleet-harness-backups

echo "==> A commit pushed to the repository becomes a live object"
clone=$(mktemp -d)
kubectl port-forward -n fleet-harness svc/git 18443:8443 >/dev/null 2>&1 &
forward=$!
trap 'kill "$forward" 2>/dev/null || true; rm -rf "$clone"' EXIT
for _ in $(seq 30); do
  curl -fsS --cacert "$work/ca.crt" -o /dev/null \
    "https://localhost:18443/fleet.git/info/refs?service=git-upload-pack" && break
  sleep 1
done

repo=$clone/fleet
harness_git() {
  git -c http.sslCAInfo="$work/ca.crt" -c user.name=fleet-harness -c user.email=harness@fleet.invalid "$@"
}
harness_git clone -q https://localhost:18443/fleet.git "$repo"
# The repository is empty, so the clone has no branch yet. Name it main.
harness_git -C "$repo" symbolic-ref HEAD refs/heads/main

want="${GITHUB_RUN_ID:-local}-${GITHUB_RUN_ATTEMPT:-$(date +%s)}"
mkdir -p "$repo/fleet/_cluster/namespaces" "$repo/fleet/fleet-harness-probe/configmaps"
cat >"$repo/fleet/_cluster/namespaces/fleet-harness-probe.yaml" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: fleet-harness-probe
YAML
cat >"$repo/fleet/fleet-harness-probe/configmaps/harness-probe.yaml" <<YAML
apiVersion: v1
kind: ConfigMap
metadata:
  name: harness-probe
  namespace: fleet-harness-probe
data:
  run: "$want"
YAML
harness_git -C "$repo" add .
harness_git -C "$repo" commit -q -m "Add the harness probe"
harness_git -C "$repo" push -q origin HEAD:refs/heads/main

export want
timeout 60 bash -c 'until [ "$(kubectl get configmap harness-probe -n fleet-harness-probe -o jsonpath={.data.run} 2>/dev/null)" = "$want" ]; do sleep 2; done' ||
  fail "the pushed commit did not appear as a live object within 60 seconds"
echo "ok: the pushed commit appeared as the ConfigMap fleet-harness-probe/harness-probe"

echo "==> The fleet manager trusts the private CA of the Git server"
# The second fetch runs without NODE_EXTRA_CA_CERTS and must fail on the
# certificate. Without it, a server with a public certificate passes too.
js=$(
  cat <<'JS'
const url = "https://git.fleet-harness.svc:8443/fleet.git/info/refs?service=git-receive-pack";
const fetchOnce = `fetch(${JSON.stringify(url)}).then(
  (r) => console.log(r.status, r.headers.get("content-type")),
  (e) => { console.log("error", e.cause?.code ?? e.message); process.exitCode = 1; },
)`;
const { spawnSync } = require("node:child_process");
const run = (env) => spawnSync(process.execPath, ["-e", fetchOnce], { env, encoding: "utf8" });
const withCA = run(process.env);
const env = { ...process.env };
delete env.NODE_EXTRA_CA_CERTS;
const withoutCA = run(env);
console.log("with: " + (withCA.stdout + withCA.stderr).trim());
console.log("without: " + (withoutCA.stdout + withoutCA.stderr).trim());
JS
)
out=$(kubectl exec -n camunda-operator-system deploy/camunda-operator-fleet-manager -- /nodejs/bin/node -e "$js")
echo "$out"
grep -q "^with: 200 application/x-git-receive-pack-advertisement" <<<"$out" ||
  fail "the fleet manager could not fetch the receive-pack endpoint over HTTPS with NODE_EXTRA_CA_CERTS"
grep -Eq "^without: .*(UNABLE_TO_VERIFY_LEAF_SIGNATURE|SELF_SIGNED_CERT_IN_CHAIN|UNABLE_TO_GET_ISSUER_CERT_LOCALLY)" <<<"$out" ||
  fail "the fetch without NODE_EXTRA_CA_CERTS did not fail on the certificate"
echo "ok: the fleet manager trusts the Git server through fleetManager.extraCACerts"
