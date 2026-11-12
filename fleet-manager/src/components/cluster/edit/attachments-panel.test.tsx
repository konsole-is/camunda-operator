// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyClaimChange,
  previewClaimChange,
} from "@/app/fleet/clusters/[namespace]/[name]/actions";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import type { ClusterField } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { FakeEventSource } from "../../../../test/fakes/event-source";
import { claim, slotContext } from "../../../../test/fakes/slot-context";
import { fakeWriter } from "../../../../test/fakes/writer";
import type { BackupStorage } from "../backups";
import type { ClusterSlotContext } from "../slots";
import { ClaimEditorPanels } from "./claim-editor";
import { ClusterEditProvider } from "./cluster-edit";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));

const actions = { preview: previewClaimChange, apply: applyClaimChange };

const storage = (type: string) =>
  ({
    kind: "SecondaryStorageConfig",
    metadata: { namespace: "team-a", name: "orders-storage" },
    spec: { type },
  }) as unknown as Resource<"SecondaryStorageConfig">;

const authConfig = (name: string) =>
  ({
    kind: "ManagementAuthConfig",
    metadata: { name },
    spec: {},
  }) as unknown as Resource<"ManagementAuthConfig">;

const rdbmsBackups: BackupStorage = {
  kind: "rdbms",
  bucket: { name: "backups", location: "s3://acme-backups/orders" },
  schedules: [],
};

// withOptimize gives the claim an Optimize attachment, as a claim that has
// one stores it.
const withOptimize = (optimize: object) =>
  claim("41", { attachments: { optimize } });

function panel(over: Partial<ClusterSlotContext> = {}) {
  const context = slotContext({
    storage: storage("elasticsearch"),
    managementAuthConfigs: [authConfig("keycloak"), authConfig("camunda")],
    ...over,
  });
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>
        <ClusterEditProvider context={context} actions={actions}>
          <ClaimEditorPanels context={context} requested="attachments" />
        </ClusterEditProvider>
      </GitPushesProvider>
    </QueryClientProvider>,
  );
}

const attachments = () => screen.getByRole("region", { name: "Attachments" });
const box = (name: string) =>
  within(attachments()).getByRole<HTMLButtonElement>("checkbox", { name });
const field = (label: string) =>
  within(attachments()).getByLabelText<HTMLInputElement>(label);
const bar = () => screen.queryByRole("region", { name: "Unsaved changes" });

async function save() {
  fireEvent.click(within(bar()!).getByRole("button", { name: "Review" }));
  const button = within(
    screen.getByRole("dialog", { name: "Review the change" }),
  ).getByRole<HTMLButtonElement>("button", { name: "Save" });
  await waitFor(() => expect(button.disabled).toBe(false));
  fireEvent.click(button);
  await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
  return (fakeWriter.applied[0].apply[0] as Resource<"CamundaClusterClaim">)
    .spec;
}

// refuse makes the next write fail with an error at path.
function refuse(path: string) {
  fakeWriter.results = [
    {
      ok: false,
      errors: [
        {
          code: "Invalid",
          message: "the schedule is not a cron expression",
          path,
        },
      ],
    },
  ];
}

