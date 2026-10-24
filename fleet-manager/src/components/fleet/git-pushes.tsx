"use client";
import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import type { GitSyncRef } from "@/lib/k8s/hooks";
import type { KindName } from "@/lib/k8s/kinds";
import type { RefKey, WriteResult } from "@/lib/write/types";
import type { PushedObject } from "./git-waiting-line";

/** PendingPush is a pushed object that GitOps has not applied yet. */
export type PendingPush = PushedObject & { key: RefKey; ref: GitSyncRef };

type GitPushes = {
  pushes: PendingPush[];
  record(result: WriteResult, pushedAt: string): void;
  settle(key: RefKey): void;
};

const GitPushesContext = createContext<GitPushes | undefined>(undefined);

/** GitPushesProvider holds the pending pushes of the browser tab for useGitPushes. */
export function GitPushesProvider({ children }: { children: ReactNode }) {
  const [pushes, setPushes] = useState<PendingPush[]>([]);
  const record = useCallback((result: WriteResult, pushedAt: string) => {
    if (!result.ok || result.mode !== "git") return;
    const added = (Object.entries(result.revisions) as [RefKey, string][]).map(
      ([key, revision]) => ({
        key,
        revision,
        path: result.paths[key] ?? "",
        pushedAt,
        ref: refOfKey(key),
      }),
    );
    setPushes((old) => [
      ...old.filter((p) => !added.some((a) => a.key === p.key)),
      ...added,
    ]);
  }, []);
  const settle = useCallback(
    (key: RefKey) => setPushes((old) => old.filter((p) => p.key !== key)),
    [],
  );
  const value = useMemo(
    () => ({ pushes, record, settle }),
    [pushes, record, settle],
  );
  return (
    <GitPushesContext.Provider value={value}>
      {children}
    </GitPushesContext.Provider>
  );
}

/**
 * useGitPushes returns the pending pushes. record adds one push for each
 * object of a Git result, and replaces an older push of the same object;
 * it ignores any other result. settle removes the push of key. It throws
 * without a GitPushesProvider above it.
 */
export function useGitPushes(): GitPushes {
  const pushes = useContext(GitPushesContext);
  if (!pushes) throw new Error("useGitPushes needs a GitPushesProvider");
  return pushes;
}

function refOfKey(key: RefKey): GitSyncRef {
  const [kind, namespace, name] = key.split("/");
  return {
    kind: kind as KindName,
    ...(namespace ? { namespace } : {}),
    name,
  };
}
