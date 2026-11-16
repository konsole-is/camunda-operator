"use client";

import { AttachmentFields } from "@/components/cluster/attachment-fields";
import type { ClusterAttachments } from "@/lib/clusters/attachments";
import type { StorageSummary } from "@/lib/clusters/storage";
import type { FieldLock, Step } from "@/lib/fields";
import type { KindName } from "@/lib/k8s/kinds";

/**
 * ClusterAttachmentsStep is step 4 of the cluster create flow: Optimize and
 * the backup schedule of the draft, with the rules of AttachmentFields over
 * the step 2 summary. lock gives the lock of a claim path of the draft. A
 * locked Optimize URL says whether the access option fills it later.
 * unread are the kinds that could not be read.
 */
export function ClusterAttachmentsStep({
  value,
  summary,
  managementAuthConfigs,
  lock,
  set,
  reset,
  invalid,
  unread,
}: {
  value: ClusterAttachments | undefined;
  summary: StorageSummary;
  managementAuthConfigs: readonly string[];
  lock: (steps: Step[]) => FieldLock | undefined;
  set: (steps: Step[], value: unknown) => void;
  reset: (steps: Step[]) => void;
  invalid: (steps: Step[], message: string | undefined) => void;
  unread?: readonly KindName[];
}) {
  return (
    <AttachmentFields
      value={value}
      set={set}
      reset={reset}
      invalid={invalid}
      context={{
        secondaryStorage: summary.secondaryStorageType,
        backupStorage: summary.backupStorage !== undefined,
        managementAuthConfigs,
        lock,
        errors: [],
        pending: 0,
        lockedUrl: summary.optimizeUrl
          ? "Known once the access option is ready"
          : "None: the access option gives no Optimize URL",
        unread,
      }}
    />
  );
}
