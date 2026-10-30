// This module runs in the browser and on the server.
import type { FleetEnvironmentConditionType } from "@/generated/conditions";
import type { Environment } from "@/generated/crds/environment";

const READY: FleetEnvironmentConditionType = "Ready";

/**
 * environmentReady says whether a claim can pick environment. Only
 * Ready=False makes it not ready, with the message of the condition. A new
 * Environment has no status yet and is ready.
 */
export function environmentReady(environment: Environment): {
  ready: boolean;
  message?: string;
} {
  const condition = environment.status?.conditions?.find(
    (c) => c.type === READY,
  );
  return condition?.status === "False"
    ? { ready: false, message: condition.message }
    : { ready: true };
}
