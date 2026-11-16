// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import catalog from "../../../../catalog/official-plugins.json";
import type { CatalogChart } from "@/startup/bundled-data";
import {
  condition,
  pluginRegistration,
  pluginRequest,
  registrationSources,
} from "./fixtures";
import { registrationGroups, registrationSummary } from "./groups";
import { RegistrationList, RegistrationListLive } from "./registration-list";

const watch = vi.hoisted(() => ({ unanswered: [] as string[] }));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (_kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
  }),
  useWatchesFresh: (lists: { kind: string }[]) =>
    lists.map(({ kind }) => !watch.unanswered.includes(kind)),
}));

afterEach(() => {
  cleanup();
  watch.unanswered = [];
});

const officialCharts = catalog.charts as CatalogChart[];
const sources = registrationSources();
const list = (over: Partial<Parameters<typeof RegistrationList>[0]> = {}) =>
  render(
    <RegistrationList
      groups={registrationGroups(sources)}
      summary={registrationSummary(sources.registrations)}
      charts={[]}
      sections={sources.sections}
      errors={[]}
      now="2026-11-13T09:00:00Z"
      {...over}
    />,
  );

const rowOf = (region: string, name: string) =>
  within(screen.getByRole("region", { name: region }))
    .getAllByRole("row")
    .find((r) => r.textContent?.includes(name))!;

