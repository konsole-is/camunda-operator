// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { detailData } from "./fixtures";
import { RegistrationDetail } from "./registration-detail";
import type { RegistrationTab } from "./registration-tabs";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const NOW = "2026-11-13T09:00:00Z";
const detail = (
  over: Partial<Parameters<typeof RegistrationDetail>[0]> = {},
  tab: RegistrationTab = "about",
) =>
  render(
    <RegistrationDetail
      data={detailData("elasticsearch")}
      errors={[]}
      stale={[]}
      tab={tab}
      now={NOW}
      {...over}
    />,
  );

describe("RegistrationDetail", () => {
  it("names the registration by its label, with its name under it", () => {
    detail();
    expect(
      screen.getByRole("heading", {
        level: 1,
        name: "New Elasticsearch cluster",
      }),
    ).toBeTruthy();
    expect(screen.getByText("elasticsearch", { selector: "p" })).toBeTruthy();
  });

  it("opens on the tab it is given", () => {
    detail({}, "health");
    expect(
      screen.getByRole("tab", { name: "Health" }).getAttribute("aria-selected"),
    ).toBe("true");
    expect(screen.getByRole("region", { name: "Events" })).toBeTruthy();
  });

  it("sets the tab query parameter when a tab is chosen", () => {
    const replace = vi.spyOn(window.history, "replaceState");
    detail();
    const tab = screen.getByRole("tab", { name: "Health" });
    fireEvent.mouseDown(tab);
    fireEvent.click(tab);
    expect(replace).toHaveBeenLastCalledWith(null, "", "?tab=health");
  });

  it("says when the registration no longer exists", () => {
    const data = detailData("elasticsearch");
    detail({
      data: { ...data, sources: { ...data.sources, registrations: [] } },
    });
    expect(
      screen.getByText("This registration no longer exists."),
    ).toBeTruthy();
    expect(screen.queryByRole("tablist")).toBeNull();
  });

  it("keeps the registration that the server read while PluginRegistration is not read", () => {
    const data = detailData("elasticsearch");
    detail({
      data: { ...data, sources: { ...data.sources, registrations: [] } },
      stale: ["PluginRegistration"],
    });
    expect(
      screen.queryByText("This registration no longer exists."),
    ).toBeNull();
    expect(screen.getByRole("tablist")).toBeTruthy();
  });

  it("shows each read error and names each list that is not read", () => {
    detail({
      errors: [{ kind: "Grant", message: "forbidden" }],
      stale: ["Event"],
    });
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read Grant: forbidden",
    );
    expect(
      screen.getByText(
        "Not read: Grant, Event. What needs them shows as not known.",
      ),
    ).toBeTruthy();
  });

  it("shows the label of the live registration", () => {
    const data = detailData("elasticsearch");
    const live = data.sources.registrations.map((r) =>
      r.metadata.name === "elasticsearch"
        ? { ...r, spec: { ...r.spec, label: "Elasticsearch cluster" } }
        : r,
    );
    detail({
      data: { ...data, sources: { ...data.sources, registrations: live } },
    });
    expect(
      within(document.body).getByRole("heading", {
        level: 1,
        name: "Elasticsearch cluster",
      }),
    ).toBeTruthy();
  });
});
