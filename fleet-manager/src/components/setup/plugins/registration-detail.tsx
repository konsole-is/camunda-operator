"use client";

import { type ReactNode, useState } from "react";
import { useClock } from "@/components/fleet/use-clock";
import type { ReadError } from "@/components/planes/planes-table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { AboutTab } from "./about-tab";
import { contractOf } from "./contract";
import { ContractTab } from "./contract-tab";
import {
  type RegistrationDetailData,
  useRegistrationDetail,
} from "./detail-sources";
import { unreadKinds } from "./groups";
import { GrantsTab } from "./grants-tab";
import { HealthTab } from "./health-tab";
import { parameterRows } from "./parameters";
import { ParametersTab } from "./parameters-tab";
import { usedByRows } from "./used-by";
import { UsedByTab } from "./used-by-tab";
import {
  REGISTRATION_TABS,
  type RegistrationTab,
  registrationTabOf,
} from "./registration-tabs";

/**
 * RegistrationDetail is the page of the registration data.name, opened on
 * tab. A tab change sets the tab query parameter without a navigation.
 * errors holds the error of each failed watch, and stale the kind of each
 * list whose watch has no snapshot on its current connection. While
 * PluginRegistration is read and has no registration of the name, the page
 * says that it no longer exists. While it is not read, the page shows the
 * registration of its list, else data.registration. now is the time that
 * the ages count to.
 */
export function RegistrationDetail({
  data,
  errors,
  stale,
  tab: initialTab,
  now,
}: {
  data: RegistrationDetailData;
  errors: ReadError[];
  stale: readonly string[];
  tab: RegistrationTab;
  now: string;
}): ReactNode {
  const [tab, setTab] = useState(initialTab);
  const selectTab = (value: string) => {
    const next = registrationTabOf(value);
    setTab(next);
    window.history.replaceState(null, "", `?tab=${next}`);
  };
  const unread = unreadKinds({ errors, stale });
  const { sources } = data;
  const registration =
    sources.registrations.find((r) => r.metadata.name === data.name) ??
    (unread.includes("PluginRegistration") ? data.registration : undefined);
  return (
    <div className="flex flex-col gap-6">
      <header className="flex min-w-0 flex-col gap-1">
        <h1 className="font-heading text-3xl font-bold tracking-[-0.02em] break-words">
          {registration?.spec.label ?? data.name}
        </h1>
        <p className="font-mono text-sm break-all text-muted">{data.name}</p>
      </header>
      {errors.map((e) => (
        <p key={e.kind} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
      {unread.length > 0 && (
        <p className="text-sm text-muted">
          {`Not read: ${unread.join(", ")}. What needs them shows as not known.`}
        </p>
      )}
      {!registration ? (
        <p className="text-sm">This registration no longer exists.</p>
      ) : (
        <Tabs value={tab} onValueChange={selectTab}>
          <TabsList className="h-auto max-w-full flex-wrap">
            {REGISTRATION_TABS.map((t) => (
              <TabsTrigger key={t.value} value={t.value}>
                {t.label}
              </TabsTrigger>
            ))}
          </TabsList>
          <TabsContent value="about">
            <AboutTab
              registration={registration}
              registrations={sources.registrations}
              unread={unread}
            />
          </TabsContent>
          <TabsContent value="contract">
            <ContractTab
              contract={contractOf(registration, sources.sections, unread)}
            />
          </TabsContent>
          <TabsContent value="parameters">
            <ParametersTab
              rows={parameterRows(registration, sources.environments, unread)}
              unread={unread}
            />
          </TabsContent>
          <TabsContent value="grants">
            <GrantsTab
              registration={registration}
              grants={data.grants}
              unread={unread}
              now={now}
            />
          </TabsContent>
          <TabsContent value="health">
            <HealthTab
              registration={registration}
              events={data.events}
              unread={unread}
              now={now}
            />
          </TabsContent>
          <TabsContent value="usedBy">
            <UsedByTab
              rows={usedByRows(
                data.name,
                { ...sources, errors, stale },
                data.grants,
              )}
              unread={unread}
            />
          </TabsContent>
        </Tabs>
      )}
    </div>
  );
}

/**
 * RegistrationDetailLive is RegistrationDetail on the live sources,
 * starting from initial. now is the time of the render on the server.
 */
export function RegistrationDetailLive({
  initial,
  tab,
  now,
}: {
  initial: RegistrationDetailData;
  tab: RegistrationTab;
  now: string;
}): ReactNode {
  const { errors, stale, ...data } = useRegistrationDetail(initial);
  const clock = useClock(now, true);
  return (
    <RegistrationDetail
      data={data}
      errors={errors}
      stale={stale}
      tab={tab}
      now={clock}
    />
  );
}
