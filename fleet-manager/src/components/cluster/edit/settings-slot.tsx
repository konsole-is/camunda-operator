"use client";

import { useState } from "react";
import { secretRefsOf } from "@/components/fleet/secret-ref-picker";
import { Button } from "@/components/ui/button";
import { clusterFields, type Step } from "@/lib/fields";
import { useResources } from "@/lib/k8s/hooks";
import { claimAfter, type Session } from "@/lib/session";
import { isSectionId, type ProcessCardId } from "../cards";
import { settingsOf } from "../editable-settings";
import { mergedSpecs } from "../impacts";
import { clusterFieldsInputOf, componentSettings } from "../settings";
import { SettingsEditor } from "../settings-editor";
import type { ClusterSlotContext } from "../slots";
import { changeInputOf } from "./change-input";
import { claimViewAfter } from "./claim-editor";
import { type ClusterClaim, useClusterEdit } from "./cluster-edit";
import { useEditorGeneration } from "./editor-generation";
import { SectionPanel } from "./section-panel";

/**
 * ComponentSettingsEditor fills component.settings.editor: an "Edit
 * settings" button that opens the editors of each settings group of the
 * open component. For a selection of the claim, the button opens the
 * editor that the Claim tab has for its section. Changes go to the session
 * of the edit host. The values are those after the pending changes. After
 * the pending changes are discarded or written, the editors show the live
 * values again. It renders nothing without an open component, without a
 * session, or for a selection that the claim no longer has after the
 * pending changes.
 */
export function ComponentSettingsEditor({ ctx }: { ctx: ClusterSlotContext }) {
  const { session, pending, set, reset, discard } = useClusterEdit();
  const [open, setOpen] = useState(false);
  const { generation, own } = useEditorGeneration(pending);
  if (!ctx.openId || !session) return null;
  const openId = ctx.openId;
  const section = isSectionId(openId)
    ? claimViewAfter(ctx, session).sections.find((s) => s.id === openId)
    : undefined;
  if (isSectionId(openId) && !section) return null;
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
      {open &&
        (section ? (
          <div className="rounded-lg border border-border p-4">
            <SectionPanel context={ctx} section={section} />
          </div>
        ) : (
          !isSectionId(openId) && (
            <Editors
              key={generation}
              ctx={ctx}
              openId={openId}
              session={session}
              set={own(set)}
              reset={own(reset)}
              discard={own((steps: Step[]) => discard(steps))}
            />
          )
        ))}
    </div>
  );
}

// Editors follows the Secrets of the namespace only while the editors are
// open.
function Editors({
  ctx,
  openId,
  session,
  set,
  reset,
  discard,
}: {
  ctx: ClusterSlotContext;
  openId: ProcessCardId;
  session: Session<ClusterClaim>;
  set: (steps: Step[], value: unknown) => void;
  reset: (steps: Step[]) => void;
  discard: (steps: Step[]) => void;
}) {
  const { errors, invalid } = useClusterEdit();
  const namespace = ctx.cluster.metadata?.namespace ?? "";
  const secrets = secretRefsOf(useResources("Secret", { namespace }, []).items);
  const { before, after: merged } = mergedSpecs(changeInputOf(ctx, session));
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
          settings={settingsOf(
            group.component === "optimize"
              ? {
                  group,
                  spec: claim.spec.attachments?.optimize ?? {},
                  live: session.claim.spec.attachments?.optimize ?? {},
                }
              : {
                  group,
                  spec: merged.spec,
                  live: before.spec,
                  claim: claim.spec.cluster ?? {},
                },
          )}
          secrets={secrets}
          errors={errors}
          set={set}
          reset={reset}
          invalid={invalid}
          discard={discard}
        />
      ))}
    </div>
  );
}
