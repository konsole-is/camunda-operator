"use client";

import { Pencil } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { type ReactNode, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { claimAfter, type Session } from "@/lib/session";
import { claimView, type ClaimView } from "../claim";
import { ClaimTab, type ClaimTabEdit, Panel } from "../claim-tab";
import { mergedSpecs } from "../impacts";
import type { ClusterSlotContext } from "../slots";
import { type ClaimPanel, claimPanelOf } from "../tabs";
import { AttachmentsPanel } from "./attachments-panel";
import { BasicsPanel } from "./basics-panel";
import { changeInputOf } from "./change-input";
import { type ClusterClaim, useClusterEdit } from "./cluster-edit";
import { ReleasePanel } from "./release-panel";
import { SectionPanel } from "./section-panel";

// EditDef is one panel of the Claim tab that has an editor. label is the
// heading, name the accessible name of the panel.
type EditDef = {
  panel: ClaimPanel;
  label: string;
  name: string;
  render: () => ReactNode;
};

/**
 * ClaimEditor fills cluster.claim.editor: ClaimEditorPanels with the panel
 * that the section query parameter names.
 */
export function ClaimEditor({ context }: { context: ClusterSlotContext }) {
  const requested = claimPanelOf(useSearchParams().get("section"));
  return <ClaimEditorPanels context={context} requested={requested} />;
}

/**
 * ClaimEditorPanels is the Claim tab with an Edit button on each panel that
 * has an editor. One panel is in edit at a time, in place of its read
 * panel. Done closes it and keeps its pending changes. The read view shows
 * the claim after the pending changes. requested opens its panel and
 * scrolls it into view, on mount and on each change. Without a session, it
 * shows the read view with no Edit button. It renders nothing without the
 * claim.
 */
export function ClaimEditorPanels({
  context,
  requested,
}: {
  context: ClusterSlotContext;
  requested?: ClaimPanel;
}) {
  const edit = useClusterEdit();
  const [open, setOpen] = useState(requested);
  const [seen, setSeen] = useState(requested);
  const [scroll, setScroll] = useState(requested);
  const [focus, setFocus] = useState<{ panel: ClaimPanel; on: Focus }>();
  if (requested !== seen) {
    setSeen(requested);
    setOpen(requested);
    setScroll(requested);
  }
  const stored = context.claim;
  if (!stored) return null;
  const session = edit.session;
  const view = viewOf(context, stored, session);
  if (!session)
    return <ClaimTab view={view} stored={stored} now={context.now} />;

  const panels: ClaimTabEdit["panels"] = {};
  const actions: ClaimTabEdit["actions"] = {};
  for (const def of editDefs(context, view)) {
    const focused = focus?.panel === def.panel ? focus.on : undefined;
    if (def.panel === open)
      panels[def.panel] = (
        <EditPanel
          def={def}
          scroll={scroll === def.panel}
          focus={focused === "panel"}
          onShown={() => {
            setScroll(undefined);
            setFocus(undefined);
          }}
          onDone={() => {
            setOpen(undefined);
            setFocus({ panel: def.panel, on: "button" });
          }}
        />
      );
    else
      actions[def.panel] = (
        <EditButton
          name={def.name}
          focus={focused === "button"}
          onFocused={() => setFocus(undefined)}
          onClick={() => {
            setOpen(def.panel);
            setFocus({ panel: def.panel, on: "panel" });
          }}
        />
      );
  }
  return (
    <ClaimTab
      view={view}
      stored={stored}
      now={context.now}
      edit={{ panels, actions }}
    />
  );
}

// Focus says where the focus goes after a panel opens or closes: into the
// panel, or to its Edit button.
type Focus = "panel" | "button";

function editDefs(context: ClusterSlotContext, view: ClaimView): EditDef[] {
  return [
    {
      panel: "basics",
      label: "Basics",
      name: "Basics",
      render: () => <BasicsPanel view={view} />,
    },
    {
      panel: "release",
      label: "Release and preset",
      name: "Release and preset",
      render: () => <ReleasePanel context={context} />,
    },
    ...view.sections.map((section): EditDef => ({
      panel: section.id,
      label: section.sectionLabel,
      name: `${section.sectionLabel}: ${section.optionLabel}`,
      render: () => <SectionPanel context={context} section={section} />,
    })),
    {
      panel: "attachments",
      label: "Attachments",
      name: "Attachments",
      render: () => <AttachmentsPanel context={context} />,
    },
  ];
}

function viewOf(
  context: ClusterSlotContext,
  stored: ClusterClaim,
  session: Session<ClusterClaim> | undefined,
): ClaimView {
  const merged = session
    ? mergedSpecs(changeInputOf(context, session)).after
    : context.merged;
  return claimView({
    claim: session ? claimAfter(session) : stored,
    cluster: context.cluster,
    merged,
    environments: context.environments,
    sections: context.sections,
    registrations: context.registrations,
  });
}

function EditPanel({
  def,
  scroll,
  focus,
  onShown,
  onDone,
}: {
  def: EditDef;
  scroll: boolean;
  focus: boolean;
  onShown: () => void;
  onDone: () => void;
}) {
  const ref = useRef<HTMLElement>(null);
  useEffect(() => {
    if (!scroll && !focus) return;
    if (scroll) ref.current?.scrollIntoView({ block: "start" });
    if (focus) ref.current?.focus();
    onShown();
  }, [scroll, focus, onShown]);
  return (
    <Panel
      ref={ref}
      label={def.label}
      name={def.name}
      editing
      action={
        <Button size="sm" variant="outline" onClick={onDone}>
          Done
        </Button>
      }
    >
      {def.render()}
    </Panel>
  );
}

function EditButton({
  name,
  focus,
  onFocused,
  onClick,
}: {
  name: string;
  focus: boolean;
  onFocused: () => void;
  onClick: () => void;
}) {
  const ref = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (!focus) return;
    ref.current?.focus();
    onFocused();
  }, [focus, onFocused]);
  return (
    <Button
      ref={ref}
      size="sm"
      variant="ghost"
      aria-label={`Edit ${name}`}
      onClick={onClick}
    >
      <Pencil aria-hidden />
      Edit
    </Button>
  );
}
