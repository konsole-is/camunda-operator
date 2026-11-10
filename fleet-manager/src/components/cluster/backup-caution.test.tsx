// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ClaimReview } from "@/components/fleet/claim-review";
import type { Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import type { RollBackupLike } from "@/lib/policy/backups";
import { openSession, setValue, type Session } from "@/lib/session";
import type { WriteMode } from "@/lib/write/types";
import { backupReview } from "./backup-caution";
import type { ClusterChangeInput } from "./impacts";

afterEach(cleanup);

type Claim = Resource<"CamundaClusterClaim">;
const f = (field: string): Step => ({ field });

const REFS = { presetRef: "medium", releaseRef: "camunda-8-9-5" };
const SPEC = {
  zeebe: { replicas: 3, storageSize: "32Gi" },
  gateway: { replicas: 2 },
};

function input(steps: Step[], value: unknown): ClusterChangeInput {
  const claim = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { name: "orders", namespace: "team-a", resourceVersion: "41" },
    spec: { ...REFS, cluster: SPEC },
  } as unknown as Claim;
  const s = openSession({
    claim,
    resource: { metadata: { labels: { [CLAIM_LABEL]: "orders" } } },
    locks: () => undefined,
  })!;
  const result = setValue(s, steps, value);
  if (!("session" in result)) throw new Error("refused");
  return {
    session: result.session as Session<Claim>,
    live: {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaCluster",
      metadata: {
        name: "orders",
        namespace: "team-a",
        labels: { [CLAIM_LABEL]: "orders" },
      },
      spec: { ...REFS, ...SPEC },
    } as unknown as Resource<"CamundaCluster">,
    presets: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaClusterPreset",
        metadata: { name: "medium" },
        spec: { cluster: {} },
      } as unknown as Resource<"CamundaClusterPreset">,
    ],
    releases: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaRelease",
        metadata: { name: "camunda-8-9-5" },
        spec: { version: "8.9.5" },
      } as unknown as Resource<"CamundaRelease">,
    ],
    sections: [],
  };
}

const rollingChange = input(
  [f("spec"), f("cluster"), f("zeebe"), f("resources")],
  {
    limits: { cpu: "2" },
  },
);
const deletionPolicyChange = input([f("spec"), f("deletionPolicy")], "Orphan");
const runningRdbms = {
  kind: "LogicalBackupRDBMS",
  metadata: { namespace: "team-a", name: "nightly" },
  spec: { clusterRef: { name: "orders" } },
  status: { phase: "Running" },
} as RollBackupLike;

function renderReview(
  i: ClusterChangeInput,
  backups: RollBackupLike[],
  mode: WriteMode,
  onOpenBackups?: () => void,
) {
  render(
    <ClaimReview
      {...backupReview(i, backups, mode, {
        backupsHref: "?tab=backups",
        onOpenBackups,
      })}
      session={i.session}
      mode={mode}
      intent="Change orders"
      preview={async () => ({ objects: [], errors: [] })}
      apply={async () => ({ ok: true, mode: "direct" })}
      show={() => {}}
    />,
  );
}

describe("backupReview", () => {
  it.each([
    ["direct", "Change anyway"],
    ["git", "Push anyway"],
  ] as const)(
    "a caution in %s mode makes the confirm the danger button %s",
    async (mode, label) => {
      renderReview(rollingChange, [runningRdbms], mode);
      const note = screen.getByRole("note", { name: "Caution" });
      expect(note.textContent).toContain(
        "CAUTION: Wait until backup nightly finishes.",
      );
      expect(note.textContent).toContain(
        "The backup waits up to 10 minutes for the rollout to finish.",
      );
      expect(
        note.textContent?.includes("GitOps applies the change later."),
      ).toBe(mode === "git");
      const button = await waitFor(() =>
        screen.getByRole<HTMLButtonElement>("button", { name: label }),
      );
      expect(button.getAttribute("data-variant")).toBe("destructive");
    },
  );

  it("the link opens the Backups tab", async () => {
    const onOpenBackups = vi.fn();
    renderReview(rollingChange, [runningRdbms], "direct", onOpenBackups);
    const link = screen.getByRole("link", { name: "Open the Backups tab" });
    expect(link.getAttribute("href")).toBe("?tab=backups");
    await userEvent.click(link);
    expect(onOpenBackups).toHaveBeenCalledOnce();
  });

  it.each([
    ["direct", "Save"],
    ["git", "Push to Git"],
  ] as const)("a muted line in %s mode keeps the confirm %s", (mode, label) => {
    renderReview(deletionPolicyChange, [runningRdbms], mode);
    expect(screen.queryByRole("note", { name: "Caution" })).toBeNull();
    expect(
      screen.getByText(
        "Backup nightly is running. This change does not roll the brokers, so it does not affect the backup.",
      ),
    ).toBeTruthy();
    expect(
      screen.getByRole("button", { name: label }).getAttribute("data-variant"),
    ).toBe("default");
  });

  it("puts the muted line at the end of the impacts", () => {
    expect(
      backupReview(deletionPolicyChange, [runningRdbms], "direct", {
        backupsHref: "?tab=backups",
      }),
    ).toEqual({
      impacts: [
        "Nothing restarts. The policy applies when the cluster is deleted.",
        "Backup nightly is running. This change does not roll the brokers, so it does not affect the backup.",
      ],
    });
  });

  it("no running backup gives the impacts only", () => {
    renderReview(rollingChange, [], "direct");
    expect(screen.queryByRole("note", { name: "Caution" })).toBeNull();
    expect(screen.getByText("Broker pods restart.")).toBeTruthy();
    expect(screen.queryByText(/is running/)).toBeNull();
  });
});
