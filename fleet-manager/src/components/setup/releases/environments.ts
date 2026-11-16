import type { Environment } from "@/generated/crds/environment";
import { byCodeUnit } from "@/lib/policy";
import { allowedIn, defaultIn } from "@/lib/usage";

/** ReleaseEnvironmentState is what one environment does with a release. */
export type ReleaseEnvironmentState = "Default" | "Allowed" | "Not allowed";

/**
 * releaseEnvironments returns each environment with its label, the display
 * name or else the name, and its state for release, sorted by label.
 * Default means that new clusters get the release when they name none.
 */
export function releaseEnvironments(
  release: string,
  environments: readonly Environment[],
): { name: string; label: string; state: ReleaseEnvironmentState }[] {
  const defaults = defaultIn(environments, "CamundaRelease", release);
  const allowed = allowedIn(environments, "CamundaRelease", release);
  return environments
    .map((environment) => {
      const name = environment.metadata.name ?? "";
      const state: ReleaseEnvironmentState = defaults.includes(name)
        ? "Default"
        : allowed.includes(name)
          ? "Allowed"
          : "Not allowed";
      return { name, label: environmentLabel(environment), state };
    })
    .sort((a, b) => byCodeUnit(a.label, b.label));
}

/** environmentLabel is the display name of environment, or its name without one. */
export function environmentLabel(environment: Environment): string {
  return environment.spec.displayName || (environment.metadata.name ?? "");
}
