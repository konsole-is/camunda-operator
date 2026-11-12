import { Info } from "lucide-react";
import type { EnvironmentImpact, ReferenceWarning } from "./impact";

/**
 * impactLines returns the text of impact: the new-claims-only line with the
 * count of the claims that keep their recorded policy, the namespaces that
 * a first environment leaves without one, then one line for each warning.
 */
export function impactLines(impact: EnvironmentImpact): string[] {
  return [
    countLine(impact),
    ...(impact.unassigned
      ? [
          `This is the first environment. The claims in ${impact.unassigned.join(", ")} have no environment label, so they take no change until their namespaces join an environment (NoEnvironment). Their clusters keep running.`,
        ]
      : []),
    ...impact.warnings.map(warningLine),
  ];
}

/** ImpactLine shows impactLines as a note above the change bar. */
export function ImpactLine({ impact }: { impact: EnvironmentImpact }) {
  const [count, ...warnings] = impactLines(impact);
  return (
    <div
      role="note"
      aria-label="Impact"
      className="flex gap-3 rounded-md border border-info/40 bg-info/10 p-3 text-sm text-text"
    >
      <Info aria-hidden className="mt-0.5 size-4 shrink-0 text-info" />
      <div className="flex flex-col gap-1.5">
        <p>{count}</p>
        {warnings.length > 0 && (
          <ul className="flex list-disc flex-col gap-1 pl-5 text-text-2">
            {warnings.map((line, i) => (
              <li key={i}>{line}</li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

function countLine({ clusters, planes }: EnvironmentImpact): string {
  const first = "Changes apply to new claims only.";
  const parts = [
    ...(clusters > 0 ? [plural(clusters, "cluster", "clusters")] : []),
    ...(planes > 0
      ? [plural(planes, "management plane", "management planes")]
      : []),
  ];
  if (parts.length === 0) return first;
  const one = clusters + planes === 1;
  return `${first} ${parts.join(" and ")} ${one ? "keeps the policy it" : "keep the policy they"} recorded at creation.`;
}

/** warningLine returns the text of one warning, as impactLines writes it. */
export function warningLine(warning: ReferenceWarning): string {
  const claim = `${warning.claim.name} in ${warning.claim.namespace}`;
  if ("removed" in warning) {
    const [what, field] =
      warning.kind === "CamundaRelease"
        ? ["release", "release"]
        : ["cluster preset", "preset"];
    return `${claim} uses ${what} ${warning.name}, which this change no longer allows. The cluster keeps running. A later change of its ${field} must pick an allowed one.`;
  }
  if ("cleared" in warning) {
    const what =
      warning.kind === "CamundaRelease" ? "release" : "platform config";
    const removed = `${claim} names no ${what}, and this change removes the default ${what}.`;
    return warning.claim.kind === "CamundaClusterClaim"
      ? `${removed} The cluster keeps running, but the claim is not ready and its cluster takes no further change.`
      : `${removed} Its management plane keeps running, but the claim reports MissingInput and takes no further change.`;
  }
  if (warning.kind === "CamundaRelease" && warning.refused)
    return `${claim} names no release, and its cluster refuses the move to ${warning.name} (${warning.refused}). The cluster keeps running and takes no further change.`;
  return warning.kind === "CamundaRelease"
    ? `${claim} names no release and moves to ${warning.name}. Its brokers restart one at a time.`
    : `${claim} names no platform config and moves to ${warning.name}.`;
}

function plural(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`;
}
