// @vitest-environment jsdom
import type { V1Namespace } from "@kubernetes/client-node";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { Environment } from "@/generated/crds/environment";
import { useCreateFlow } from "@/components/create/use-create-flow";
import type { Plane } from "@/components/planes/plane";
import { ENVIRONMENT_LABEL, MANAGEMENT_CLUSTER_ANNOTATION } from "@/lib/labels";
import {
  EMPTY_PLANE_DRAFT,
  type PlaneContext,
  type PlaneDraft,
} from "@/lib/planes/draft";
import { ServedStep } from "./served-step";

afterEach(cleanup);

const environment = (name: string, displayName?: string): Environment =>
  ({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name },
    spec: { platformConfigRef: "basic", ...(displayName && { displayName }) },
  }) as Environment;

const namespace = (name: string, env: string): V1Namespace => ({
  metadata: { name, labels: { [ENVIRONMENT_LABEL]: env } },
});

const cluster = (
  ns: string,
  name: string,
  opts: {
    labels?: Record<string, string>;
    holder?: string;
    config?: string;
  } = {},
): CamundaCluster =>
  ({
    metadata: {
      namespace: ns,
      name,
      ...(opts.labels && { labels: opts.labels }),
      ...(opts.holder && {
        annotations: { [MANAGEMENT_CLUSTER_ANNOTATION]: opts.holder },
      }),
    },
    spec: { platformConfigRef: opts.config ?? "basic" },
    status: { gateway: { grpcEndpoint: "g:26500", restEndpoint: "http://g" } },
  }) as unknown as CamundaCluster;

const ctx: PlaneContext = {
  environments: [
    environment("production", "Production"),
    environment("staging"),
  ],
  platformConfigs: [
    { metadata: { name: "basic" }, spec: {} },
    {
      metadata: { name: "oidc" },
      spec: { auth: { method: "oidc", oidc: { issuerUrl: "https://idp" } } },
    },
  ] as unknown as CamundaPlatformConfig[],
  takenNames: [],
  namespaces: [],
};

const namespaces = [
  namespace("team-a", "production"),
  namespace("team-b", "staging"),
];

const plane = (name: string, servedEnvironments: string[]): Plane => ({
  id: `cluster/platform/${name}`,
  namespace: "platform",
  name,
  managed: true,
  components: ["Identity"],
  serves: { environments: servedEnvironments },
  servedEnvironments,
  attached: 0,
  notAttached: [],
  optimize: 0,
});

const draft: PlaneDraft = {
  ...EMPTY_PLANE_DRAFT,
  environment: "production",
  name: "plane-a",
  namespace: { kind: "existing", name: "platform" },
  mode: "externalKeycloak",
  externalKeycloak: { url: "https://idp", realm: "x", adminSecret: "a" },
};

function Harness({
  initial = draft,
  clusters = [
    cluster("team-a", "c1", { labels: { tier: "gold" } }),
    cluster("team-a", "c2", { holder: "other/plane-b" }),
    cluster("team-b", "c3"),
  ],
  planes = [],
  onSelectorErrors = vi.fn(),
}: {
  initial?: PlaneDraft;
  clusters?: CamundaCluster[];
  planes?: Plane[];
  onSelectorErrors?: (errors: unknown[]) => void;
}) {
  const flow = useCreateFlow<PlaneDraft>({
    steps: [{ id: "served", label: "Environments served" }],
    initial,
  });
  return (
    <ServedStep
      draft={flow.draft}
      set={flow.set}
      ctx={ctx}
      namespaces={namespaces}
      clusters={clusters}
      planes={planes}
      onSelectorErrors={onSelectorErrors}
    />
  );
}

const box = (name: RegExp) =>
  screen.getByRole("checkbox", { name }) as HTMLInputElement;
const previewRows = () =>
  within(screen.getByRole("region", { name: "Expected clusters" }))
    .getAllByRole("row")
    .slice(1);

