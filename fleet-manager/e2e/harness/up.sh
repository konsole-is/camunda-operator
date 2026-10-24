#!/usr/bin/env bash
# Creates the kind cluster of the fleet manager journeys and installs what
# the journeys need: the operator and the fleet manager from the chart, a Git
# server with a private CA, RustFS, the fake plugin, and one sample
# CamundaCluster with its two PluginRequests. CI only: never run it against a
# cluster that you use.
#
# Usage: up.sh (make fleet-harness-up runs it)
#
# Inputs:
#   KIND_CLUSTER        the name of the kind cluster (default fleet-harness)
#   FLEET_HARNESS_WORK  the directory for the CA files (default
#                       ${RUNNER_TEMP:-<new temp dir>}/fleet-harness). check.sh
#                       reads ca.crt from it.
set -euo pipefail

harness=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo=$(cd "$harness/../../.." && pwd)
cluster=${KIND_CLUSTER:-fleet-harness}
work=${FLEET_HARNESS_WORK:-${RUNNER_TEMP:-$(mktemp -d)}/fleet-harness}
operator_image=controller:harness
fleet_image=fleet-manager:harness
fakeplugin_image=fleet-harness-fakeplugin:dev
gitserver_image=fleet-harness-gitserver:dev
access_url=https://camundacluster-sample-fleet-smoke.fleet-harness.test

cd "$repo"

echo "==> Creating the kind cluster $cluster"
kind create cluster --name "$cluster" --wait 120s

echo "==> Building the images and loading them into kind"
make docker-build IMG="$operator_image"
make docker-build-fleet FLEET_IMG="$fleet_image"
make docker-build-fakeplugin FAKEPLUGIN_IMG="$fakeplugin_image"
make docker-build-gitserver GITSERVER_IMG="$gitserver_image"
for image in "$operator_image" "$fleet_image" "$fakeplugin_image" "$gitserver_image"; do
  kind load docker-image --name "$cluster" "$image"
done

echo "==> Generating the chart"
make helm-generate IMG="$operator_image" FLEET_IMG="$fleet_image"

echo "==> Writing the private CA of the Git server into $work"
"$harness/tls.sh" "$work"
for namespace in camunda-operator-system fleet-harness fleet-harness-store fleet-smoke; do
  kubectl create namespace "$namespace" --dry-run=client -o yaml | kubectl apply -f -
done
kubectl create secret tls git-tls -n fleet-harness \
  --cert="$work/tls.crt" --key="$work/tls.key"
kubectl create configmap fleet-harness-ca -n camunda-operator-system \
  --from-file=ca.crt="$work/ca.crt"

echo "==> Starting the Git server"
sed "s|__GITSERVER_IMAGE__|$gitserver_image|" "$harness/git-server.yaml" | kubectl apply -f -
kubectl rollout status deployment/git -n fleet-harness --timeout=120s

echo "==> Installing the operator and the fleet manager from the chart"
make helm-deploy IMG="$operator_image" FLEET_IMG="$fleet_image" \
  HELM_EXTRA_ARGS="--set fleetManager.enabled=true --set fleetManager.extraCACerts.configMapName=fleet-harness-ca"
kubectl rollout status deployment/camunda-operator-fleet-manager -n camunda-operator-system --timeout=180s

echo "==> Starting RustFS and creating its bucket"
kubectl apply -n fleet-harness-store -f test/e2e/testdata/rustfs.yaml
kubectl wait --for=condition=complete job/rustfs-bucket -n fleet-harness-store --timeout=300s

echo "==> Starting the fake plugin"
sed -e "s|__FAKEPLUGIN_IMAGE__|$fakeplugin_image|" -e "s|__ACCESS_EXTERNAL_URL__|$access_url|" \
  "$harness/fakeplugin.yaml" | kubectl apply -f -
kubectl rollout status deployment/fakeplugin -n fleet-harness --timeout=120s

echo "==> Applying the sample CamundaCluster"
# The sample names a preset, a release, and storage that do not exist, so the
# operator starts no Camunda pod. The pages need only the object.
kubectl apply -n fleet-smoke -f config/samples/core_v1_camundacluster.yaml

echo "==> Applying the bucket contract and the PluginRequests of the sample"
kubectl apply -f "$harness/backup-storage.yaml"
kubectl apply -f "$harness/requests.yaml"

echo "==> The harness is up"
