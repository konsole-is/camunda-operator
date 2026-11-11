"use client";

import { useState } from "react";
import { secretRefsOf } from "@/components/fleet/secret-ref-picker";
import { Button } from "@/components/ui/button";
import { clusterFields } from "@/lib/fields";
import { useResources } from "@/lib/k8s/hooks";
import { claimAfter, type Session } from "@/lib/session";
import type { CardId } from "../cards";
import { settingsOf } from "../editable-settings";
import { mergedSpecs } from "../impacts";
import { clusterFieldsInputOf, componentSettings } from "../settings";
import { SettingsEditor } from "../settings-editor";
import type { ClusterSlotContext } from "../slots";
import { changeInputOf } from "./change-input";
import { type ClusterClaim, useClusterEdit } from "./cluster-edit";

/**
 * ComponentSettingsEditor fills component.settings.editor: an "Edit
 * settings" button that opens the editors of each settings group of the
 * open component. Changes go to the session of the edit host. The values
 * are those after the pending changes. After the pending changes are
 * discarded or written, the editors show the live values again. It renders
 * nothing without an open component or without a session.
 */
export function ComponentSettingsEditor({ ctx }: { ctx: ClusterSlotContext }) {
  const { session, pending } = useClusterEdit();
  const [open, setOpen] = useState(false);
  // The editors start from their values, so they remount once nothing is
  // pending, to show the live values.
  const [generation, setGeneration] = useState(0);
  const [wasPending, setWasPending] = useState(pending > 0);
  if (wasPending !== pending > 0) {
    setWasPending(pending > 0);
    if (pending === 0) setGeneration((g) => g + 1);
  }
  if (!ctx.openId || !session) return null;
  return (
    <div className="flex flex-col gap-3">
      <Button
        variant="outline"
        size="sm"
        className="self-start"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        {open ? "Close the editors" : "Edit settings"}
      </Button>
      {open && (
        <Editors
          key={generation}
          ctx={ctx}
          openId={ctx.openId}
          session={session}
        />
      )}
    </div>
  );
}

// Editors follows the Secrets of the namespace only while the editors are
// open.
function Editors({
  ctx,
  openId,
  session,
}: {
  ctx: ClusterSlotContext;
  openId: CardId;
  session: Session<ClusterClaim>;
}) {
  const { set, reset, errors } = useClusterEdit();
  const namespace = ctx.cluster.metadata?.namespace ?? "";
  const secrets = secretRefsOf(useResources("Secret", { namespace }, []).items);
  const merged = mergedSpecs(changeInputOf(ctx, session)).after;
  const claim = claimAfter(session);
  const fields = clusterFields(
    clusterFieldsInputOf({
      cluster: ctx.cluster,
      merged,
      claim,
      sections: ctx.sections,
      registrations: ctx.registrations,
      optimize: ctx.optimize,
      schedules: ctx.schedules,
    }),
  );
  return (
    <div className="flex flex-col gap-6 rounded-lg border border-border p-4">
      {componentSettings(fields, openId, merged.spec).map((group) => (
        <SettingsEditor
          key={group.label}
          group={group}
          settings={settingsOf({
            group,
            spec:
              group.component === "optimize"
                ? (claim.spec.attachments?.optimize ?? {})
                : merged.spec,
          })}
          secrets={secrets}
          errors={errors}
          set={set}
          reset={reset}
        />
      ))}
    </div>
  );
}