describe("ServedStep", () => {
  it("offers every existing Environment by object name", () => {
    render(<Harness />);
    expect(box(/production/)).toBeTruthy();
    expect(box(/staging/)).toBeTruthy();
  });

  it("preselects the environment of step 1", () => {
    render(<Harness />);
    expect(box(/production/).checked).toBe(true);
    expect(box(/staging/).checked).toBe(false);
  });

  it("lists the clusters of the picked environments with one result each", () => {
    render(<Harness />);
    expect(previewRows().map((r) => r.textContent)).toEqual([
      expect.stringContaining("Will attach"),
      expect.stringContaining("Served by other/plane-b"),
    ]);
  });

  it("adds the clusters of another environment that the admin picks", async () => {
    render(<Harness />);
    await userEvent.click(box(/staging/));
    expect(previewRows().map((r) => r.textContent)).toEqual([
      expect.stringContaining("team-a/c1"),
      expect.stringContaining("team-a/c2"),
      expect.stringContaining("team-b/c3"),
    ]);
  });

  it("narrows the preview by the cluster labels", async () => {
    render(<Harness />);
    await userEvent.click(screen.getByRole("button", { name: "Add an entry" }));
    await userEvent.type(screen.getByRole("textbox", { name: "Key" }), "tier");
    await userEvent.type(
      screen.getByRole("textbox", { name: "Value" }),
      "gold",
    );
    expect(previewRows().map((r) => r.textContent)).toEqual([
      expect.stringContaining("team-a/c1"),
    ]);
  });

  it("reports the errors of the cluster labels", async () => {
    const onSelectorErrors = vi.fn();
    render(<Harness onSelectorErrors={onSelectorErrors} />);
    await userEvent.click(screen.getByRole("button", { name: "Add an entry" }));
    await userEvent.type(screen.getByRole("textbox", { name: "Value" }), "x");
    expect(onSelectorErrors).toHaveBeenLastCalledWith([
      { path: "0.key", message: "Enter a key" },
    ]);
  });

  it("reports the errors of the restored cluster labels when the step mounts again", () => {
    const onSelectorErrors = vi.fn();
    render(
      <Harness
        initial={{ ...draft, clusterLabels: { tier: "-gold" } }}
        onSelectorErrors={onSelectorErrors}
      />,
    );
    expect(onSelectorErrors).toHaveBeenLastCalledWith([
      expect.objectContaining({ path: "0.value" }),
    ]);
  });

  it("reports no errors for valid restored cluster labels", () => {
    const onSelectorErrors = vi.fn();
    render(
      <Harness
        initial={{ ...draft, clusterLabels: { tier: "gold" } }}
        onSelectorErrors={onSelectorErrors}
      />,
    );
    expect(onSelectorErrors).toHaveBeenLastCalledWith([]);
  });

  it("shows the new-issuer message on each OIDC cluster in Keycloak mode and not on a basic one", () => {
    render(
      <Harness
        initial={{ ...draft, mode: "keycloak" }}
        clusters={[
          cluster("team-a", "basic"),
          cluster("team-a", "oidc", { config: "oidc" }),
        ]}
      />,
    );
    const [basicRow, oidcRow] = previewRows();
    expect(
      within(oidcRow).getByText(
        "This cluster trusts another issuer. A Keycloak that this plane runs is a new issuer.",
      ),
    ).toBeTruthy();
    expect(within(basicRow).queryByText(/another issuer/)).toBeNull();
  });

  it("says that the preview shows expected results", () => {
    render(<Harness />);
    expect(screen.getByText(/^Expected results\./)).toBeTruthy();
  });

  it("says so when no cluster matches", () => {
    render(<Harness clusters={[]} />);
    expect(screen.getByText("No cluster matches yet.")).toBeTruthy();
  });

  it("names the planes that serve each environment now", () => {
    render(<Harness planes={[plane("plane-x", ["production"])]} />);
    const card = box(/production/).closest("label") as HTMLElement;
    expect(within(card).getByText("Served by platform/plane-x")).toBeTruthy();
    const other = box(/staging/).closest("label") as HTMLElement;
    expect(within(other).getByText("No plane serves it yet.")).toBeTruthy();
  });
});
