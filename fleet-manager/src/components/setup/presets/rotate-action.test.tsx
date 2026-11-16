// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { WriteMode } from "@/lib/write/types";
import { presetFixtures } from "./fixtures";
import { RotateAction } from "./rotate-action";
import type { PresetActions, PresetSources } from "./sources";

afterEach(cleanup);

const medium = presetFixtures().clusterPresets[0];

function setup(
  opts: {
    mode?: WriteMode;
    sources?: PresetSources;
    unread?: string[];
    disabled?: boolean;
  } = {},
) {
  const mode = "mode" in opts ? opts.mode : "direct";
  const actions = {
    previewChange: vi.fn<PresetActions["previewChange"]>(async () => ({
      objects: [
        {
          ref: { kind: "CamundaClusterPreset", name: "medium" },
          diff: [{ op: "added" as const, text: "passwordRotation" }],
        },
      ],
      errors: [],
    })),
    applyChange: vi.fn<PresetActions["applyChange"]>(async () => ({
      ok: true as const,
      mode: "direct" as const,
    })),
  };
  const show = vi.fn();
  const onReviewing = vi.fn();
  const view = (unread: string[]) => (
    <RotateAction
      preset={medium}
      sources={opts.sources ?? presetFixtures()}
      unread={unread}
      mode={mode}
      disabled={opts.disabled ?? false}
      actions={actions}
      show={show}
      onReviewing={onReviewing}
    />
  );
  const { rerender } = render(view(opts.unread ?? []));
  return {
    actions,
    show,
    onReviewing,
    rerender: (unread: string[]) => rerender(view(unread)),
  };
}

const rotate = () =>
  fireEvent.click(
    screen.getByRole("button", {
      name: "Rotate the admin password of 1 cluster",
    }),
  );

describe("RotateAction", () => {
  it("says how many clusters rotate and that the Connectors pods restart", async () => {
    const { actions, show } = setup();
    rotate();
    const review = screen.getByRole("region", { name: "Review the rotation" });
    expect(review.textContent).toContain(
      "The admin password of each of these clusters changes: team-orders/orders.",
    );
    expect(review.textContent).toContain(
      "The Connectors pods of each cluster restart.",
    );
    const confirm = within(review).getByRole("button", { name: "Rotate" });
    await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
    fireEvent.click(confirm);
    await waitFor(() => expect(actions.applyChange).toHaveBeenCalledOnce());
    const change = actions.applyChange.mock.calls[0][0];
    expect(change.intent).toBe(
      "Rotate the admin password of the clusters of the preset medium",
    );
    await waitFor(() =>
      expect(show).toHaveBeenCalledWith(
        { ok: true, mode: "direct" },
        "The rotation of the admin password is saved.",
      ),
    );
  });

  it("reads Push to Git in Git mode", () => {
    setup({ mode: "git" });
    rotate();
    expect(
      within(
        screen.getByRole("region", { name: "Review the rotation" }),
      ).getByRole("button", { name: "Push to Git" }),
    ).toBeTruthy();
  });

  it("offers no rotation without a cluster to rotate", () => {
    const sources = presetFixtures();
    sources.platformConfigs[0].spec = {
      auth: { method: "oidc" },
    } as (typeof sources.platformConfigs)[number]["spec"];
    setup({ sources });
    expect(screen.queryByRole("button", { name: /^Rotate/ })).toBeNull();
    expect(
      screen.getByText(/No cluster takes a rotation from this preset/),
    ).toBeTruthy();
  });

  it("offers no rotation while the clusters cannot be read", () => {
    setup({ unread: ["CamundaCluster"] });
    expect(screen.queryByRole("button", { name: /^Rotate/ })).toBeNull();
    expect(
      screen.getByText(
        "The rotation is possible when the fleet manager can read CamundaCluster.",
      ),
    ).toBeTruthy();
  });

  it("closes the review when the clusters cannot be read any more", async () => {
    const { actions, rerender, onReviewing } = setup();
    rotate();
    await waitFor(() => expect(onReviewing).toHaveBeenLastCalledWith(true));
    rerender(["CamundaPlatformConfig"]);
    expect(
      screen.queryByRole("region", { name: "Review the rotation" }),
    ).toBeNull();
    await waitFor(() => expect(onReviewing).toHaveBeenLastCalledWith(false));
    expect(actions.applyChange).not.toHaveBeenCalled();
  });

  it("is disabled while disabled is set or without a write mode", () => {
    setup({ disabled: true });
    expect(
      screen.getByRole("button", {
        name: "Rotate the admin password of 1 cluster",
      }),
    ).toHaveProperty("disabled", true);
    cleanup();
    setup({ mode: undefined });
    expect(
      screen.getByRole("button", {
        name: "Rotate the admin password of 1 cluster",
      }),
    ).toHaveProperty("disabled", true);
  });
});
