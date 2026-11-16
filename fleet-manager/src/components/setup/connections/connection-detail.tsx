"use client";

import type { ReactNode } from "react";
import { Panel, Term, Terms } from "@/components/cluster/claim-tab";
import { StatusChip } from "@/components/fleet/status-chip";
import type { ReadError } from "@/components/planes/planes-table";
import { unreadKinds } from "@/components/setup/releases/rows";
import {
  type ContractRef,
  contractKey,
  contractUsage,
  contractWriter,
  unreadUserKinds,
  writerEditable,
  writerLabel,
} from "@/lib/usage";
import { requestsOf } from "./connection-list";
import { contractFacts, readyOf } from "./facts";
import { SecretRefView } from "./secret-ref-view";
import {
  type ConnectionPageData,
  type ConnectionSources,
  useConnectionSources,
} from "./sources";
import { CONNECTION_TABS, contractsOf } from "./tabs";
import { UsersTable } from "./users-table";

/**
 * ConnectionDetail is the page of one contract: its own Ready, its writer,
 * its settings, and the objects that use it. A contract that sources does
 * not hold shows as missing. Each of the read errors shows above.
 */
export function ConnectionDetail({
  target,
  sources,
}: {
  target: ContractRef;
  sources: ConnectionSources & { errors: ReadError[] };
}): ReactNode {
  const contract = contractsOf(sources, target.kind).find(
    (c) =>
      c.metadata.name === target.name &&
      (target.namespace === undefined ||
        c.metadata.namespace === target.namespace),
  );
  const kindLabel = CONNECTION_TABS.find((t) => t.kind === target.kind)!.label;
  const errors = sources.errors.map((e) => (
    <p key={e.kind} role="alert" className="text-sm text-bad">
      Could not read {e.kind}: {e.message}
    </p>
  ));
  if (!contract) {
    return (
      <div className="flex flex-col gap-6">
        {errors}
        <p role="alert" className="text-sm text-bad">
          The contract {target.name} does not exist.
        </p>
      </div>
    );
  }
  const ready = readyOf(contract);
  const writer = contractWriter(contract, requestsOf(sources), sources.planes);
  const users = contractUsage(sources).get(contractKey(target)) ?? [];
  const unread = unreadUserKinds(target.kind, unreadKinds(sources));
  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-2">
        <p className="text-sm text-muted">{kindLabel}</p>
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="font-heading text-3xl font-bold tracking-[-0.02em] [overflow-wrap:anywhere]">
            {target.name}
          </h1>
          <StatusChip condition={ready} />
        </div>
        {target.namespace !== undefined && (
          <p className="font-mono text-sm text-muted">{target.namespace}</p>
        )}
      </header>
      {errors}
      <div className="grid gap-4 lg:grid-cols-2">
        <Panel label="Status">
          {ready ? (
            <Terms>
              <Term label="Reason">{ready.reason || "None"}</Term>
              <Term label="Message">{ready.message || "None"}</Term>
            </Terms>
          ) : (
            <p className="text-sm text-muted">No status yet</p>
          )}
        </Panel>
        <Panel label="Writer">
          <div className="flex flex-col gap-1 text-sm">
            <p>{writerLabel(writer)}</p>
            {!writerEditable(writer) && (
              <p className="text-muted">
                Managed by {writerLabel(writer)}. Change it there.
              </p>
            )}
          </div>
        </Panel>
      </div>
      <Panel label="Settings">
        <Terms>
          {contractFacts(contract).map((fact) => (
            <Term key={fact.label} label={fact.label} note={fact.note}>
              {fact.secret ? (
                <SecretRefView secret={fact.secret} />
              ) : (
                fact.value
              )}
            </Term>
          ))}
        </Terms>
      </Panel>
      <Panel label="Used by">
        <UsersTable users={users} unread={unread} />
      </Panel>
    </div>
  );
}

/** ConnectionDetailLive is ConnectionDetail on the live sources, starting from initial. */
export function ConnectionDetailLive({
  initial,
  target,
}: {
  initial: ConnectionPageData;
  target: ContractRef;
}): ReactNode {
  return (
    <ConnectionDetail target={target} sources={useConnectionSources(initial)} />
  );
}
