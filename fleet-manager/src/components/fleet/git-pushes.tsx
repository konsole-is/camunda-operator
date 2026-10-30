"use client";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { type GitSyncRef, useGitGone, useGitSync } from "@/lib/k8s/hooks";
import type { KindName } from "@/lib/k8s/kinds";
import type { RefKey, WriteResult } from "@/lib/write/types";
import type { PushedDelete, PushedObject } from "./git-waiting-line";

/** PendingPush is a pushed object or delete that GitOps has not applied yet. */
export type PendingPush = (PushedObject | PushedDelete) & {
  key: RefKey;
  ref: GitSyncRef;
};

type GitPushes = {
  pushes: PendingPush[];
  record(result: WriteResult, pushedAt: string): void;
  settle(key: RefKey): void;
};

const GitPushesContext = createContext<GitPushes | undefined>(undefined);

/**
 * GitPushesProvider holds the pending pushes of the browser tab for
 * useGitPushes, and settles each push when GitOps applied it. It needs a
 * QueryClientProvider above it.
 */
export function GitPushesProvider({ children }: { children: ReactNode }) {
  const [pushes, setPushes] = useState<PendingPush[]>([]);
  const record = useCallback((result: WriteResult, pushedAt: string) => {
    if (!result.ok || result.mode !== "git") return;
    const written: PendingPush[] = (
      Object.entries(result.revisions) as [RefKey, string][]
    ).map(([key, revision]) => ({
      key,
      revision,
      path: result.paths[key] ?? "",
      pushedAt,
      ref: refOfKey(key),
    }));
    const deleted: PendingPush[] = (
      Object.entries(result.deleted) as [RefKey, string][]
    ).map(([key, path]) => ({
      key,
      path,
      pushedAt,
      deleted: true,
      ref: refOfKey(key),
    }));
    const added = [...written, ...deleted];
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
      {pushes.map((push) =>
        "deleted" in push ? (
          <SettleDelete
            key={`${push.key}:delete`}
            push={push}
            settle={settle}
          />
        ) : (
          <SettleWrite
            key={`${push.key}:${push.revision}`}
            push={push}
            settle={settle}
          />
        ),
      )}
    </GitPushesContext.Provider>
  );
}

type Settle = { settle: (key: RefKey) => void };

function SettleWrite({
  push,
  settle,
}: Settle & { push: PushedObject & { key: RefKey; ref: GitSyncRef } }) {
  const sync = useGitSync(push.ref, push.revision);
  useEffect(() => {
    if (sync === "applied") settle(push.key);
  }, [sync, settle, push.key]);
  return null;
}

function SettleDelete({
  push,
  settle,
}: Settle & { push: PushedDelete & { key: RefKey; ref: GitSyncRef } }) {
  const sync = useGitGone(push.ref);
  useEffect(() => {
    if (sync === "applied") settle(push.key);
  }, [sync, settle, push.key]);
  return null;
}

/**
 * useGitPushes returns the pending pushes. record adds one push for each
 * written and each deleted object of a Git result, and replaces an older
 * push of the same object; it ignores any other result. settle removes the push of key. It throws
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
