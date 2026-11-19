"use client";

import { useSearchParams } from "next/navigation";
import { type ReactNode, useState } from "react";
import { DataTable } from "@/components/fleet/data-table";
import { type ConditionLike, describeReady } from "@/components/fleet/status";
import { StatusChip } from "@/components/fleet/status-chip";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  type ReadError,
  type SourceReads,
  unreadKinds,
} from "@/lib/k8s/unread";
import { byCodeUnit, readyOf } from "@/lib/policy";
import {
  type ContractKind,
  type ContractRef,
  type ContractWriter,
  contractKey,
  contractUsage,
  contractWriter,
  unreadUserKinds,
  type WaitingRequest,
  waitingRequests,
  waitingUsersText,
  writerLabel,
} from "@/lib/usage";
import { KEY_FACT_LABELS, type KeyFacts, keyFacts } from "./facts";
import {
  type ConnectionPageData,
  type ConnectionSources,
  useConnectionSources,
} from "./sources";
import {
  CONNECTION_TABS,
  type ConnectionTab,
  connectionTabOf,
  contractHref,
  contractsOf,
} from "./tabs";

/**
 * ContractRow is one contract of the Connections list. users counts the
 * distinct objects that name it. unread names the kinds that can use the
 * contract and that the page could not read, and waiting the requests of
 * waitingRequests, so users can be too low.
 */
export type ContractRow = {
  ref: ContractRef;
  facts: KeyFacts;
  writer: ContractWriter;
  users: number;
  unread: string[];
  waiting: WaitingRequest[];
  ready?: ConditionLike;
};

/**
 * contractRows returns a row for each contract of kind in sources, sorted
 * by namespace and name. While errors holds an error of PluginRequest, the
 * request list is not known, so a contract with the plugin labels stays
 * with its registration. Only errors count as unread, not stale lists.
 * usage must be contractUsage of sources.
 */
export function contractRows(
  sources: ConnectionSources & { errors?: readonly ReadError[] },
  kind: ContractKind,
  usage = contractUsage(sources),
): ContractRow[] {
  const reads = { errors: sources.errors };
  const requests = requestsOf({ ...sources, ...reads, stale: [] });
  const failed = unreadKinds(reads);
  const unread = unreadUserKinds(kind, failed);
  return contractsOf(sources, kind)
    .map((contract) => {
      const ref: ContractRef = {
        kind,
        ...(kind !== "ManagementAuthConfig" && {
          namespace: contract.metadata.namespace ?? "",
        }),
        name: contract.metadata.name ?? "",
      };
      const ready = readyOf(contract.status?.conditions);
      return {
        ref,
        facts: keyFacts(contract),
        writer: contractWriter(contract, requests, sources.planes),
        users: usage.get(contractKey(ref))?.length ?? 0,
        unread,
        waiting: waitingRequests(ref, sources, failed),
        ...(ready && { ready }),
      };
    })
    .sort(
      (a, b) =>
        byCodeUnit(a.ref.namespace ?? "", b.ref.namespace ?? "") ||
        byCodeUnit(a.ref.name, b.ref.name),
    );
}

/** requestsOf returns the requests of sources, or undefined while the PluginRequest list failed or is stale. */
export function requestsOf(
  sources: ConnectionSources & SourceReads,
): ConnectionSources["requests"] | undefined {
  return unreadKinds(sources).includes("PluginRequest")
    ? undefined
    : sources.requests;
}

const WRITER_FILTER = {
  key: "writer",
  label: "Writer",
  options: [
    { value: "hand", label: "Created by hand" },
    { value: "kept", label: "Kept from a registration" },
    { value: "registration", label: "Registration" },
    { value: "plane", label: "Management plane" },
    { value: "owner", label: "Another resource" },
  ],
} satisfies { options: { value: ContractWriter["by"]; label: string }[] } & {
  key: string;
  label: string;
};

/**
 * ConnectionList is the list page of the connection contracts, with one
 * tab for each contract kind. Each of errors shows above the tabs, and the
 * rows stay. onTab gets the tab that the user opens.
 */
