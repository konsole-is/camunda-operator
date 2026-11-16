import type { ReactNode } from "react";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import type { SectionComponent } from "@/lib/claims/sections";
import type { Resource } from "@/lib/k8s/kinds";
import { labelSelectorString } from "@/lib/labels";
import { conditionsOf } from "@/lib/policy";
import { ComponentDetail } from "./component-detail";
import { NotRead } from "./not-read";

/**
 * SectionComponentDetail is the component detail of a selection of the
 * claim. It shows the request of the selection with its conditions, the
 * pods of its pod selector, its metrics, the lines of its controller about
 * it, and its events. A selection whose request does not exist yet shows
 * none of them, and says why. requestsError is the error of the read of the
 * PluginRequests, and requestsStale says that their list is out of date:
 * with either, the detail shows the state and the conditions of the
 * request as not known, and says why in place of children. A request
 * whose pod selector has an operator that is not valid shows no pods, and
 * says so. Conditions of the wrong type show as an error, not as
 * conditions. The Overview ends with children, and the Settings tab holds
 * settings. It needs a QueryClientProvider above it.
 */
export function SectionComponentDetail({
  component,
  registrations,
  namespace,
  requestsError,
  requestsStale = false,
  settings,
  now,
  children,
}: {
  component: SectionComponent;
  registrations: readonly Resource<"PluginRegistration">[];
  namespace: string;
  requestsError?: string;
  requestsStale?: boolean;
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
  const unread = requestsError !== undefined || requestsStale;
  const why =
    requestsError !== undefined ? (
      <p className="text-sm text-warn">
        Could not read PluginRequest: {requestsError}
      </p>
    ) : (
      <NotRead kinds={["PluginRequest"]} />
    );

  if (!request) {
    return (
      <ComponentDetail
        title={component.label}
        subtitle={subtitle}
        namespace={namespace}
        eventRefs={[]}
        conditions={[]}
        ready={ready}
        {...(unread && { unread: "PluginRequest" })}
        settings={settings}
        now={now}
      >
        {unread ? (
          why
        ) : (
          <>
            <p className="text-sm text-muted">
              The claim controller has not made the request of this section yet.
            </p>
            {children}
          </>
        )}
      </ComponentDetail>
    );
  }

  const name = request.metadata?.name ?? "";
  const pods = podSelectorOf(request);
  const read = conditionsOf(request.status?.conditions);
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
      conditions={"conditions" in read ? read.conditions : []}
      ready={ready}
      {...(unread && { unread: "PluginRequest" })}
      settings={settings}
      now={now}
    >
      {"error" in read && <p className="text-sm text-bad">{read.error}</p>}
      {pods.error !== undefined && (
        <p className="text-sm text-warn">
          The pod selector of the request is not valid: {pods.error}.
        </p>
      )}
      {unread ? why : children}
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
