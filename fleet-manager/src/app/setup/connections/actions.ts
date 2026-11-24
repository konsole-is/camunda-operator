"use server";

import { planeContractHolder } from "@/components/setup/connections/draft";
import type { ConnectionPageData } from "@/components/setup/connections/sources";
import { contractsOf } from "@/components/setup/connections/tabs";
import { unreadChangeText, unreadDeleteText } from "@/lib/k8s/unread";
import {
  CONTRACT_KINDS,
  type ContractKind,
  contractKey,
  contractUsage,
  contractWriter,
  unreadUserKinds,
  waitingDeleteText,
  waitingRequests,
  writerEditable,
  writerLabel,
} from "@/lib/usage";
import { existsError, staleError } from "@/lib/write/errors";
import { refKey, refOf } from "@/lib/write/refs";
import {
  adminChangeSchema,
  issuesOf,
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
 * previewContractChange is the dry run of a create, an update, or a delete
 * of one connection contract, under the rules of applyContractChange.
 */
export async function previewContractChange(change: Change): Promise<Preview> {
  const checked = await check(change);
  if ("errors" in checked) return { objects: [], errors: checked.errors };
  return runPreview(checked.change, checked.schema);
}

/**
 * applyContractChange writes a create, an update, or a delete of one
 * connection contract with the writer of the settings. It reads every list
 * from the cache of the server first. It refuses with Invalid a change that
 * writes more than one contract or another kind, a contract in a namespace
 * that the cluster does not have, an update or a delete without a
 * precondition, and a create with metadata other than a name and a
 * namespace. It refuses a create with Stale when the list that the server
 * read holds the contract, and with Forbidden while that list or, for a
 * Management Identity contract, a list of management planes or plane
 * claims cannot be read, or when a management plane or a plane claim
 * writes a Management Identity contract of that name. It refuses with
 * Stale a change of a contract that is not in the list that the server
 * read, or whose precondition is not the resourceVersion of the copy in
 * that list. Of the other contracts, it refuses with Forbidden a change
 * of a contract that a registration, a management plane, or a controller
 * writes, a delete of a contract that an object uses, and a delete while
 * waitingRequests gives a request for the contract. While a list that
 * these checks read cannot be read, it refuses each change of a contract of
 * that kind or that it can use.
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
  const schema = adminChangeSchema(CONTRACT_KINDS, namespaces)
    // A create with labels or annotations could write the labels of a
    // registration or a management plane.
    .refine((c) => c.create === undefined || isHandCreate(c), {
      message:
        "A create must write one contract with only its name and namespace.",
    })
    // The checks below read the users before the change, so a change that
    // also points an object at a contract that it deletes would pass them.
    .refine((c) => c.apply.length + (c.delete?.length ?? 0) === 1, {
      message: "The change must write exactly one contract.",
    });
  const parsed = schema.safeParse(input);
  if (!parsed.success) return { errors: issuesOf(parsed.error) };
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
    change.create?.includes(refKey(ref))
      ? createRefusalsOf(ref, sources)
      : refusalsOf(
          ref,
          change.preconditions?.[refKey(ref)],
          deleted.has(ref),
          sources,
        ),
  );
  return errors.length > 0 ? { errors } : { change, schema };
}

function isHandCreate(change: Change): boolean {
  const [object, ...others] = change.apply;
  return (
    object !== undefined &&
    others.length === 0 &&
    change.create?.length === 1 &&
    change.create[0] === refKey(refOf(object)) &&
    Object.keys(object.metadata).every(
      (key) => key === "name" || key === "namespace",
    )
  );
}

const PLANE_KINDS = [
  "CamundaManagementCluster",
  "CamundaManagementClusterClaim",
] as const;

// A new contract has no writer and no users, so only its name can be
// refused.
function createRefusalsOf(
  ref: ObjectRef,
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
      `The fleet manager cannot read ${kind}, so ${named} is not created.`,
    );
  if (
    contractsOf(sources, kind).some(
      (c) =>
        c.metadata.name === ref.name && c.metadata.namespace === ref.namespace,
    )
  )
    return [existsError(ref)];
  if (kind !== "ManagementAuthConfig") return [];
  const blind = PLANE_KINDS.filter((k) => unread.includes(k));
  if (blind.length > 0)
    return refuse(
      `The fleet manager cannot read ${blind.join(", ")}, so ${named} is not created.`,
    );
  // The operator refuses a management plane whose contract name another
  // writer holds.
  const holder = planeContractHolder(
    ref.name,
    sources.planes,
    sources.planeClaims,
  );
  return holder
    ? refuse(
        `The management plane ${holder} writes ${named}. Pick another name.`,
      )
    : [];
}

function refusalsOf(
  ref: ObjectRef,
  precondition: string | undefined,
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
  // The check of who writes the contract reads this copy, and a cache that
  // lags must not pass it for another version than the one that the writer
  // gets.
  if (!live || live.metadata.resourceVersion !== precondition)
    return [staleError(ref)];
  const requests = unread.includes("PluginRequest")
    ? undefined
    : sources.requests;
  const writer = contractWriter(live, requests, sources.planes);
  if (!writerEditable(writer))
    return refuse(
      `${named} is managed by ${writerLabel(writer)}. Change it there.`,
    );
  const blind = unreadUserKinds(kind, unread);
  if (blind.length > 0)
    return refuse(deleting ? unreadDeleteText(blind) : unreadChangeText(blind));
  const contract = {
    kind,
    ...(ref.namespace !== undefined && { namespace: ref.namespace }),
    name: ref.name,
  };
  if (!deleting) return [];
  const users = contractUsage(sources).get(contractKey(contract)) ?? [];
  if (users.length > 0)
    return refuse(
      `Delete is possible when no resource uses ${named}. Used by ${users
        .map((u) => `${u.kind} ${u.namespace}/${u.name}`)
        .join(", ")}.`,
    );
  const waiting = waitingRequests(contract, sources, unread);
  if (waiting.length > 0) return refuse(waitingDeleteText(waiting));
  return [];
}