describe("RegistrationList", () => {
  it("shows the groups in section order with what each fills", () => {
    list();
    expect(
      screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent),
    ).toEqual([
      "Cloud identity",
      "Encryption key",
      "Volume encryption",
      "Backup storage",
      "Secondary storage",
      "Access",
      "Connectors permissions",
      "Databases",
    ]);
    const backup = screen.getByRole("region", { name: "Backup storage" });
    expect(
      within(backup).getByText("Fills CamundaCluster spec.backupStorageRef"),
    ).toBeTruthy();
    expect(within(backup).getByText("Pick one")).toBeTruthy();
    expect(within(backup).getByText("Empty option: No backups")).toBeTruthy();
  });

  it("says when a section fills no field, has no empty option, or no option", () => {
    list();
    const key = screen.getByRole("region", { name: "Encryption key" });
    expect(within(key).getByText("Fills no field")).toBeTruthy();
    expect(
      within(key).getByText("No option is installed for this section."),
    ).toBeTruthy();
    const storage = screen.getByRole("region", { name: "Secondary storage" });
    expect(within(storage).getByText("No empty option")).toBeTruthy();
  });

  it("marks the fields that a management plane fills", () => {
    list();
    const databases = screen.getByRole("region", { name: "Databases" });
    expect(
      within(databases).getByText(
        "Fills CamundaManagementCluster spec.identity.databaseConfigRef for a management plane",
      ),
    ).toBeTruthy();
  });

  it("shows the controller, the version, and how long a row is healthy", () => {
    list();
    const row = rowOf("Secondary storage", "existing-secondary-storage");
    expect(within(row).getByText("camunda-operator 0.0.0-dev")).toBeTruthy();
    expect(within(row).getByText(/Healthy since 3d ago/)).toBeTruthy();
    expect(
      within(row)
        .getByText(/Healthy since/)
        .closest("time")
        ?.getAttribute("dateTime"),
    ).toBe("2026-11-10T09:00:00Z");
    expect(row.textContent).not.toMatch(/checked/i);
  });

  it("shows the mode, the requests by environment, the operator versions, and Orphan", () => {
    list();
    const row = rowOf("Secondary storage", "existing-secondary-storage");
    expect(within(row).getByText("Optional")).toBeTruthy();
    expect(within(row).getByText("Production 1")).toBeTruthy();
    expect(within(row).getByText("Any")).toBeTruthy();
    expect(within(row).getByText("Supported")).toBeTruthy();
  });

  it("shows the reason of a refused registration and No status for a new one", () => {
    const refused = pluginRegistration(
      "lost",
      {
        label: "Lost",
        sections: ["gone"],
        operatorVersions: ">=0.3.0",
        deletionPolicies: ["Delete"],
      },
      [condition("Accepted", "False", "SectionMissing")],
    );
    const fresh = pluginRegistration("fresh", { sections: ["access"] });
    const withBoth = {
      ...sources,
      registrations: [...sources.registrations, refused, fresh],
    };
    list({ groups: registrationGroups(withBoth) });
    const row = rowOf("Not in a section", "lost");
    expect(
      within(row).getByText("Accepted: False · SectionMissing"),
    ).toBeTruthy();
    expect(within(row).getByText("Ready: No status")).toBeTruthy();
    expect(within(row).getByText(">=0.3.0")).toBeTruthy();
    expect(within(row).getByText("Not supported")).toBeTruthy();
    expect(within(row).queryByText(/Healthy since/)).toBeNull();
    const freshRow = rowOf("Access", "fresh");
    expect(within(freshRow).getByText("Accepted: No status")).toBeTruthy();
  });

  it("shows no request count by environment for a registration without requests", () => {
    list({
      groups: registrationGroups({
        ...sources,
        requests: [
          pluginRequest("team-orders", "x", {
            registration: "elasticsearch",
          }),
        ],
      }),
    });
    const row = rowOf("Secondary storage", "existing-secondary-storage");
    expect(within(row).queryByText(/Production/)).toBeNull();
  });

  it("counts healthy and not healthy registrations", () => {
    list({ summary: { healthy: 6, notHealthy: 1 } });
    expect(screen.getByText("6 healthy, 1 not healthy")).toBeTruthy();
  });

  it("shows each read error and keeps the groups", () => {
    list({
      errors: [{ kind: "PluginRequest", message: "forbidden" }],
    });
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read PluginRequest: forbidden",
    );
    expect(screen.getByRole("region", { name: "Access" })).toBeTruthy();
  });

  it("names each list that is not read", () => {
    list({
      errors: [{ kind: "PluginRequest", message: "forbidden" }],
      stale: ["Namespace"],
    });
    expect(
      screen.getByText(
        "Not read: PluginRequest, Namespace. What needs them shows as not known.",
      ),
    ).toBeTruthy();
  });

  it("shows no health count and no chart as not installed while PluginRegistration is not read", () => {
    const unread = { ...sources, stale: ["PluginRegistration"] };
    list({
      groups: registrationGroups(unread),
      stale: unread.stale,
      charts: officialCharts,
    });
    expect(screen.getByText("Health not known")).toBeTruthy();
    expect(screen.queryByText(/\d+ healthy/)).toBeNull();
    const panel = screen.getByRole("region", {
      name: "Charts you can install",
    });
    expect(
      within(panel).getByText("Which charts are installed is not known."),
    ).toBeTruthy();
    expect(within(panel).queryByText("rustfs", { exact: true })).toBeNull();
  });

  it("says that the options of a section are not known while PluginRegistration is not read", () => {
    const unread = {
      ...sources,
      registrations: [],
      stale: ["PluginRegistration"],
    };
    list({ groups: registrationGroups(unread), stale: unread.stale });
    const key = screen.getByRole("region", { name: "Encryption key" });
    expect(within(key).getByText("Options not known")).toBeTruthy();
    expect(
      screen.queryByText("No option is installed for this section."),
    ).toBeNull();
  });

  it("shows the requests as not known while PluginRequest is not read", () => {
    const unread = { ...sources, requests: [], stale: ["PluginRequest"] };
    list({ groups: registrationGroups(unread), stale: unread.stale });
    const row = rowOf("Secondary storage", "existing-secondary-storage");
    expect(within(row).getByText("Not known")).toBeTruthy();
  });

  it("shows the total but not the environments of the requests while Environment is not read", () => {
    const unread = { ...sources, stale: ["Environment"] };
    list({ groups: registrationGroups(unread), stale: unread.stale });
    const row = rowOf("Secondary storage", "existing-secondary-storage");
    expect(within(row).getByText("1")).toBeTruthy();
    expect(within(row).getByText("Environments not known")).toBeTruthy();
    expect(within(row).queryByText(/Production/)).toBeNull();
  });

  it("puts the registrations under Section not known while PluginSection is not read", () => {
    const unread = { ...sources, sections: [], stale: ["PluginSection"] };
    list({
      groups: registrationGroups(unread),
      sections: [],
      stale: unread.stale,
    });
    expect(
      screen.getByRole("region", { name: "Section not known" }),
    ).toBeTruthy();
    expect(
      screen.queryByRole("region", { name: "Not in a section" }),
    ).toBeNull();
  });

  it("says that the catalog could not be read", () => {
    list({ catalogError: "reading catalog /catalog.json: ENOENT" });
    expect(
      within(
        screen.getByRole("region", { name: "Charts you can install" }),
      ).getByRole("alert").textContent,
    ).toBe(
      "Could not read the chart catalog: reading catalog /catalog.json: ENOENT",
    );
  });

  it("hides the catalog panel when every chart is installed or the catalog is empty", () => {
    list({ charts: [] });
    expect(
      screen.queryByRole("region", { name: "Charts you can install" }),
    ).toBeNull();
  });

  it("says plugin in no label but the page title", () => {
    list({ charts: officialCharts });
    const labels = [
      ...screen.getAllByRole("heading"),
      ...screen.getAllByRole("columnheader"),
      ...screen.getAllByRole("button"),
    ]
      .filter((e) => !(e.tagName === "H1"))
      .map((e) => e.textContent ?? "");
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe(
      "Plugins",
    );
    expect(labels.filter((l) => /plugin/i.test(l))).toEqual([]);
  });
});

