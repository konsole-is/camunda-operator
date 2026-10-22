"use client";
// The client half of the data layer. It imports only types from the server
// modules of src/lib/k8s, and the isomorphic deltas module.
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { REVISION_ANNOTATION } from "@/lib/labels";
import type { NamedObject } from "./deltas";
import type { KindName, Resource } from "./kinds";
import type { ListOptions } from "./read";
import {
  failureKey,
  sharedWatch,
  watchKey,
  type WatchSpec,
} from "./shared-watch";

/** Ref names one object. namespace is unset for a cluster-scoped kind. */
export interface Ref {
  namespace?: string;
  name: string;
}

/**
 * useResources returns the objects of kind that opts select. It returns
 * initial, the snapshot of the server render, at once. It then follows
 * /api/watch: each snapshot frame replaces the list, also after a
 * reconnect, and each delta changes it. When the first list of the kind on
 * the server fails, error holds its message and items keep the last list,
 * which can be out of date, until the next snapshot. All hooks under one
 * QueryClientProvider share one connection to /api/watch, and hooks with
 * the same kind and filter share one entry. The hook leaves the connection
 * on unmount. A component that calls it needs a QueryClientProvider above
 * it.
 */
export function useResources<K extends KindName>(
  kind: K,
  opts: ListOptions,
  initial: Resource<K>[],
): { items: Resource<K>[]; error?: string } {
  return useWatchedList(
    kind,
    { namespace: opts.namespace, labelSelector: opts.labelSelector },
    initial,
  );
}

/**
 * useResource returns one object of kind, live, and error, as useResources
 * does for a list. item is undefined when the object does not exist or was
 * deleted.
 */
export function useResource<K extends KindName>(
  kind: K,
  ref: Ref,
  initial?: Resource<K>,
): { item?: Resource<K>; error?: string } {
  const { items, error } = useWatchedList(
    kind,
    { namespace: ref.namespace, name: ref.name },
    initial ? [initial] : [],
  );
  const item = items.find(
    (o) =>
      (o as NamedObject).metadata?.name === ref.name &&
      (o as NamedObject).metadata?.namespace === ref.namespace,
  );
  return error === undefined ? { item } : { item, error };
}

/** GitSyncRef names the object of a push. */
export interface GitSyncRef extends Ref {
  kind: KindName;
}

/** GitSync is "waiting" until the live object carries the pushed revision. */
export type GitSync = "waiting" | "applied";

/**
 * useGitSync follows the object of ref and returns "applied" once its
 * annotation REVISION_ANNOTATION equals revision. It returns "waiting" for
 * any other revision, while the object does not exist, and while the watch
 * fails. It needs a QueryClientProvider above it, as useResource does.
 */
export function useGitSync(ref: GitSyncRef, revision: string): GitSync {
  const { item, error } = useResource(ref.kind, ref);
  if (error !== undefined) return "waiting";
  const annotations = (
    item as { metadata?: { annotations?: Record<string, string> } } | undefined
  )?.metadata?.annotations;
  return annotations?.[REVISION_ANNOTATION] === revision
    ? "applied"
    : "waiting";
}

function useWatchedList<K extends KindName>(
  kind: K,
  filter: Omit<WatchSpec, "kind">,
  initial: Resource<K>[],
): { items: Resource<K>[]; error?: string } {
  const client = useQueryClient();
  const { namespace, labelSelector, name } = filter;
  const spec = { kind, namespace, labelSelector, name };
  // The lists and the failures change only through the shared watch, never
  // by a refetch.
  const listKey = watchKey(spec);
  const { data } = useQuery({
    queryKey: listKey,
    queryFn: () => client.getQueryData<Resource<K>[]>(listKey) ?? initial,
    initialData: initial,
    staleTime: Infinity,
  });
  const errorKey = failureKey(spec);
  const { data: failure } = useQuery({
    queryKey: errorKey,
    queryFn: () => client.getQueryData<string | null>(errorKey) ?? null,
    initialData: null,
    staleTime: Infinity,
  });

  useEffect(
    () =>
      sharedWatch(client).subscribe({ kind, namespace, labelSelector, name }),
    [client, kind, namespace, labelSelector, name],
  );

  return typeof failure === "string"
    ? { items: data, error: failure }
    : { items: data };
}
