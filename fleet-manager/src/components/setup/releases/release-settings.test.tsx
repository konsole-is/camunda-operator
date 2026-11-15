// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import type { WriteMode, WriteResult } from "@/lib/write/types";
import { claim, imagesFixture, releaseFixtures } from "./fixtures";
import type { ReleaseActions } from "./release-detail";
import { ReleaseSettings } from "./release-settings";
import { RELEASE_CHANGE_NOTICE } from "./draft";
import type { SourceReads } from "./rows";
import type { ReleasesData } from "./sources";

const router = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));
vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

const SECRETS = {
  "team-orders": [{ name: "orders-db", keys: ["password"] }],
};

function setup(
  opts: {
    name?: string;
    copyFrom?: string;
    mode?: WriteMode;
    sources?: ReleasesData & SourceReads;
    secrets?: Record<string, SecretRefs>;
    result?: WriteResult;
  } = {},
) {
  const {
    sources = releaseFixtures(),
    secrets = SECRETS,
    result = { ok: true, mode: "direct" },
    copyFrom,
  } = opts;
  const name = "name" in opts ? opts.name : "camunda-8-9-0";
  const mode = "mode" in opts ? opts.mode : "direct";
  const actions = {
    previewChange: vi.fn<ReleaseActions["previewChange"]>(async () => ({
      objects: [],
      errors: [],
    })),
    applyChange: vi.fn<ReleaseActions["applyChange"]>(async () => result),
  };
  const view = (s: ReleasesData & SourceReads) => (
    <GitPushesProvider>
      <ReleaseSettings
        sources={{ ...s, errors: [...(s.errors ?? [])] }}
        name={name}
        copyFrom={copyFrom}
        imagesTable={imagesFixture}
        secrets={secrets}
        mode={mode}
        actions={actions}
      />
    </GitPushesProvider>
  );
  const { rerender } = render(view(sources));
  return {
    actions,
    rerender: (s: ReleasesData & SourceReads) => rerender(view(s)),
  };
}

const field = (label: string) =>
  screen.getByLabelText(label) as HTMLInputElement;

async function confirm(label: string) {
  const button = screen.getByRole("button", { name: label });
  await waitFor(() => expect(button).toHaveProperty("disabled", false));
  fireEvent.click(button);
}

function openDeleteReview(name: string) {
  fireEvent.change(screen.getByLabelText(/to confirm/), {
    target: { value: name },
  });
  fireEvent.click(screen.getByRole("button", { name: "Review the delete" }));
  expect(
    screen.getByRole("region", { name: "Review the delete" }),
  ).toBeTruthy();
}

beforeEach(() => router.push.mockClear());
afterEach(cleanup);

