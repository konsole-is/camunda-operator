"use client";
// The client half of the data layer. It imports only types from the server
// modules of src/lib/k8s, and the isomorphic deltas module.
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
  applyDelta,
  type NamedObject,
  type WatchDelta,
  type WatchFailure,
  type WatchSnapshot,
} from "./deltas";
import type { KindName, Resource } from "./kinds";

export interface ListOptions {
  namespace?: string;
  labelSelector?: string;
}

/** Ref names one object. namespace is unset for a cluster-scoped kind. */
export interface Ref {
  namespace?: string;
  name: string;
}

interface Filter {
  namespace?: string;
  labelSelector?: string;
  name?: string;
}

/**
 * useResources returns the objects of kind that opts select. It returns
 * initial, the snapshot of the server render, at once. It then follows
 * /api/watch: each snapshot frame replaces the list, also after a
 * reconnect, and each delta changes it. When the first list of the kind on
 * the server fails, error holds its message and items keep the last list,
 * which can be out of date, until the next snapshot. The stream closes on
 * unmount. A component that calls it needs a QueryClientProvider above it.
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

function queryKey(kind: KindName, filter: Filter) {
  return [
    "k8s",
    kind,
    filter.namespace ?? "",
    filter.labelSelector ?? "",
    filter.name ?? "",
  ] as const;
}

function watchURL(kind: KindName, filter: Filter): string {
  const params = new URLSearchParams({ kind });
  if (filter.namespace) params.set("namespace", filter.namespace);
  if (filter.labelSelector) params.set("labelSelector", filter.labelSelector);
  if (filter.name) params.set("name", filter.name);
  return `/api/watch?${params}`;
}

function useWatchedList<K extends KindName>(
  kind: K,
  filter: Filter,
  initial: Resource<K>[],
): { items: Resource<K>[]; error?: string } {
  const client = useQueryClient();
  const [error, setError] = useState<string>();
  const { namespace, labelSelector, name } = filter;
  const key = queryKey(kind, filter);
  const { data } = useQuery({
    queryKey: key,
    // The list changes only through the stream below, never by a refetch.
    queryFn: () => client.getQueryData<Resource<K>[]>(key) ?? initial,
    initialData: initial,
    staleTime: Infinity,
  });

  useEffect(() => {
    const current = { namespace, labelSelector, name };
    const effectKey = queryKey(kind, current);
    const source = new EventSource(watchURL(kind, current));
    source.addEventListener("snapshot", (event) => {
      const snapshot = JSON.parse(
        (event as MessageEvent<string>).data,
      ) as WatchSnapshot<Resource<K>>;
      client.setQueryData(effectKey, snapshot.objects);
      setError(undefined);
    });
    source.addEventListener("failure", (event) => {
      const failure = JSON.parse(
        (event as MessageEvent<string>).data,
      ) as WatchFailure;
      setError(failure.message);
    });
    source.addEventListener("delta", (event) => {
      const delta = JSON.parse(
        (event as MessageEvent<string>).data,
      ) as WatchDelta<NamedObject>;
      client.setQueryData<Resource<K>[]>(
        effectKey,
        (list = []) =>
          applyDelta(list as NamedObject[], delta) as Resource<K>[],
      );
    });
    return () => source.close();
  }, [client, kind, namespace, labelSelector, name]);

  return error === undefined ? { items: data } : { items: data, error };
}
