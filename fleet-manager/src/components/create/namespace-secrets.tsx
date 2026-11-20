"use client";

import { useEffect } from "react";
import {
  type SecretRefs,
  secretRefsOf,
} from "@/components/fleet/secret-ref-picker";
import { useResources, useWatchesOverdue } from "@/lib/k8s/hooks";

/**
 * NamespaceSecrets follows the Secrets of namespace and reports their refs,
 * with the read error, to onChange each time they change. While the watch
 * is overdue and there is no read error, the error says that the list is
 * out of date. It renders nothing: a wrapper around a step would remount
 * the step, and with it the namespace picker, when the choice changes.
 * onChange must be stable.
 */
export function NamespaceSecrets({
  namespace,
  onChange,
}: {
  namespace: string;
  onChange: (secrets: SecretRefs, error?: string) => void;
}) {
  const { items, error } = useResources("Secret", { namespace }, []);
  const [overdue] = useWatchesOverdue([
    { kind: "Secret", opts: { namespace } },
  ]);
  const reported =
    error ?? (overdue ? "the list of Secrets is out of date" : undefined);
  const key = JSON.stringify(secretRefsOf(items));
  useEffect(() => {
    onChange(JSON.parse(key) as SecretRefs, reported);
  }, [key, reported, onChange]);
  return null;
}