describe("RegistrationListLive", () => {
  it("lists the charts that no registration names as its controller", () => {
    render(
      <RegistrationListLive
        initial={{
          ...sources,
          registrations: [
            ...sources.registrations,
            pluginRegistration("aws-kms", {
              controller: { name: "aws-kms", version: "0.1.0" },
              sections: ["encryption-key"],
            }),
          ],
        }}
        catalog={{ charts: officialCharts }}
        now="2026-11-13T09:00:00Z"
      />,
    );
    const panel = screen.getByRole("region", {
      name: "Charts you can install",
    });
    expect(within(panel).queryByText("aws-kms", { exact: true })).toBeNull();
    expect(within(panel).getByText("rustfs", { exact: true })).toBeTruthy();
  });

  it("shows nothing as empty, healthy, or not installed while a watch has not answered", () => {
    watch.unanswered = ["PluginRegistration"];
    render(
      <RegistrationListLive
        initial={{ ...sources, registrations: [] }}
        catalog={{ charts: officialCharts }}
        now="2026-11-13T09:00:00Z"
      />,
    );
    expect(
      screen.getByText(
        "Not read: PluginRegistration. What needs them shows as not known.",
      ),
    ).toBeTruthy();
    expect(
      screen.queryByText("No option is installed for this section."),
    ).toBeNull();
    expect(screen.getAllByText("Options not known")).toHaveLength(8);
    expect(screen.getByText("Health not known")).toBeTruthy();
    expect(
      within(
        screen.getByRole("region", { name: "Charts you can install" }),
      ).queryByText("rustfs", { exact: true }),
    ).toBeNull();
  });

  it("says that the catalog could not be read", () => {
    render(
      <RegistrationListLive
        initial={sources}
        catalog={{ error: "reading catalog /catalog.json: ENOENT" }}
        now="2026-11-13T09:00:00Z"
      />,
    );
    expect(
      within(
        screen.getByRole("region", { name: "Charts you can install" }),
      ).getByRole("alert").textContent,
    ).toBe(
      "Could not read the chart catalog: reading catalog /catalog.json: ENOENT",
    );
  });
});