describe("ReleaseSettings", () => {
  it("saves a changed version with the resourceVersion and says the pods roll", async () => {
    const { actions } = setup();
    fireEvent.change(field("Camunda version"), { target: { value: "8.9.1" } });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    const review = within(
      screen.getByRole("region", { name: "Review the release" }),
    );
    expect(
      review.getByText(
        "1 cluster, 1 Elasticsearch cluster use this release. Their pods roll.",
      ),
    ).toBeTruthy();
    expect(
      review.getByText(
        "The version changes from 8.9.0 to 8.9.1 for each of them.",
      ),
    ).toBeTruthy();
    await confirm("Save");
    await waitFor(() => expect(actions.applyChange).toHaveBeenCalledTimes(1));
    expect(actions.applyChange.mock.calls[0][0]).toMatchObject({
      intent: "Change the release camunda-8-9-0",
      preconditions: { "CamundaRelease//camunda-8-9-0": "41" },
      force: true,
      apply: [{ spec: { version: "8.9.1" } }],
    });
  });

  it("shows the notice that a change reaches every user", () => {
    setup();
    expect(screen.getByRole("note").textContent).toBe(RELEASE_CHANGE_NOTICE);
  });

  it("keeps a Secret entry that no user namespace holds and marks where it is missing", () => {
    setup({ secrets: { "team-orders": [] } });
    const every = within(screen.getByRole("group", { name: "Every workload" }));
    const row = within(every.getByRole("group", { name: "Variable 2" }));
    expect(
      (row.getByRole("combobox", { name: "Secret" }) as HTMLSelectElement)
        .value,
    ).toBe("orders-db");
    expect(
      (row.getByRole("combobox", { name: "Key" }) as HTMLSelectElement).value,
    ).toBe("password");
    expect(row.getByText("Missing in team-orders")).toBeTruthy();
    fireEvent.change(field("Camunda version"), { target: { value: "8.9.1" } });
    expect(screen.getByRole("button", { name: "Review" })).toHaveProperty(
      "disabled",
      false,
    );
  });

  it("shows the Stale error and Discard loads the live release", async () => {
    setup({
      result: {
        ok: false,
        errors: [
          {
            code: "Stale",
            message: "The release camunda-8-9-0 changed since you opened it.",
          },
        ],
      },
    });
    fireEvent.change(field("Camunda version"), { target: { value: "8.9.1" } });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    await confirm("Save");
    await waitFor(() =>
      expect(
        screen.getAllByText(
          "The release camunda-8-9-0 changed since you opened it.",
        ).length,
      ).toBeGreaterThan(0),
    );
  });

  it("Discard loads the live copy that arrived while the form was open", () => {
    const { rerender } = setup();
    fireEvent.change(field("Camunda version"), { target: { value: "8.9.1" } });
    const newer = releaseFixtures();
    newer.releases[0].spec.version = "8.9.5";
    newer.releases[0].metadata.resourceVersion = "42";
    rerender(newer);
    expect(field("Camunda version").value).toBe("8.9.1");
    fireEvent.click(screen.getByRole("button", { name: "Discard" }));
    expect(field("Camunda version").value).toBe("8.9.5");
  });

  it("offers delete only for a release that can be deleted", () => {
    setup({ name: "camunda-8-9-1" });
    expect(
      screen.getByRole("button", { name: "Review the delete" }),
    ).toBeTruthy();
    cleanup();
    setup();
    expect(
      screen.getByText(
        "Delete is possible when no cluster, Elasticsearch cluster, database server, or claim names this release and no environment allows it.",
      ),
    ).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: "Review the delete" }),
    ).toBeNull();
  });

  it("deletes the release with its precondition and its UID and goes to the list", async () => {
    const { actions } = setup({ name: "camunda-8-9-1" });
    fireEvent.change(screen.getByLabelText(/to confirm/), {
      target: { value: "camunda-8-9-1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Review the delete" }));
    await confirm("Delete the release");
    await waitFor(() =>
      expect(router.push).toHaveBeenCalledWith("/setup/releases"),
    );
    expect(actions.applyChange.mock.calls[0][0]).toMatchObject({
      delete: [{ kind: "CamundaRelease", name: "camunda-8-9-1" }],
      preconditions: { "CamundaRelease//camunda-8-9-1": "41" },
      uids: { "CamundaRelease//camunda-8-9-1": "uid-camunda-8-9-1" },
    });
  });

  it("closes the delete review when a claim names the release", () => {
    const { rerender } = setup({ name: "camunda-8-9-1" });
    openDeleteReview("camunda-8-9-1");
    const sources = releaseFixtures();
    sources.claims.push(claim("team-new", "late", "camunda-8-9-1"));
    rerender(sources);
    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
    expect(
      screen.queryByRole("button", { name: "Review the delete" }),
    ).toBeNull();
  });

  it("offers no delete while a list cannot be read and closes an open delete review", () => {
    const { rerender } = setup({ name: "camunda-8-9-1" });
    openDeleteReview("camunda-8-9-1");
    rerender({
      ...releaseFixtures(),
      errors: [{ kind: "CamundaClusterClaim", message: "forbidden" }],
    });
    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
    expect(
      screen.getByText(
        "Delete is possible when the fleet manager can read CamundaClusterClaim.",
      ),
    ).toBeTruthy();
  });

  it("offers no delete while a list has no snapshot on the connection", () => {
    setup({
      name: "camunda-8-9-1",
      sources: { ...releaseFixtures(), stale: ["CamundaCluster"] },
    });
    expect(
      screen.getByText(
        "Delete is possible when the fleet manager can read CamundaCluster.",
      ),
    ).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: "Review the delete" }),
    ).toBeNull();
  });

  it("copies the release of copyFrom for a new release and creates it by name", async () => {
    const { actions } = setup({
      name: undefined,
      copyFrom: "camunda-8-9-0",
      secrets: {},
    });
    expect(
      (screen.getByLabelText("Copy from") as HTMLSelectElement).value,
    ).toBe("camunda-8-9-0");
    expect(field("Camunda version").value).toBe("8.9.0");
    expect(screen.queryByRole("note")).toBeNull();
    fireEvent.change(field("Release name"), {
      target: { value: "camunda-8-9-2" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    expect(
      screen.getByText("The release is new. No resource uses it yet."),
    ).toBeTruthy();
    await confirm("Create");
    await waitFor(() => expect(actions.applyChange).toHaveBeenCalledTimes(1));
    const change = actions.applyChange.mock.calls[0][0];
    expect(change.create).toEqual(["CamundaRelease//camunda-8-9-2"]);
    expect(change.apply[0]).toMatchObject({
      metadata: { name: "camunda-8-9-2" },
      spec: {
        version: "8.9.0",
        extraEnv: releaseFixtures().releases[0].spec.extraEnv,
      },
    });
  });

  it("keeps the typed name when the copy source changes", () => {
    setup({ name: undefined, secrets: {} });
    fireEvent.change(field("Release name"), { target: { value: "next" } });
    fireEvent.change(screen.getByLabelText("Copy from"), {
      target: { value: "camunda-8-8-5" },
    });
    expect(field("Release name").value).toBe("next");
    expect(field("Camunda version").value).toBe("8.8.5");
  });

  it("blocks a save with a version that is not x.y.z", () => {
    setup();
    fireEvent.change(field("Camunda version"), { target: { value: "8.9" } });
    expect(
      screen.getAllByText("The Camunda version is a version such as 8.9.0.")
        .length,
    ).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Review" })).toHaveProperty(
      "disabled",
      true,
    );
  });

  it("disables the form while changes are off", () => {
    setup({ mode: undefined });
    expect(screen.getByText(CHANGES_OFF)).toBeTruthy();
    expect(field("Camunda version").matches(":disabled")).toBe(true);
  });
});
