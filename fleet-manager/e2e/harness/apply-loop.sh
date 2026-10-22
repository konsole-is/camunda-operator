#!/usr/bin/env bash
# The GitOps loop of the harness. It applies the folder fleet of each new
# commit of the branch main of the in-cluster repository to the cluster.
#
# Usage: apply-loop.sh (it runs until it is stopped)
#
# Inputs:
#   APPLY_INTERVAL  the seconds between two reads of the branch (default 5)
#   GIT_PATH        the folder of the repository to apply (default fleet)
#
# kubectl apply -R reads the files in name order, so the namespaces in
# _cluster/ come before the namespace folders (contract 6.3). A commit whose
# apply fails stays unapplied, and the next pass tries it again.
set -euo pipefail

interval=${APPLY_INTERVAL:-5}
path=${GIT_PATH:-fleet}
applied=""

git_in_pod() {
  kubectl exec -n fleet-harness deploy/git -- git -C /srv/git/fleet.git "$@"
}

while true; do
  sha=$(git_in_pod rev-parse --verify -q refs/heads/main 2>/dev/null || true)
  if [ -n "$sha" ] && [ "$sha" != "$applied" ]; then
    tmp=$(mktemp -d)
    if git_in_pod archive --format=tar "$sha" | tar -x -C "$tmp"; then
      if [ ! -d "$tmp/$path" ]; then
        echo "commit $sha has no folder $path, nothing to apply"
        applied=$sha
      elif kubectl apply --server-side --field-manager=fleet-harness-gitops -R -f "$tmp/$path"; then
        echo "applied $sha"
        applied=$sha
      else
        echo "could not apply $sha, trying again"
      fi
    else
      echo "could not read $sha, trying again"
    fi
    rm -rf "$tmp"
  fi
  sleep "$interval"
done
