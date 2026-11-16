import type { CamundaClusterPresetSpec } from "@/generated/crds/camundaclusterpreset";
import { equalJSON } from "@/lib/policy";

/**
 * renderedPreset returns a copy of spec as PresetFingerprint in
 * pkg/components/camundacluster/confighash.go hashes it: without the fields
 * that render into no pod template. A change of PresetFingerprint changes
 * this port.
 */
export function renderedPreset(
  spec: CamundaClusterPresetSpec,
): CamundaClusterPresetSpec {
  const rendered = structuredClone(spec);
  const cluster = rendered.cluster;
  // The wrappers go with the value: the first rotation of a preset adds the
  // blocks that carry it, and must not roll the workloads.
  if (cluster.auth) {
    if (cluster.auth.basic) {
      delete cluster.auth.basic.passwordRotation;
      delete cluster.auth.basic.adminEmail;
      if (Object.keys(cluster.auth.basic).length === 0)
        delete cluster.auth.basic;
    }
    if (Object.keys(cluster.auth).length === 0) delete cluster.auth;
  }
  if (cluster.connectors) {
    delete cluster.connectors.serviceAccount;
    if (Object.keys(cluster.connectors).length === 0) delete cluster.connectors;
  }
  return rendered;
}

/**
 * fingerprintMoved says whether the operator rolls the workloads that
 * inherit the preset when its spec changes from before to after. A cluster
 * rolls also when it sets the changed field itself.
 */
export function fingerprintMoved(
  before: CamundaClusterPresetSpec,
  after: CamundaClusterPresetSpec,
): boolean {
  return !equalJSON(renderedPreset(before), renderedPreset(after));
}
