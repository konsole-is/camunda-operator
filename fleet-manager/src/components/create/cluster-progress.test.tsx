// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { ClusterClaimObject } from "@/lib/clusters/draft";
import { ClusterProgress, ClusterProgressView } from "./cluster-progress";

const read = vi.hoisted(() => ({
  claim: undefined as unknown,
  claimError: undefined as string | undefined,
  requests: [] as unknown[],
  requestsError: undefined as string | undefined,
  asked: [] as { kind: string; opts: unknown }[],
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResource: (kind: string, opts: unknown) => {
    read.asked.push({ kind, opts });
    return { item: read.claim, error: read.claimError };
  },
  useResources: (kind: string, opts: unknown) => {
    read.asked.push({ kind, opts });
    return { items: read.requests, error: read.requestsError };
  },
}));

afterEach(() => {
  cleanup();
  read.claim = undefined;
  read.claimError = undefined;
  read.requests = [];
  read.requestsError = undefined;
  read.asked = [];
});

const sections = [
  {
    metadata: { name: "encryption-key" },
    spec: { label: "Encryption key", selection: "One" },
  },
] as PluginSection[];
const registrations = [
  { metadata: { name: "kms" }, spec: { label: "KMS key" } },
] as PluginRegistration[];

const condition = (
  type: string,
  status: "True" | "False",
  reason: string,
  message: string,
) => ({
  type,
  status,
  reason,
  message,
  observedGeneration: 1,
  lastTransitionTime: "2026-11-16T00:00:00Z",
});

const claimWith = (
  conditions: ReturnType<typeof condition>[],
  cluster?: { name: string },
) =>
  ({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { namespace: "team-a", name: "orders", generation: 1 },
    spec: {
      presetRef: "small",
      plugins: [{ section: "encryption-key", registration: "kms" }],
    },
    status: { conditions, ...(cluster && { cluster }) },
  }) as unknown as CamundaClusterClaim;

const view = (claim?: CamundaClusterClaim) =>
  render(
    <ClusterProgressView
      name="orders"
      namespace="team-a"
      claim={claim}
      sections={sections}
      registrations={registrations}
      requests={[]}
    />,
  );

const rowOf = (label: string) =>
  screen.getByText(label, { exact: true }).closest("li")!;

describe("ClusterProgressView", () => {
  it("shows a claim that waits for outputs with the waiting sections and RequestsReady False", () => {
    view(
      claimWith([
        condition(
          "Ready",
          "False",
          "PrerequisiteNotMet",
          "waiting for the outputs of encryption-key",
        ),
        condition(
          "RequestsReady",
          "False",
          "Blocked",
          "waiting for the outputs of encryption-key",
        ),
      ]),
    );
    expect(
      screen.getByRole("heading", { name: "Creating the cluster orders" }),
    ).toBeTruthy();
    expect(
      screen.getAllByText("waiting for the outputs of encryption-key").length,
    ).toBe(2);
    expect(
      within(rowOf("Requests"))
        .getByRole("img", { name: /^Ready:/ })
        .getAttribute("aria-label"),
    ).toBe("Ready: Not ready (Blocked)");
  });

  it("shows a precheck refusal without a Claim tab link while no cluster exists", () => {
    view(
      claimWith([
        condition(
          "Ready",
          "False",
          "WiringCycle",
          "The sections wait for each other.",
        ),
      ]),
    );
    expect(screen.getByRole("status").textContent).toContain(
      "The sections wait for each other.",
    );
    expect(
      screen.queryByRole("link", { name: "Open the Claim tab" }),
    ).toBeNull();
  });

  it("links the Claim tab for a precheck refusal once the cluster exists", () => {
    view(
      claimWith(
        [
          condition(
            "Ready",
            "False",
            "WiringCycle",
            "The sections wait for each other.",
          ),
        ],
        { name: "orders" },
      ),
    );
    expect(
      screen
        .getByRole("link", { name: "Open the Claim tab" })
        .getAttribute("href"),
    ).toBe("/fleet/clusters/team-a/orders?tab=claim");
  });

  it("links the cluster page once the cluster is ready", () => {
    view(
      claimWith(
        [
          condition("Ready", "True", "Ready", ""),
          condition(
            "ClusterReady",
            "True",
            "Ready",
            "Ready: The cluster is ready",
          ),
        ],
        { name: "orders" },
      ),
    );
    expect(
      screen
        .getByRole("link", { name: "Open the cluster page" })
        .getAttribute("href"),
    ).toBe("/fleet/clusters/team-a/orders");
  });

  it("shows no cluster link while the cluster is not ready", () => {
    view(
      claimWith(
        [
          condition(
            "ClusterReady",
            "False",
            "StorageAlreadyAttached",
            "StorageAlreadyAttached: the backend is held by another cluster",
          ),
        ],
        { name: "orders" },
      ),
    );
    expect(
      screen.queryByRole("link", { name: "Open the cluster page" }),
    ).toBeNull();
    expect(
      within(rowOf("Cluster")).getByText(
        "StorageAlreadyAttached: the backend is held by another cluster",
      ),
    ).toBeTruthy();
  });

  it("encodes the namespace and the name in the links", () => {
    render(
      <ClusterProgressView
        name="a b"
        namespace="x/y"
        claim={claimWith([condition("ClusterReady", "True", "Ready", "")], {
          name: "a b",
        })}
        sections={sections}
        registrations={registrations}
        requests={[]}
      />,
    );
    expect(
      screen
        .getByRole("link", { name: "Open the cluster page" })
        .getAttribute("href"),
    ).toBe("/fleet/clusters/x%2Fy/a%20b");
  });
});

