"use client";

import type { ReactNode } from "react";
import {
  type ConditionRow,
  ConditionsTable,
} from "@/components/fleet/conditions-table";
import { type PodLike, PodTable } from "@/components/fleet/pod-table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useResources } from "@/lib/k8s/hooks";
import type { MetricsRef, PodUsage, Unavailable } from "@/lib/observe/types";
import { usePodUsage } from "@/lib/observe/use-observe";
import { DetailMetrics, DetailMetricsView } from "./detail-metrics";

/** DetailTab names one tab of a component detail. */
export type DetailTab = "overview" | "metrics" | "settings";

/** DETAIL_TABS are the tabs of a component detail, in order. */
export const DETAIL_TABS: readonly { value: DetailTab; label: string }[] = [
  { value: "overview", label: "Overview" },
  { value: "metrics", label: "Metrics" },
  { value: "settings", label: "Settings" },
];

type DetailProps = {
  title: string;
  subtitle?: string;
  conditions: ConditionRow[];
  settings?: ReactNode;
  children?: ReactNode;
  now: string;
};

/**
 * ComponentDetail shows one component: the pods in namespace that
 * podSelector selects, with their live usage, and the given conditions.
 * The Overview tab ends with children, the Metrics tab charts the metrics
 * ref, and the Settings tab holds settings. Without metrics, the Metrics
 * tab shows the live usage only. It reads nothing else, so any page can
 * mount it for any component. It needs a QueryClientProvider above it.
 */
export function ComponentDetail({
  namespace,
  podSelector,
  metrics,
  ...props
}: DetailProps & {
  namespace: string;
  podSelector: string;
  metrics?: MetricsRef;
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
    />
  );
}

/**
 * ComponentDetailView is ComponentDetail with its pods and usage given.
 * An undefined usage is not loaded yet. Without metricsTab, the Metrics
 * tab shows the live usage of the pods and no charts.
 */
export function ComponentDetailView({
  title,
  subtitle,
  conditions,
  pods,
  usage,
  settings,
  metricsTab,
  children,
  now,
}: DetailProps & {
  pods: PodLike[];
  usage: PodUsage[] | Unavailable | undefined;
  metricsTab?: ReactNode;
}) {
  return (
    <section
      aria-label={title}
      className="flex flex-col gap-4 rounded-lg border border-border bg-panel p-4"
    >
      <header className="flex flex-col gap-0.5">
        <h2
          tabIndex={-1}
          className="font-heading text-lg font-semibold text-text outline-none"
        >
          {title}
        </h2>
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
          <div className="flex flex-col gap-4 pt-2">
            <ConditionsTable
              conditions={conditions}
              label={`${title} conditions`}
              now={now}
            />
            <PodTable pods={pods} usage={usage ?? []} now={now} />
            {children}
          </div>
        </TabsContent>
        <TabsContent value="metrics">
          {metricsTab ?? (
            <DetailMetricsView pods={pods} usage={usage} range="1h" />
          )}
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
