#!/usr/bin/env bash
# Checks a fleet manager image: it runs as non-root on Node.js 26, the
# standalone server answers with a read-only root file system, the app reads
# the images table and the catalog at start, and it stops when one is missing.
#
# Usage: fleet-manager/deploy/test-image.sh <image>
set -euo pipefail

image="${1:?usage: $0 <image>}"
tool="${CONTAINER_TOOL:-docker}"
name="fleet-manager-image-test-$$"
trap '"$tool" rm -f "$name" >/dev/null 2>&1 || true' EXIT

fail() {
  echo "test-image: $*" >&2
  exit 1
}

user="$("$tool" image inspect --format '{{.Config.User}}' "$image")"
[[ "$user" == "65532:65532" ]] || fail "the image runs as '$user', want 65532:65532"

node="$("$tool" run --rm --entrypoint /nodejs/bin/node "$image" -p process.versions.node)"
[[ "$node" == 26.* ]] || fail "the image runs Node.js $node, want 26"

"$tool" run -d --name "$name" --read-only --tmpfs /app/.next/cache --tmpfs /tmp \
  -p 127.0.0.1::3000 "$image" >/dev/null
port="$("$tool" port "$name" 3000/tcp | head -n 1 | sed 's/.*://')"
for _ in $(seq 60); do
  curl -fsS "http://127.0.0.1:${port}/api/healthz" >/dev/null 2>&1 && break
  sleep 1
done
logs="$("$tool" logs "$name" 2>&1)"
curl -fsS "http://127.0.0.1:${port}/api/healthz" >/dev/null ||
  fail "the server did not answer on /api/healthz. Its log:"$'\n'"$logs"
grep -q '^Read the images table /app/data/images-table.json' <<<"$logs" ||
  fail "the app did not read the images table at start. Its log:"$'\n'"$logs"
grep -q 'and the catalog /app/data/official-plugins.json' <<<"$logs" ||
  fail "the app did not read the catalog at start. Its log:"$'\n'"$logs"

set +e
out="$(timeout 60 "$tool" run --rm -e FLEET_CATALOG=/app/data/missing.json "$image" 2>&1)"
code=$?
set -e
[[ $code -ne 0 && $code -ne 124 ]] || fail "the server did not stop without its catalog (exit $code)"
grep -q '/app/data/missing.json' <<<"$out" || fail "the start error does not name the missing file: $out"

echo "test-image: $image runs as 65532 on Node.js $node and reads its data at start"
