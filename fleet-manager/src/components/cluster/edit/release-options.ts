import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import type { Resource } from "@/lib/k8s/kinds";
import { releaseAllowed, releaseMove } from "@/lib/policy";
import type { BackupObject } from "../backups";
import type { ClusterClaim } from "./cluster-edit";

/**
 * ReleaseOption is one release of the select. disabled holds the reason it
 * cannot be chosen. downgrade holds the versions of a downgrade.
 */
export type ReleaseOption = {
  name: string;
  version: string;
  isDefault: boolean;
  isCurrent: boolean;
  disabled?: string;
  downgrade?: { from: string; to: string };
};

/**
 * releaseChoices returns the releases that the Environment allows, newest
 * first, each with what releaseMove reports for it, and the current release
 * of the claim also when the Environment no longer allows it. The current
 * release is the release of the claim, else the release of the cluster.
 * note says why no version check runs: "The version checks run when the
 * cluster resumes." for a suspended cluster, "The version checks run when
 * the cluster reports its version." for a cluster without
 * status.management.version. Then no option is disabled or a downgrade.
 */
export function releaseChoices(input: {
  cluster: Resource<"CamundaCluster">;
  claim: ClusterClaim;
  releases: readonly Resource<"CamundaRelease">[];
  environment?: Resource<"Environment">;
  backups: readonly BackupObject[];
}): { options: ReleaseOption[]; note?: string } {
  const { cluster, claim, releases, backups } = input;
  // The cache objects differ from the generated types only in their
  // timestamps, which the policy library does not read.
  const environment = input.environment as Environment | undefined;
  const current = claim.spec?.releaseRef || cluster.spec.releaseRef;
  const note = cluster.spec.suspend
    ? "The version checks run when the cluster resumes."
    : cluster.status?.management?.version
      ? undefined
      : "The version checks run when the cluster reports its version.";
  const options = releases
    .filter((r) => {
      const name = r.metadata?.name ?? "";
      return name === current || releaseAllowed(name, environment);
    })
    .map((r): ReleaseOption => {
      const name = r.metadata?.name ?? "";
      const move = releaseMove(
        cluster,
        r as unknown as CamundaRelease,
        environment,
        backups,
      );
      const checked = note === undefined;
      return {
        name,
        version: r.spec.version,
        isDefault: environment?.spec.releases?.default === name,
        isCurrent: name === current,
        ...(checked &&
          move.skippedMinor && {
            disabled: `Skips ${move.skippedMinor}. Move to an ${move.skippedMinor} release first.`,
          }),
        ...(checked &&
          move.downgrade &&
          move.current && {
            downgrade: { from: move.current, to: move.target },
          }),
      };
    });
  options.sort(
    (a, b) =>
      compareVersions(b.version, a.version) || a.name.localeCompare(b.name),
  );
  return { options, ...(note && { note }) };
}

function compareVersions(a: string, b: string): number {
  const parts = (v: string) => v.split(".").map((n) => Number(n) || 0);
  const [x, y] = [parts(a), parts(b)];
  for (let i = 0; i < Math.max(x.length, y.length); i++) {
    const d = (x[i] ?? 0) - (y[i] ?? 0);
    if (d !== 0) return d;
  }
  return 0;
}
