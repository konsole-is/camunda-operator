"use client";

import { type ReactNode, useState } from "react";
import { ConditionList } from "@/components/fleet/condition-list";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import { StatusChip } from "@/components/fleet/status-chip";
import { ConditionBadge } from "@/components/setup/plugins/condition-badge";
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
import { NotRead } from "./not-read";
import { NO_PODS, type PodLike, PodTable } from "./pod-table";

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
  unread?: string;
  settings?: ReactNode;
  children?: ReactNode;
  now: string;
};

type TabProps = {
  tab?: DetailTab;
  onTab?: (tab: DetailTab) => void;
};

type ObserveProps = {
  namespace: string;
  metrics?: MetricsRef;
  controllerLogs?: ControllerLogs;
  eventRefs?: EventRef[];
};

/**
 * ComponentDetail shows one component: its state from ready, the condition
 * that decides it, then the pods in namespace that podSelector selects,
 * with their live usage, and the given conditions. The Overview tab ends
 * with children, the Metrics tab charts the metrics ref, and the Settings
 * tab holds settings. Without metrics, the Metrics tab shows the live usage
 * only. With controllerLogs, the Logs tab also follows the controller pods.
 * The Events tab lists the events of the pods, of the Services and PVCs
 * that relatedSelector selects (podSelector without it), and of eventRefs.
 * A missing or empty podSelector selects no pods: the detail watches no
 * pods and no usage, says that the component runs no pods, and lists only
 * the events of eventRefs. unread names the kind of the object of the
 * component while the page could not read it or its list is out of date:
 * the state then shows Not known, and the Conditions section names the
 * kind in place of conditions. It needs a QueryClientProvider above it.
 */
export function ComponentDetail({
  podSelector,
  relatedSelector,
  ...props
}: DetailProps &
  ObserveProps & { podSelector?: string; relatedSelector?: string }) {
  // The tab lives here, so that it stays open when the pod selector comes
  // or goes and the view below mounts again.
  const [tab, setTab] = useState<DetailTab>("overview");
  return podSelector ? (
    <PodDetail
      {...props}
      tab={tab}
      onTab={setTab}
      podSelector={podSelector}
      relatedSelector={relatedSelector}
    />
  ) : (
    <PodlessDetail {...props} tab={tab} onTab={setTab} />
  );
}

function PodDetail({
  namespace,
  podSelector,
  metrics,
  relatedSelector,
  controllerLogs,
  eventRefs,
  ...props
}: DetailProps &
  ObserveProps &
  TabProps & { podSelector: string; relatedSelector?: string }) {
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

function PodlessDetail({
  namespace,
  metrics,
  controllerLogs,
  eventRefs,
  ...props
}: DetailProps & ObserveProps & TabProps) {
  return (
    <ComponentDetailView
      {...props}
      pods={[]}
      usage={[]}
      podless
      metricsTab={
        <DetailMetrics
          namespace={namespace}
          source={metrics}
          pods={[]}
          usage={[]}
          podless
        />
      }
      logsTab={
        <DetailLogs
          namespace={namespace}
          pods={[]}
          controller={controllerLogs}
        />
      }
      eventsTab={
        <DetailEvents
          namespace={namespace}
          pods={[]}
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
 * eventsTab, that tab says there is nothing to show. A podless component
 * shows no pod numbers and says that it runs no pods. With tab, the caller
 * holds the open tab, and onTab gets each tab that the user opens.
 */
export function ComponentDetailView({
  title,
  subtitle,
  conditions,
  ready,
  unread,
  pods,
  usage,
  settings,
  metricsTab,
  logsTab,
  eventsTab,
  children,
  now,
  podless = false,
  tab,
  onTab,
}: DetailProps &
  TabProps & {
    pods: PodLike[];
    podless?: boolean;
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
          {unread ? (
            <ConditionBadge type="Ready" unknown />
          ) : (
            <StatusChip condition={ready} readyReason />
          )}
        </div>
        {subtitle && <p className="text-sm text-muted">{subtitle}</p>}
      </header>
      <Tabs
        defaultValue="overview"
        value={tab}
        onValueChange={(value) => onTab?.(value as DetailTab)}
      >
        <TabsList variant="line">
          {DETAIL_TABS.map((t) => (
            <TabsTrigger key={t.value} value={t.value}>
              {t.label}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="overview">
          <div className="flex flex-col gap-6 pt-2">
            {!podless && <ComponentStats pods={pods} usage={usage} />}
            <div className="flex flex-wrap items-start gap-6">
              <DetailSection title="Pods" className="flex-[999_1_32rem]">
                {podless ? (
                  <p className="text-sm text-muted">{NO_PODS}</p>
                ) : (
                  <PodTable pods={pods} usage={usage ?? []} />
                )}
              </DetailSection>
              <DetailSection title="Conditions" className="flex-[1_1_18rem]">
                {unread ? (
                  <NotRead kinds={[unread]} />
                ) : (
                  <ConditionList
                    conditions={conditions}
                    label={`${title} conditions`}
                    now={now}
                  />
                )}
              </DetailSection>
            </div>
            {children}
          </div>
        </TabsContent>
        <TabsContent value="metrics">
          {metricsTab ?? (
            <DetailMetricsView
              pods={pods}
              usage={usage}
              range="1h"
              podless={podless}
            />
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
