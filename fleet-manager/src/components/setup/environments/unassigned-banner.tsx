import { TriangleAlert } from "lucide-react";
import type { UnassignedNamespace } from "./rows";

/**
 * UnassignedBanner names the namespaces whose claims the claim controller
 * refuses: those without an environment and those whose label names an
 * environment that does not exist. It renders nothing for no
 * namespace.
 */
export function UnassignedBanner({
  namespaces,
}: {
  namespaces: readonly UnassignedNamespace[];
}) {
  const names = (reason: UnassignedNamespace["reason"]) =>
    namespaces.filter((n) => n.reason === reason).map((n) => n.name);
  const none = names("NoEnvironment");
  const unknown = names("UnknownEnvironment");
  if (none.length + unknown.length === 0) return null;
  return (
    <div
      role="note"
      aria-label="Namespaces without an environment"
      className="flex gap-3 rounded-md border border-warn/40 bg-warn/10 p-3 text-sm text-text"
    >
      <TriangleAlert aria-hidden className="mt-0.5 size-4 shrink-0 text-warn" />
      <div className="flex flex-col gap-1">
        {none.length > 0 && (
          <p>
            These namespaces hold clusters and have no environment:{" "}
            {none.join(", ")}. New clusters in these namespaces are refused, and
            their claims take no change (NoEnvironment).
          </p>
        )}
        {unknown.length > 0 && (
          <p>
            These namespaces name an environment that does not exist:{" "}
            {unknown.join(", ")}. New clusters in them are refused, and their
            claims take no change (UnknownEnvironment).
          </p>
        )}
      </div>
    </div>
  );
}
