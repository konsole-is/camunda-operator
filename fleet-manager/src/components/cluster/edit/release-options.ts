import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import type { Resource } from "@/lib/k8s/kinds";
import {
  moveStatus,
  releaseAllowed,
  releaseMove,
  versionCheckWait,
} from "@/lib/policy";
import { parseVersion } from "@/lib/policy/release";
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
 * The texts come from moveStatus. A backup never disables an option: the
 * Claim tab warns on a running backup instead. note is versionCheckWait of
 * the cluster. Then no option is disabled or a downgrade.
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
  const note = versionCheckWait(cluster);
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
      // The select lists only allowed releases and the current one, and the
      // backup warning of the Claim tab covers a backup.
      const status = moveStatus(
        { ...move, allowed: true, backup: undefined },
        cluster,
      );
      return {
        name,
        version: r.spec.version,
        isDefault: environment?.spec.releases?.default === name,
        isCurrent: name === current,
        ...(status.kind === "skipsMinor" && { disabled: status.text }),
        ...(status.kind === "downgrade" && {
          downgrade: { from: status.from, to: status.to },
        }),
      };
    });
  options.sort(
    (a, b) =>
      compareVersions(b.version, a.version) || a.name.localeCompare(b.name),
  );
  return { options, ...(note && { note }) };
}

/**
 * createReleaseChoices returns the releases that a new cluster can use: those
 * that environment allows, every release without an Environment, newest
 * first. The default release of environment is marked. No option is
 * current, disabled, or a downgrade, because a new cluster runs no version.
 */
export function createReleaseChoices(
  releases: readonly Resource<"CamundaRelease">[],
  environment: Environment | undefined,
): ReleaseOption[] {
  return releases
    .filter((r) => releaseAllowed(r.metadata?.name ?? "", environment))
    .map((r) => {
      const name = r.metadata?.name ?? "";
      return {
        name,
        version: r.spec.version,
        isDefault: environment?.spec.releases?.default === name,
        isCurrent: false,
      };
    })
    .sort(
      (a, b) =>
        compareVersions(b.version, a.version) || a.name.localeCompare(b.name),
    );
}

/**
 * releaseLabel returns the text of a release option: its version and name,
 * then the reason when it is disabled, else its marks Default and
 * downgrade.
 */
export function releaseLabel(
  release: {
    name: string;
    version: string;
    disabled?: string;
    downgrade?: { from: string; to: string };
  },
  isDefault: boolean,
): string {
  const label = `${release.version} (${release.name})`;
  if (release.disabled) return `${label}: ${release.disabled}`;
  const marks = [
    isDefault && "Default",
    release.downgrade && "downgrade",
  ].filter(Boolean);
  return marks.length > 0 ? `${label}, ${marks.join(", ")}` : label;
}

// A version that parseVersion cannot read is lower than every other.
function compareVersions(a: string, b: string): number {
  const x = parseVersion(a) ?? [-1, -1, -1];
  const y = parseVersion(b) ?? [-1, -1, -1];
  return x[0] - y[0] || x[1] - y[1] || x[2] - y[2];
}
