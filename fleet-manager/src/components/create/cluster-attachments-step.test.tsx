// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  OPTIMIZE_NEEDS_ELASTICSEARCH,
  SCHEDULE_NEEDS_BACKUP_STORAGE,
} from "@/lib/clusters/attachments";
import type { StorageSummary } from "@/lib/clusters/storage";
import type { FieldLock, Step } from "@/lib/fields";
import { ClusterAttachmentsStep } from "./cluster-attachments-step";

afterEach(cleanup);

const f = (field: string): Step => ({ field });
const OPTIMIZE: Step[] = [f("spec"), f("attachments"), f("optimize")];
const URL_STEPS: Step[] = [...OPTIMIZE, f("externalUrl")];

const ACCESS_LOCK: FieldLock = {
  reason: "filled",
  section: "access",
  sectionLabel: "Access",
};

const summary = (over: Partial<StorageSummary> = {}): StorageSummary => ({
  secondaryStorageType: "elasticsearch",
  backupStorage: { section: "backup-storage", registration: "bucket" },
  optimizeUrl: false,
  slots: [],
  picks: [],
  ...over,
});

const urlLock = (steps: Step[]) =>
  JSON.stringify(steps) === JSON.stringify(URL_STEPS) ? ACCESS_LOCK : undefined;

function renderStep(
  over: Partial<ComponentProps<typeof ClusterAttachmentsStep>> = {},
) {
  const props: ComponentProps<typeof ClusterAttachmentsStep> = {
    value: undefined,
    summary: summary(),
    managementAuthConfigs: ["central"],
    lock: () => undefined,
    set: vi.fn(),
    reset: vi.fn(),
    invalid: vi.fn(),
    ...over,
  };
  render(<ClusterAttachmentsStep {...props} />);
  return props;
}

const box = (name: string) =>
  screen.getByRole<HTMLButtonElement>("checkbox", { name });
const descriptionOf = (element: HTMLElement) =>
  document.getElementById(element.getAttribute("aria-describedby") ?? "")
    ?.textContent;

describe("ClusterAttachmentsStep", () => {
  it("locks the Optimize URL when the access option gives one", () => {
    renderStep({
      value: { optimize: { managementAuthConfigRef: "central" } },
      summary: summary({ optimizeUrl: true }),
      lock: urlLock,
    });
    const group = screen.getByRole("group", { name: "External URL" });
    expect(within(group).getByText("Filled by Access")).toBeTruthy();
    expect(
      within(group).getByText("Known once the access option is ready"),
    ).toBeTruthy();
    expect(screen.queryByRole("textbox", { name: "External URL" })).toBeNull();
  });

  it("says that nothing fills a locked URL when the access option gives none", () => {
    renderStep({
      value: { optimize: { managementAuthConfigRef: "central" } },
      lock: urlLock,
    });
    const group = screen.getByRole("group", { name: "External URL" });
    expect(
      within(group).getByText("None: the access option gives no Optimize URL"),
    ).toBeTruthy();
  });

  it("asks for the Optimize URL without an access option", async () => {
    const set = vi.fn();
    renderStep({
      value: { optimize: { managementAuthConfigRef: "central" } },
      set,
    });
    await userEvent.type(
      screen.getByRole("textbox", { name: "External URL" }),
      "h",
    );
    expect(set).toHaveBeenCalledWith(URL_STEPS, "h");
  });

  it("refuses Optimize with PostgreSQL secondary storage", () => {
    renderStep({ summary: summary({ secondaryStorageType: "rdbms" }) });
    expect(box("Attach Optimize").disabled).toBe(true);
    expect(descriptionOf(box("Attach Optimize"))).toBe(
      OPTIMIZE_NEEDS_ELASTICSEARCH,
    );
  });

  it("refuses the schedule without backup storage", () => {
    renderStep({ summary: summary({ backupStorage: undefined }) });
    expect(box("Back up on a schedule").disabled).toBe(true);
    expect(descriptionOf(box("Back up on a schedule"))).toBe(
      SCHEDULE_NEEDS_BACKUP_STORAGE,
    );
  });

  it("keeps Optimize removable after the storage stops being Elasticsearch", async () => {
    const reset = vi.fn();
    renderStep({
      value: { optimize: { managementAuthConfigRef: "central" } },
      summary: summary({ secondaryStorageType: "rdbms" }),
      reset,
    });
    expect(box("Attach Optimize").disabled).toBe(false);
    await userEvent.click(box("Attach Optimize"));
    expect(reset).toHaveBeenCalledWith(OPTIMIZE);
  });
});
