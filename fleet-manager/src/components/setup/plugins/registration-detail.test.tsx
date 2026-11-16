// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  condition,
  detailData,
  fleetEnvironment,
  pluginGrant,
  pluginRegistration,
  pluginRequest,
  registrationEvent,
  registrationSources,
} from "./fixtures";
import { RegistrationDetail } from "./registration-detail";
import { REGISTRATION_TABS, type RegistrationTab } from "./registration-tabs";

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

  it.each(["about", "contract", "parameters", "grants"] as const)(
    "says on the %s tab that the registration can be old while PluginRegistration is not read",
    (tab) => {
      detail({ stale: ["PluginRegistration"] }, tab);
      expect(
        screen.getByText(
          "This is the registration as the page last read it. It can be old.",
        ),
      ).toBeTruthy();
      expect(
        screen.getByText(
          "Not read: PluginRegistration. What needs them shows as not known, and the registration as the page last read it.",
        ),
      ).toBeTruthy();
    },
  );

  it("does not say that the registration can be old while it is read", () => {
    detail();
    expect(screen.queryByText(/It can be old\./)).toBeNull();
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

// fullData is a registration with every part that a tab can show.
function fullData() {
  const registration = pluginRegistration(
    "aws-s3",
    {
      label: "New S3 bucket",
      description: "Makes a bucket for the backups of the cluster.",
      docsUrl: "https://example.com/docs/aws-s3",
      sections: ["backup-storage"],
      controller: { name: "aws-s3", version: "0.4.1" },
      limits: [{ label: "Regions", value: "eu-west-1" }],
      inputs: [
        { name: "principal", from: "cloud-identity/roleArn", required: true },
      ],
      outputs: [{ key: "objectStorageConfig" }],
      parameters: {
        type: "object",
        properties: { versioning: { type: "boolean" } },
      },
      parameterValues: { versioning: true },
      grantTemplates: [
        {
          name: "bucket-access",
          format: "aws-iam-statement",
          template: { Effect: "Allow", Resource: "{{ .outputs.bucket }}" },
          requiresInputs: ["principal"],
        },
      ],
    },
    [
      condition("Accepted", "True", "Healthy"),
      condition("Ready", "True", "Healthy"),
    ],
  );
  const labels = {
    "plugins.camunda.io/registration": "aws-s3",
    "plugins.camunda.io/request": "orders-backup-storage",
  };
  const sources = registrationSources({
    requests: [
      pluginRequest(
        "team-orders",
        "orders-backup-storage",
        { registration: "aws-s3" },
        {
          conditions: [condition("Ready", "False", "GrantRejected")] as never,
          summary: [{ label: "Bucket", value: "cpo-orders" }],
          resources: [
            { kind: "S3Bucket", name: "cpo-orders", onOrphan: "Kept" },
          ],
        },
      ),
    ],
    environments: [
      fleetEnvironment("production", { displayName: "Production" }),
    ],
  });
  return {
    name: "aws-s3",
    registration,
    sources: {
      ...sources,
      registrations: [...sources.registrations, registration],
    },
    grants: [
      pluginGrant(
        "team-orders",
        "orders-bucket-access",
        { template: "bucket-access" },
        labels,
        [condition("Ready", "False", "OverCeiling")],
      ),
    ],
    events: [registrationEvent("aws-s3", "Normal", "Accepted", "Accepted")],
  };
}

describe("RegistrationDetail words", () => {
  it.each(REGISTRATION_TABS.map((t) => t.value))(
    "says plugin in no label of the tab %s",
    (tab) => {
      render(
        <RegistrationDetail
          data={fullData()}
          errors={[]}
          stale={[]}
          tab={tab}
          now={NOW}
        />,
      );
      const labels = [
        ...screen.queryAllByRole("heading").filter((h) => h.tagName !== "H1"),
        ...screen.queryAllByRole("columnheader"),
        ...screen.queryAllByRole("rowheader"),
        ...document.querySelectorAll("dt"),
        ...screen.queryAllByRole("tab"),
        ...screen.queryAllByRole("button"),
      ].map((e) => e.textContent ?? "");
      expect(labels.length).toBeGreaterThan(5);
      expect(labels.filter((l) => /plugin/i.test(l))).toEqual([]);
    },
  );
});
