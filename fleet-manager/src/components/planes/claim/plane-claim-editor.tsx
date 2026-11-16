"use client";

import { type ReactNode, useState } from "react";
import { EditButton, EditPanel } from "@/components/cluster/edit/claim-editor";
import { planeExternalUrls } from "@/lib/planes/urls";
import { claimAfter } from "@/lib/session";
import { BasicsPanel } from "./basics-panel";
import { ComponentsPanel } from "./components-panel";
import { IdentityPanel } from "./identity-panel";
import {
  changesWaitText,
  planeFieldsOf,
  usePlaneClaimEdit,
} from "./plane-claim-edit";
import {
  PLANE_CLAIM_PANELS,
  type PlaneClaimPanel,
  PlaneClaimTab,
  type PlaneClaimTabEdit,
} from "./plane-claim-tab";
import { PlaneDeletionPanel } from "./plane-deletion-panel";
import { PlaneSectionPanel } from "./plane-section-panel";
import { ServedPanel } from "./served-panel";
import { VersionsPanel } from "./versions-panel";

const EDITORS: Record<PlaneClaimPanel, () => ReactNode> = {
  basics: () => <BasicsPanel />,
  identity: () => <IdentityPanel />,
  environments: () => <ServedPanel />,
  components: () => <ComponentsPanel />,
  versions: () => <VersionsPanel />,
  databases: () => <PlaneSectionPanel group="databases" />,
  access: () => <PlaneSectionPanel group="access" />,
  deletion: () => <PlaneDeletionPanel />,
};

/**
 * PlaneClaimEditor is the Claim tab of a plane, with an Edit button on
 * each panel. One panel is in edit at a time, in place of its read panel,
 * and Done closes it and keeps its pending changes. The read view shows the
 * claim after the pending changes. Without a session it is read only. While
 * the page could not read the plane or its claim, it offers no edit and
 * says why. requested opens its panel on mount. It needs a
 * PlaneClaimEditProvider above it.
 */
export function PlaneClaimEditor({
  requested,
}: {
  requested?: PlaneClaimPanel;
}) {
  const plane = usePlaneClaimEdit();
  const [open, setOpen] = useState(requested);
  const [focus, setFocus] = useState<{
    panel: PlaneClaimPanel;
    on: "panel" | "button";
  }>();
  const stored = plane.claim;
  if (!stored)
    return (
      <p className="text-sm text-muted">
        {plane.claimUnread
          ? changesWaitText(plane.unread)
          : "No claim made this management plane, so it has no Claim tab to change."}
      </p>
    );
  const session = plane.edit?.session;
  const shown = session ? claimAfter(session) : stored;
  const fields = planeFieldsOf(shown, plane);
  const urls = planeExternalUrls(plane.plane);
  const note = plane.claimUnread && (
    <p className="text-sm text-muted">{changesWaitText(plane.unread)}</p>
  );
  if (!session || plane.claimUnread)
    return (
      <div className="flex flex-col gap-4">
        {note}
        <PlaneClaimTab fields={fields} stored={stored} urls={urls} />
      </div>
    );

  const edit: Required<PlaneClaimTabEdit> = { panels: {}, actions: {} };
  for (const { id, label } of PLANE_CLAIM_PANELS) {
    const focused = focus?.panel === id ? focus.on : undefined;
    if (id === open)
      edit.panels[id] = (
        <EditPanel
          def={{ label, name: label, render: EDITORS[id] }}
          scroll={false}
          focus={focused === "panel"}
          onShown={() => setFocus(undefined)}
          onDone={() => {
            setOpen(undefined);
            setFocus({ panel: id, on: "button" });
          }}
        />
      );
    else
      edit.actions[id] = (
        <EditButton
          name={label}
          focus={focused === "button"}
          onFocused={() => setFocus(undefined)}
          onClick={() => {
            setOpen(id);
            setFocus({ panel: id, on: "panel" });
          }}
        />
      );
  }
  return (
    <PlaneClaimTab fields={fields} stored={stored} urls={urls} edit={edit} />
  );
}
