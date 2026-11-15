"use client";

import type { ReactNode } from "react";
import { ConditionList } from "@/components/fleet/condition-list";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import { StatusChip } from "@/components/fleet/status-chip";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useResources } from "@/lib/k8s/hooks";
import type { EventRef } from "@/lib/observe/events";
import type { MetricsRef, PodUsage, Unavailable } from "@/lib/observe/types";
import { usePodUsage } from "@/lib/observe/use-observe";
import { ComponentStats } from "./component-stats";
import { DetailEvents } from "./detail-events";
import { type ControllerLogs, DetailLogs } from "./detail-logs";
import { DetailMetrics, DetailMetricsView } from "./detail-metrics";
import { DetailSection } from "./detail-section";
import { type PodLike, PodTable } from "./pod-table";

/** DetailTab names one tab of a component detail. */
export type DetailTab = "overview" | "metrics" | "logs" | "events" | "settings";

/** DETAIL_TABS are the tabs of a component detail, in order. */
export const DETAIL_TABS: readonly { value: DetailTab; label: string }[] = [
  { value: "overview", label: "Overview" },
  { value: "metrics", label: "Metrics" },
  { value: "logs", label: "Logs" },
  { value: "events", label: "Events" },
  { value: "settings", label: "Settings" },
];

type DetailProps = {
  title: string;
  subtitle?: string;
  conditions: ConditionRow[];
  ready: ConditionRow | undefined;
  settings?: ReactNode;
  children?: ReactNode;
  now: string;
};

/**
 * ComponentDetail shows one component: its state from ready, the condition
 * that decides it, then the pods in namespace that podSelector selects,
 * with their live usage, and the given conditions. The Overview tab ends
 * with children, the Metrics tab charts the metrics ref, and the Settings
 * tab holds settings. Without metrics, the Metrics tab shows the live usage
 * only. With controllerLogs, the Logs tab also follows the controller pods. The Events tab lists the events of the
 * pods, of the Services and PVCs that relatedSelector selects (podSelector
 * without it), and of eventRefs. It needs a QueryClientProvider above it.
 */
export function ComponentDetail({
  namespace,
  podSelector,
  metrics,
  relatedSelector,
  controllerLogs,
  eventRefs,
  ...props
}: DetailProps & {
  namespace: string;
  podSelector: string;
  relatedSelector?: string;
  metrics?: MetricsRef;
  controllerLogs?: ControllerLogs;
  eventRefs?: EventRef[];
}) {
  const pods = useResources(
    "Pod",
    { namespace, labelSelector: podSelector },
    [],
  );
  const usage = usePodUsage(namespace, podSelector);
  return (
    <ComponentDetailView
      {...props}
      pods={pods.items}
      usage={usage}
      metricsTab={
        <DetailMetrics
          namespace={namespace}
          source={metrics}
          pods={pods.items}
          usage={usage}
        />
      }
      logsTab={
        <DetailLogs
          namespace={namespace}
          pods={pods.items}
          podsError={pods.error}
          controller={controllerLogs}
        />
      }
      eventsTab={
        <DetailEvents
          namespace={namespace}
          selector={relatedSelector ?? podSelector}
          pods={pods.items}
          podsError={pods.error}
          refs={eventRefs ?? []}
          now={props.now}
        />
      }
    />
  );
}

/**
 * ComponentDetailView is ComponentDetail with its pods and usage given.
 * An undefined usage is not loaded yet. Without metricsTab, the Metrics
 * tab shows the live usage of the pods and no charts. Without logsTab or
 * eventsTab, that tab says there is nothing to show.
 */
export function ComponentDetailView({
  title,
  subtitle,
  conditions,
  ready,
  pods,
  usage,
  settings,
  metricsTab,
  logsTab,
  eventsTab,
  children,
  now,
}: DetailProps & {
  pods: PodLike[];
  usage: PodUsage[] | Unavailable | undefined;
  metricsTab?: ReactNode;
  logsTab?: ReactNode;
  eventsTab?: ReactNode;
}) {
  return (
    <section
      aria-label={title}
      className="flex flex-col gap-4 rounded-lg border border-border bg-panel p-4"
    >
      <header className="flex flex-col gap-0.5">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <h2
            tabIndex={-1}
            className="font-heading text-lg font-semibold text-text outline-none"
          >
            {title}
          </h2>
          <StatusChip condition={ready} readyReason />
        </div>
        {subtitle && <p className="text-sm text-muted">{subtitle}</p>}
      </header>
      <Tabs defaultValue="overview">
        <TabsList variant="line">
          {DETAIL_TABS.map((t) => (
            <TabsTrigger key={t.value} value={t.value}>
              {t.label}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="overview">
          <div className="flex flex-col gap-6 pt-2">
            <ComponentStats pods={pods} usage={usage} />
            <div className="flex flex-wrap items-start gap-6">
              <DetailSection title="Pods" className="flex-[999_1_32rem]">
                <PodTable pods={pods} usage={usage ?? []} />
              </DetailSection>
              <DetailSection title="Conditions" className="flex-[1_1_18rem]">
                <ConditionList
                  conditions={conditions}
                  label={`${title} conditions`}
                  now={now}
                />
              </DetailSection>
            </div>
            {children}
          </div>
        </TabsContent>
        <TabsContent value="metrics">
          {metricsTab ?? (
            <DetailMetricsView pods={pods} usage={usage} range="1h" />
          )}
        </TabsContent>
        <TabsContent value="logs">
          {logsTab ?? <p className="pt-2 text-sm text-muted">No logs.</p>}
        </TabsContent>
        <TabsContent value="events">
          {eventsTab ?? <p className="pt-2 text-sm text-muted">No events</p>}
        </TabsContent>
        <TabsContent value="settings">
          <div className="pt-2">
            {settings ?? <p className="text-sm text-muted">No settings.</p>}
          </div>
        </TabsContent>
      </Tabs>
    </section>
  );
}