const written: ClusterClaimObject = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  metadata: { namespace: "team-a", name: "orders" },
  spec: {
    presetRef: "small",
    plugins: [{ section: "encryption-key", registration: "kms" }],
  },
};

describe("ClusterProgress", () => {
  it("follows the claim and the requests that carry its claim label", () => {
    render(
      <ClusterProgress
        claim={written}
        sections={sections}
        registrations={registrations}
      />,
    );
    expect(read.asked).toEqual(
      expect.arrayContaining([
        {
          kind: "CamundaClusterClaim",
          opts: { namespace: "team-a", name: "orders" },
        },
        {
          kind: "PluginRequest",
          opts: {
            namespace: "team-a",
            labelSelector: "fleet.camunda.io/claim=orders",
          },
        },
      ]),
    );
  });

  it("shows the written claim with each row pending until the live claim exists", () => {
    render(
      <ClusterProgress
        claim={written}
        sections={sections}
        registrations={registrations}
      />,
    );
    expect(
      within(rowOf("Encryption key"))
        .getAllByRole("img")
        .map((dot) => dot.getAttribute("aria-label")),
    ).toEqual(["Outputs: No status", "Ready: No status"]);
  });

  it("shows the live claim and its requests once the watch sends them", () => {
    const { rerender } = render(
      <ClusterProgress
        claim={written}
        sections={sections}
        registrations={registrations}
      />,
    );
    read.claim = {
      ...written,
      metadata: { ...written.metadata, generation: 1 },
      status: {
        requests: [
          {
            section: "encryption-key",
            registration: "kms",
            name: "orders-encryption-key",
            outputsReady: "True",
            ready: "True",
          },
        ],
      },
    };
    read.requests = [
      {
        metadata: { namespace: "team-a", name: "orders-encryption-key" },
        status: { summary: [{ label: "Key", value: "alias/orders" }] },
      },
    ];
    rerender(
      <ClusterProgress
        claim={written}
        sections={sections}
        registrations={registrations}
      />,
    );
    const row = rowOf("Encryption key");
    expect(row.textContent).toContain("KMS key, orders-encryption-key");
    expect(row.textContent).toContain("Key: alias/orders");
  });

  it("names each read that failed above the rows", () => {
    read.claimError = "camundaclusterclaims is forbidden";
    read.requestsError = "pluginrequests is forbidden";
    render(
      <ClusterProgress
        claim={written}
        sections={sections}
        registrations={registrations}
      />,
    );
    expect(screen.getAllByRole("alert").map((a) => a.textContent)).toEqual([
      "Could not read CamundaClusterClaim: camundaclusterclaims is forbidden",
      "Could not read PluginRequest: pluginrequests is forbidden",
    ]);
  });
});
