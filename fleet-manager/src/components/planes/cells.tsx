import type { Plane } from "./plane";

/** PlaneName is the name of plane over its namespace, and says when the fleet manager does not manage it. */
export function PlaneName({ plane }: { plane: Plane }) {
  return (
    <span className="flex flex-col">
      <span className="font-medium">{plane.name}</span>
      <span className="text-xs text-muted">{plane.namespace}</span>
      {!plane.managed && (
        <span className="text-xs text-text-2">
          Not managed by the fleet manager
        </span>
      )}
    </span>
  );
}

/** PlaneServes is the environments of plane, or its raw namespace selector. */
export function PlaneServes({ plane }: { plane: Plane }) {
  return "environments" in plane.serves ? (
    <span>{plane.serves.environments.join(", ")}</span>
  ) : (
    <span className="font-mono text-xs break-words">
      {plane.serves.selector}
    </span>
  );
}

/** PlaneClusters is the count of the clusters that plane serves, and of those it does not. */
export function PlaneClusters({ plane }: { plane: Plane }) {
  const missing = plane.notAttached.length;
  return (
    <span className="whitespace-nowrap">
      {missing > 0
        ? `${plane.attached} attached · ${missing} not attached`
        : `${plane.attached} attached`}
    </span>
  );
}
