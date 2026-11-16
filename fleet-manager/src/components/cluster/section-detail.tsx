import type { ReactNode } from "react";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import type { SectionComponent } from "@/lib/claims/sections";
import type { Resource } from "@/lib/k8s/kinds";
import { labelSelectorString } from "@/lib/labels";
import { ComponentDetail } from "./component-detail";

/**
 * SectionComponentDetail is the component detail of a selection of the
 * claim. It shows the request of the selection with its conditions, the
 * pods of its pod selector, its metrics, the lines of its controller about
 * it, and its events. A selection whose request does not exist yet shows
 * none of them, and says why. requestsError is the error of the read of the
 * PluginRequests: a selection without its request then says that the
 * request could not be read. A request whose pod selector has an operator
 * that is not valid shows no pods, and says so. The Overview ends with
 * children, and the Settings tab holds settings. It needs a
 * QueryClientProvider above it.
 */
export function SectionComponentDetail({
  component,
  registrations,
  namespace,
  requestsError,
  settings,
  now,
  children,
}: {
  component: SectionComponent;
  registrations: readonly Resource<"PluginRegistration">[];
  namespace: string;
  requestsError?: string;
  settings?: ReactNode;
  now: string;
  children?: ReactNode;
}) {
  const registration = component.selection.registration;
  const subtitle =
    registrations.find((r) => r.metadata?.name === registration)?.spec.label ??
    registration;
  const request = component.request;
  const ready = component.condition as ConditionRow | undefined;

  if (!request) {
    return (
      <ComponentDetail
        title={component.label}
        subtitle={subtitle}
        namespace={namespace}
        eventRefs={[]}
        conditions={[]}
        ready={ready}
        settings={settings}
        now={now}
      >
        {requestsError === undefined ? (
          <p className="text-sm text-muted">
            The claim controller has not made the request of this section yet.
          </p>
        ) : (
          <p className="text-sm text-warn">
            Could not read PluginRequest: {requestsError}
          </p>
        )}
        {children}
      </ComponentDetail>
    );
  }

  const name = request.metadata?.name ?? "";
  const pods = podSelectorOf(request);
  return (
    <ComponentDetail
      title={component.label}
      subtitle={subtitle}
      namespace={namespace}
      podSelector={pods.selector}
      metrics={{ request: name }}
      controllerLogs={{
        registration: request.spec.registration,
        request: { namespace, name },
      }}
      eventRefs={[
        {
          kind: "PluginRequest",
          name,
          namespace,
          uid: request.metadata?.uid,
        },
      ]}
      conditions={(request.status?.conditions ?? []) as ConditionRow[]}
      ready={ready}
      settings={settings}
      now={now}
    >
      {pods.error !== undefined && (
        <p className="text-sm text-warn">
          The pod selector of the request is not valid: {pods.error}.
        </p>
      )}
      {children}
    </ComponentDetail>
  );
}

// podSelectorOf reads the pod selector of request. The CRD does not limit
// the operator of a match expression, so a plugin can write one that
// labelSelectorString refuses.
function podSelectorOf(request: Resource<"PluginRequest">): {
  selector?: string;
  error?: string;
} {
  try {
    return { selector: labelSelectorString(request.status?.podSelector) };
  } catch (error) {
    return { error: error instanceof Error ? error.message : String(error) };
  }
}
