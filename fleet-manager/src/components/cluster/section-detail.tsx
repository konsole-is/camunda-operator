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
 * none of them, and says why. The Overview ends with children, and the
 * Settings tab holds settings. It needs a QueryClientProvider above it.
 */
export function SectionComponentDetail({
  component,
  registrations,
  namespace,
  settings,
  now,
  children,
}: {
  component: SectionComponent;
  registrations: readonly Resource<"PluginRegistration">[];
  namespace: string;
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
        <p className="text-sm text-muted">
          The claim controller has not made the request of this section yet.
        </p>
        {children}
      </ComponentDetail>
    );
  }

  const name = request.metadata?.name ?? "";
  return (
    <ComponentDetail
      title={component.label}
      subtitle={subtitle}
      namespace={namespace}
      podSelector={labelSelectorString(request.status?.podSelector)}
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
      {children}
    </ComponentDetail>
  );
}
