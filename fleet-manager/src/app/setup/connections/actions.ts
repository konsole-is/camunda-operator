"use server";

import type { ConnectionPageData } from "@/components/setup/connections/sources";
import { contractsOf } from "@/components/setup/connections/tabs";
import {
  CONTRACT_KINDS,
  type ContractKind,
  contractKey,
  contractUsage,
  contractWriter,
  unreadUserKinds,
  writerEditable,
  writerLabel,
} from "@/lib/usage";
import {
  adminChangeSchema,
  runChange,
  runPreview,
} from "@/lib/write/run-change";
import type {
  ApiError,
  Change,
  ObjectRef,
  Preview,
  WriteResult,
} from "@/lib/write/types";
import type { z } from "zod";
import { loadConnections } from "./load";

/**
 * previewContractChange is the dry run of an update or a delete of
 * connection contracts, under the rules of applyContractChange.
 */
export async function previewContractChange(change: Change): Promise<Preview> {
  const checked = await check(change);
  if ("errors" in checked) return { objects: [], errors: checked.errors };
  return runPreview(checked.change, checked.schema);
}

/**
 * applyContractChange writes an update or a delete of connection contracts
 * with the writer of the settings. It reads every list from the cache of the
 * server first. It refuses with Invalid a change that writes another kind, a
 * contract in a namespace that the cluster does not have, an object without
 * a precondition, or a create. It refuses with Forbidden a change of a
 * contract that a registration, a management plane, or a controller writes,
 * and a delete of a contract that an object uses. While a list that these
 * checks read cannot be read, it refuses the change that needs it.
 */
export async function applyContractChange(
  change: Change,
): Promise<WriteResult> {
  const checked = await check(change);
  if ("errors" in checked) return { ok: false, errors: checked.errors };
  return runChange(checked.change, checked.schema);
}

async function check(
  input: Change,
): Promise<
  { change: Change; schema: z.ZodType<Change> } | { errors: ApiError[] }
> {
  const sources = await loadConnections();
  const namespaces = sources.namespaces.map((n) => n.metadata?.name ?? "");
  const schema = adminChangeSchema(CONTRACT_KINDS, namespaces).refine(
    (c) => c.create === undefined,
    { message: "The change must not create a contract." },
  );
  const parsed = schema.safeParse(input);
  if (!parsed.success) {
    const refused = await runChange(input, schema);
    return { errors: refused.ok ? [] : refused.errors };
  }
  const change = parsed.data;
  const targets: ObjectRef[] = [
    ...change.apply.map((o) => ({
      apiVersion: o.apiVersion,
      kind: o.kind,
      ...(o.metadata.namespace !== undefined && {
        namespace: o.metadata.namespace,
      }),
      name: o.metadata.name,
    })),
    ...(change.delete ?? []),
  ];
  const deleted = new Set(change.delete ?? []);
  const errors = targets.flatMap((ref) =>
    refusalsOf(ref, deleted.has(ref), sources),
  );
  return errors.length > 0 ? { errors } : { change, schema };
}

function refusalsOf(
  ref: ObjectRef,
  deleting: boolean,
  sources: ConnectionPageData,
): ApiError[] {
  const kind = ref.kind as ContractKind;
  const unread = (sources.readErrors ?? []).map((e) => e.kind);
  const named = `${kind} ${ref.namespace ? `${ref.namespace}/` : ""}${ref.name}`;
  const refuse = (message: string): ApiError[] => [
    { code: "Forbidden", message, ref },
  ];
  if (unread.includes(kind))
    return refuse(
      `The fleet manager cannot read ${kind}, so ${named} is not changed.`,
    );
  const live = contractsOf(sources, kind).find(
    (c) =>
      c.metadata.name === ref.name && c.metadata.namespace === ref.namespace,
  );
  // A contract that is gone fails on its precondition in the writer.
  if (!live) return [];
  const requests = unread.includes("PluginRequest")
    ? undefined
    : sources.requests;
  const writer = contractWriter(live, requests, sources.planes);
  if (!writerEditable(writer))
    return refuse(
      `${named} is managed by ${writerLabel(writer)}. Change it there.`,
    );
  if (!deleting) return [];
  const blind = unreadUserKinds(kind, unread);
  if (blind.length > 0)
    return refuse(
      `Delete is possible when the fleet manager can read ${blind.join(", ")}.`,
    );
  const users =
    contractUsage(sources).get(
      contractKey({
        kind,
        ...(ref.namespace !== undefined && { namespace: ref.namespace }),
        name: ref.name,
      }),
    ) ?? [];
  if (users.length > 0)
    return refuse(
      `Delete is possible when no resource uses ${named}. Used by ${users
        .map((u) => `${u.kind} ${u.namespace}/${u.name}`)
        .join(", ")}.`,
    );
  return [];
}