export function ConnectionList({
  tab,
  rows,
  errors,
  onTab,
}: {
  tab: ConnectionTab;
  rows: Record<ConnectionTab, ContractRow[]>;
  errors: ReadError[];
  onTab: (tab: ConnectionTab) => void;
}): ReactNode {
  const unread = unreadKinds({ errors });
  return (
    <div className="flex flex-col gap-6">
      <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
        Connections
      </h1>
      {errors.map((e) => (
        <p key={e.kind} role="alert" className="text-sm text-bad">
          Could not read {e.kind}: {e.message}
        </p>
      ))}
      <Tabs
        value={tab}
        onValueChange={(value) => onTab(connectionTabOf(value))}
      >
        <TabsList className="max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto">
          {CONNECTION_TABS.map((t) => (
            <TabsTrigger
              key={t.value}
              value={t.value}
              className="max-w-full min-w-0 flex-none px-2.5"
            >
              {t.label}
            </TabsTrigger>
          ))}
        </TabsList>
        {CONNECTION_TABS.map((t) => (
          <TabsContent key={t.value} value={t.value}>
            <ContractTable
              label={t.label}
              kind={t.kind}
              rows={rows[t.value]}
              read={!unread.includes(t.kind)}
            />
          </TabsContent>
        ))}
      </Tabs>
    </div>
  );
}

function ContractTable({
  label,
  kind,
  rows,
  read,
}: {
  label: string;
  kind: ContractKind;
  rows: ContractRow[];
  read: boolean;
}) {
  const namespaced = kind !== "ManagementAuthConfig";
  const factLabels = KEY_FACT_LABELS[kind];
  const empty =
    kind === "ManagementAuthConfig"
      ? "No Management Identity contract yet."
      : `No ${label.toLowerCase()} contract yet.`;
  return (
    <DataTable
      label={`${label} contracts`}
      filters={[WRITER_FILTER]}
      emptyText={rows.length === 0 ? (read ? empty : "") : undefined}
      columns={[
        { key: "name", label: "Name", sortable: true },
        ...(namespaced
          ? [{ key: "namespace", label: "Namespace", sortable: true }]
          : []),
        ...factLabels.map((factLabel, i) => ({
          key: `fact${i}`,
          label: factLabel,
          sortable: true,
        })),
        { key: "writer", label: "Writer", sortable: true },
        { key: "users", label: "Users", sortable: true },
        { key: "status", label: "Status", sortable: true },
      ]}
      rows={rows.map((row) => {
        const writer = writerLabel(row.writer);
        const facts = Object.fromEntries(
          row.facts.map((fact, i) => [`fact${i}`, fact.value]),
        );
        return {
          id: contractKey(row.ref),
          label: row.ref.name,
          href: contractHref(row.ref),
          search: [
            row.ref.namespace,
            row.ref.name,
            writer,
            ...row.facts.map((f) => f.value),
          ]
            .filter(Boolean)
            .join(" "),
          filters: { writer: row.writer.by },
          sort: {
            name: row.ref.name,
            namespace: row.ref.namespace ?? "",
            ...facts,
            writer,
            users: row.users,
            status: describeReady(row.ready).label,
          },
          cells: {
            name: row.ref.name,
            namespace: row.ref.namespace,
            ...Object.fromEntries(
              row.facts.map((fact, i) => [
                `fact${i}`,
                <span key={i} className="[overflow-wrap:anywhere]">
                  {fact.value}
                </span>,
              ]),
            ),
            writer,
            users:
              row.unread.length > 0 || row.waiting.length > 0 ? (
                <div className="flex flex-col items-start gap-0.5">
                  <span>{row.users}</span>
                  {row.unread.length > 0 && (
                    <span className="text-xs text-bad [overflow-wrap:anywhere]">
                      Could not read {row.unread.join(", ")}
                    </span>
                  )}
                  {row.waiting.length > 0 && (
                    <span className="text-xs text-bad [overflow-wrap:anywhere]">
                      {waitingUsersText(row.waiting)}
                    </span>
                  )}
                </div>
              ) : (
                row.users
              ),
            status: <StatusChip condition={row.ready} />,
          },
        };
      })}
    />
  );
}

/** ConnectionListLive is ConnectionList on the live sources, starting from initial. The ?tab= value picks the first tab. */
export function ConnectionListLive({
  initial,
}: {
  initial: ConnectionPageData;
}): ReactNode {
  const sources = useConnectionSources(initial);
  const params = useSearchParams();
  const urlTab = connectionTabOf(params.get("tab") ?? undefined);
  const [tab, setTab] = useState<ConnectionTab>(urlTab);
  const [seenUrlTab, setSeenUrlTab] = useState(urlTab);
  if (urlTab !== seenUrlTab) {
    setSeenUrlTab(urlTab);
    setTab(urlTab);
  }
  const usage = contractUsage(sources);
  const rows = Object.fromEntries(
    CONNECTION_TABS.map((t) => [t.value, contractRows(sources, t.kind, usage)]),
  ) as Record<ConnectionTab, ContractRow[]>;
  return (
    <ConnectionList
      tab={tab}
      rows={rows}
      errors={sources.errors}
      onTab={(next) => {
        setTab(next);
        window.history.replaceState(null, "", `?tab=${next}`);
      }}
    />
  );
}
