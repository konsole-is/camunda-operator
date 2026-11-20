// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { FieldLock, Step } from "@/lib/fields";
import {
  type ClusterAttachments,
  DEFAULT_SCHEDULE,
  MANAGEMENT_AUTH_CONFIGS_UNREAD,
  OPTIMIZE_NEEDS_ELASTICSEARCH,
  SCHEDULE_NEEDS_BACKUP_STORAGE,
} from "@/lib/clusters/attachments";
import { type AttachmentContext, AttachmentFields } from "./attachment-fields";

afterEach(cleanup);

const f = (field: string): Step => ({ field });
const OPTIMIZE: Step[] = [f("spec"), f("attachments"), f("optimize")];
const URL_STEPS: Step[] = [...OPTIMIZE, f("externalUrl")];
const SCHEDULE: Step[] = [f("spec"), f("attachments"), f("backupSchedule")];
const COMPLETED: Step[] = [...SCHEDULE, f("retained"), f("completed")];

const ACCESS_LOCK: FieldLock = {
  reason: "filled",
  section: "access",
  sectionLabel: "Access",
};

function context(over: Partial<AttachmentContext> = {}): AttachmentContext {
  return {
    secondaryStorage: "elasticsearch",
    backupStorage: true,
    managementAuthConfigs: ["a", "b"],
    lock: () => undefined,
    errors: [],
    pending: 0,
    ...over,
  };
}

function renderFields(
  value: ClusterAttachments | undefined,
  over: Partial<AttachmentContext> = {},
) {
  const handlers = { set: vi.fn(), reset: vi.fn(), invalid: vi.fn() };
  const result = render(
    <AttachmentFields value={value} context={context(over)} {...handlers} />,
  );
  return { ...handlers, ...result };
}

const box = (name: string) =>
  screen.getByRole<HTMLButtonElement>("checkbox", { name });
const descriptionOf = (element: HTMLElement) =>
  document.getElementById(element.getAttribute("aria-describedby") ?? "")
    ?.textContent;

describe("AttachmentFields", () => {
  it("turning Optimize on sets the first ManagementAuthConfig by name", async () => {
    const { set } = renderFields(undefined);
    await userEvent.click(box("Attach Optimize"));
    expect(set).toHaveBeenCalledWith(OPTIMIZE, {
      managementAuthConfigRef: "a",
    });
  });

  it("turning Optimize off resets the whole attachment", async () => {
    const { reset } = renderFields({
      optimize: { managementAuthConfigRef: "a" },
    });
    await userEvent.click(box("Attach Optimize"));
    expect(reset).toHaveBeenCalledWith(OPTIMIZE);
  });

  it("disables Optimize with the reason when it cannot attach", () => {
    renderFields(undefined, { secondaryStorage: "rdbms" });
    expect(box("Attach Optimize").disabled).toBe(true);
    expect(descriptionOf(box("Attach Optimize"))).toBe(
      OPTIMIZE_NEEDS_ELASTICSEARCH,
    );
  });

  it("says that the ManagementAuthConfigs could not be read, not that none exists", () => {
    renderFields(undefined, {
      managementAuthConfigs: [],
      unread: ["ManagementAuthConfig"],
    });
    expect(box("Attach Optimize").disabled).toBe(true);
    expect(descriptionOf(box("Attach Optimize"))).toBe(
      MANAGEMENT_AUTH_CONFIGS_UNREAD,
    );
  });

  it("keeps Optimize removable after the storage stops being Elasticsearch", () => {
    renderFields(
      { optimize: { managementAuthConfigRef: "a" } },
      { secondaryStorage: "rdbms" },
    );
    expect(box("Attach Optimize").disabled).toBe(false);
    expect(box("Attach Optimize").getAttribute("aria-checked")).toBe("true");
  });

  it("shows the external URL locked with the lock reason", () => {
    renderFields(
      { optimize: { managementAuthConfigRef: "a" } },
      {
        lock: (steps) =>
          JSON.stringify(steps) === JSON.stringify(URL_STEPS)
            ? ACCESS_LOCK
            : undefined,
        lockedUrl: "https://optimize.orders.example.com",
      },
    );
    const group = screen.getByRole("group", { name: "External URL" });
    expect(within(group).getByText("Filled by Access")).toBeTruthy();
    expect(
      within(group).getByText("https://optimize.orders.example.com"),
    ).toBeTruthy();
    expect(screen.queryByRole("textbox", { name: "External URL" })).toBeNull();
  });

  it("an emptied URL resets it", async () => {
    const { reset } = renderFields({
      optimize: {
        managementAuthConfigRef: "a",
        externalUrl: "https://optimize.example.com",
      },
    });
    await userEvent.clear(
      screen.getByRole("textbox", { name: "External URL" }),
    );
    expect(reset).toHaveBeenCalledWith(URL_STEPS);
  });

  it("turning the schedule on sets the default cron", async () => {
    const { set } = renderFields(undefined);
    await userEvent.click(box("Back up on a schedule"));
    expect(set).toHaveBeenCalledWith(SCHEDULE, { schedule: DEFAULT_SCHEDULE });
  });

  it("disables the schedule without backup storage", () => {
    renderFields(undefined, { backupStorage: false });
    expect(box("Back up on a schedule").disabled).toBe(true);
    expect(descriptionOf(box("Back up on a schedule"))).toBe(
      SCHEDULE_NEEDS_BACKUP_STORAGE,
    );
  });

  it("reports a count with an error and clears it when the field goes", async () => {
    const { set, invalid, rerender, reset } = renderFields({
      backupSchedule: { schedule: DEFAULT_SCHEDULE },
    });
    await userEvent.type(
      screen.getByRole("spinbutton", { name: "Completed backups to keep" }),
      "1.5",
    );
    expect(invalid).toHaveBeenLastCalledWith(
      COMPLETED,
      "Completed backups to keep: Enter a whole number",
    );
    expect(set).not.toHaveBeenCalledWith(COMPLETED, 1.5);
    invalid.mockClear();
    rerender(
      <AttachmentFields
        value={undefined}
        context={context()}
        set={set}
        reset={reset}
        invalid={invalid}
      />,
    );
    expect(invalid).toHaveBeenCalledWith(COMPLETED, undefined);
  });
});
