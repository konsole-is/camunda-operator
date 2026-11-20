"use client";

import { byCodeUnit } from "@/lib/policy";
import { claimAfter, fieldLock } from "@/lib/session";
import { AttachmentFields } from "../attachment-fields";
import type { ClusterSlotContext } from "../slots";
import { useClusterEdit } from "./cluster-edit";

/**
 * AttachmentsPanel edits the attachments of the claim through
 * AttachmentFields, with the claim after the pending changes of the session
 * and the rules of the cluster in context. A locked external URL with no
 * claim value shows the URL of the running Optimize. It renders nothing
 * without a session.
 */
export function AttachmentsPanel({ context }: { context: ClusterSlotContext }) {
  const edit = useClusterEdit();
  const { session } = edit;
  if (!session) return null;
  return (
    <AttachmentFields
      value={claimAfter(session).spec?.attachments}
      set={edit.set}
      reset={edit.reset}
      invalid={edit.invalid}
      context={{
        secondaryStorage: context.storage?.spec.type,
        backupStorage: context.backupStorage.kind !== "none",
        managementAuthConfigs: context.managementAuthConfigs
          .map((a) => a.metadata?.name ?? "")
          .sort(byCodeUnit),
        lock: (steps) => fieldLock(session, steps),
        errors: edit.errors,
        pending: edit.pending,
        lockedUrl: context.optimize?.spec.externalUrl,
        optimizeHref: context.optimize
          ? "?tab=overview&component=optimize"
          : undefined,
      }}
    />
  );
}