// saveAndGoBack saves the change, waits for the refusal, and closes the
// review.
async function saveAndGoBack() {
  fireEvent.click(within(bar()!).getByRole("button", { name: "Review" }));
  const dialog = screen.getByRole("dialog", { name: "Review the change" });
  const button = within(dialog).getByRole<HTMLButtonElement>("button", {
    name: "Save",
  });
  await waitFor(() => expect(button.disabled).toBe(false));
  fireEvent.click(button);
  await waitFor(() =>
    expect(within(dialog).getByRole("alert").textContent).toContain(
      "the schedule is not a cron expression",
    ),
  );
  fireEvent.click(within(dialog).getByRole("button", { name: "Back" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
}

beforeEach(() => {
  Element.prototype.scrollIntoView = () => {};
  fakeWriter.reset();
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("the Attachments panel", () => {
  it("turning Optimize on writes the flat attachment fields", async () => {
    panel();
    fireEvent.click(box("Attach Optimize"));
    expect(field("ManagementAuthConfig").value).toBe("camunda");
    await userEvent.type(field("External URL"), "https://optimize.example.com");
    const spec = await save();
    expect(spec.attachments?.optimize).toEqual({
      managementAuthConfigRef: "camunda",
      externalUrl: "https://optimize.example.com",
    });
  });

  it("changes the ManagementAuthConfig of an attached Optimize", async () => {
    panel({ claim: withOptimize({ managementAuthConfigRef: "camunda" }) });
    fireEvent.change(field("ManagementAuthConfig"), {
      target: { value: "keycloak" },
    });
    expect((await save()).attachments?.optimize).toEqual({
      managementAuthConfigRef: "keycloak",
    });
  });

  it("turning Optimize off removes the attachment", async () => {
    panel({
      claim: withOptimize({
        managementAuthConfigRef: "camunda",
        importer: { replicas: 1 },
      }),
    });
    fireEvent.click(box("Attach Optimize"));
    expect((await save()).attachments).toBeUndefined();
  });

  it("the external URL is locked Filled by Access, with the URL of the live Optimize, when the access option fills it", () => {
    const filled: ClusterField = {
      key: "CamundaOptimize:spec.externalUrl",
      target: { kind: "CamundaOptimize" },
      group: "optimize",
      path: "spec.externalUrl",
      label: "externalUrl",
      value: "https://orders.example.com/optimize",
      state: { kind: "filled", section: "access", sectionLabel: "Access" },
      lock: { reason: "filled", section: "access", sectionLabel: "Access" },
      docNames: ["spec", "externalUrl"],
    };
    panel({
      claim: withOptimize({ managementAuthConfigRef: "camunda" }),
      fields: [filled],
      optimize: {
        kind: "CamundaOptimize",
        metadata: { namespace: "team-a", name: "orders-optimize" },
        spec: { externalUrl: "https://orders.example.com/optimize" },
      } as unknown as Resource<"CamundaOptimize">,
    });
    const url = within(attachments()).getByRole("group", {
      name: "External URL",
    });
    expect(within(url).getByText("Filled by Access")).toBeTruthy();
    expect(
      within(url).getByText("https://orders.example.com/optimize"),
    ).toBeTruthy();
    expect(within(url).queryByRole("textbox")).toBeNull();
  });

  it.each([
    ["PostgreSQL", { storage: storage("rdbms") }],
    ["no", { storage: undefined }],
  ])("Optimize is off with a note with %s secondary storage", (_, over) => {
    panel(over);
    expect(box("Attach Optimize").disabled).toBe(true);
    expect(
      within(attachments()).getByText(
        "Optimize needs Elasticsearch as secondary storage.",
      ),
    ).toBeTruthy();
  });

  it("Optimize needs a ManagementAuthConfig", () => {
    panel({ managementAuthConfigs: [] });
    expect(box("Attach Optimize").disabled).toBe(true);
    expect(
      within(attachments()).getByText(
        "No ManagementAuthConfig exists. Your admin creates one first.",
      ),
    ).toBeTruthy();
  });

  it("links to Optimize on the Overview tab", () => {
    panel({
      claim: withOptimize({ managementAuthConfigRef: "camunda" }),
      optimize: {
        kind: "CamundaOptimize",
        metadata: { namespace: "team-a", name: "orders-optimize" },
        spec: {},
      } as unknown as Resource<"CamundaOptimize">,
    });
    expect(
      within(attachments())
        .getByRole("link", { name: "Open Optimize" })
        .getAttribute("href"),
    ).toBe("?tab=overview&component=optimize");
  });

  it("a backup schedule writes its cron and its counts", async () => {
    panel({ backupStorage: rdbmsBackups });
    fireEvent.click(box("Back up on a schedule"));
    expect(field("Schedule (cron, UTC)").value).toBe("0 2 * * *");
    await userEvent.clear(field("Schedule (cron, UTC)"));
    await userEvent.type(field("Schedule (cron, UTC)"), "0 3 * * *");
    await userEvent.type(field("Completed backups to keep"), "7");
    await userEvent.type(field("Failed backups to keep"), "0");
    expect((await save()).attachments?.backupSchedule).toEqual({
      schedule: "0 3 * * *",
      retained: { completed: 7, failed: 0 },
    });
  });

  it("an empty count goes back to the default of the schedule", async () => {
    panel({
      backupStorage: rdbmsBackups,
      claim: claim("41", {
        attachments: {
          backupSchedule: { schedule: "0 2 * * *", retained: { completed: 7 } },
        },
      }),
    });
    await userEvent.clear(field("Completed backups to keep"));
    expect((await save()).attachments?.backupSchedule).toEqual({
      schedule: "0 2 * * *",
    });
  });

  it("a count that is not a whole number blocks Review", async () => {
    panel({ backupStorage: rdbmsBackups });
    fireEvent.click(box("Back up on a schedule"));
    await userEvent.type(field("Completed backups to keep"), "1.5");
    expect(
      within(bar()!).getByRole<HTMLButtonElement>("button", { name: "Review" })
        .disabled,
    ).toBe(true);
    expect(
      within(bar()!).getByText(
        "Completed backups to keep: Enter a whole number",
      ),
    ).toBeTruthy();
  });

  it("closing the panel clears the error of a count", async () => {
    panel({ backupStorage: rdbmsBackups });
    fireEvent.click(box("Back up on a schedule"));
    await userEvent.type(field("Completed backups to keep"), "1.5");
    fireEvent.click(
      within(attachments()).getByRole("button", { name: "Done" }),
    );
    expect(
      within(bar()!).getByRole<HTMLButtonElement>("button", { name: "Review" })
        .disabled,
    ).toBe(false);
  });

  it("Discard shows the counts of the claim again", async () => {
    panel({
      backupStorage: rdbmsBackups,
      claim: claim("41", {
        attachments: {
          backupSchedule: { schedule: "0 2 * * *", retained: { completed: 7 } },
        },
      }),
    });
    await userEvent.clear(field("Completed backups to keep"));
    await userEvent.type(field("Completed backups to keep"), "1.5");
    fireEvent.click(within(bar()!).getByRole("button", { name: "Discard" }));
    expect(field("Completed backups to keep").value).toBe("7");
    await userEvent.clear(field("Schedule (cron, UTC)"));
    await userEvent.type(field("Schedule (cron, UTC)"), "0 3 * * *");
    expect(
      within(bar()!).getByRole<HTMLButtonElement>("button", { name: "Review" })
        .disabled,
    ).toBe(false);
  });

  it("a completed count of 0 is an error, and a failed count of 0 is not", async () => {
    panel({ backupStorage: rdbmsBackups });
    fireEvent.click(box("Back up on a schedule"));
    await userEvent.type(field("Failed backups to keep"), "0");
    expect(
      within(bar()!).getByRole<HTMLButtonElement>("button", { name: "Review" })
        .disabled,
    ).toBe(false);
    await userEvent.type(field("Completed backups to keep"), "0");
    expect(
      within(bar()!).getByText("Completed backups to keep: Enter 1 or more"),
    ).toBeTruthy();
  });

  it("a refused write shows its message beside the field that the change set", async () => {
    refuse("spec.attachments.backupSchedule.schedule");
    panel({
      backupStorage: rdbmsBackups,
      claim: claim("41", {
        attachments: { backupSchedule: { schedule: "0 2 * * *" } },
      }),
    });
    await userEvent.clear(field("Schedule (cron, UTC)"));
    await userEvent.type(field("Schedule (cron, UTC)"), "every night");
    await saveAndGoBack();
    expect(field("Schedule (cron, UTC)").getAttribute("aria-invalid")).toBe(
      "true",
    );
    expect(
      within(attachments()).getByText("the schedule is not a cron expression"),
    ).toBeTruthy();
  });

  it("a refused write of a new attachment shows its message beside the toggle", async () => {
    refuse("spec.attachments.backupSchedule.schedule");
    panel({ backupStorage: rdbmsBackups });
    fireEvent.click(box("Back up on a schedule"));
    await saveAndGoBack();
    const note = within(attachments()).getByText(
      "the schedule is not a cron expression",
    );
    expect(note.closest("fieldset")?.querySelector("label")?.textContent).toBe(
      "Back up on a schedule",
    );
  });

  it("the backup schedule needs a backup storage option", () => {
    panel();
    expect(box("Back up on a schedule").disabled).toBe(true);
    expect(
      within(attachments()).getByText("Needs a backup storage option."),
    ).toBeTruthy();
  });
});
