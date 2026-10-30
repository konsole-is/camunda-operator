// @vitest-environment jsdom
import type { V1Namespace } from "@kubernetes/client-node";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { Environment } from "@/generated/crds/environment";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import {
  EMPTY_PLANE_DRAFT,
  type PlaneContext,
  type PlaneDraft,
} from "@/lib/planes/draft";
import { useCreateFlow } from "@/components/create/use-create-flow";
import { BasicsStep } from "./basics-step";

afterEach(cleanup);

const environment = (
  name: string,
  spec: Partial<Environment["spec"]> = {},
): Environment =>
  ({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name },
    spec,
  }) as Environment;

const namespace = (name: string, env?: string): V1Namespace => ({
  metadata: { name, ...(env && { labels: { [ENVIRONMENT_LABEL]: env } }) },
});

const ctx: PlaneContext = {
  environments: [
    environment("production", {
      displayName: "Production",
      platformConfigRef: "default-platform",
    }),
    environment("staging", { platformConfigRef: "default-platform" }),
  ],
  platformConfigs: [
    { metadata: { name: "default-platform" }, spec: {} },
    {
      metadata: { name: "oidc-platform" },
      spec: { auth: { method: "oidc", oidc: { issuerUrl: "https://idp" } } },
    },
  ] as unknown as CamundaPlatformConfig[],
  takenNames: [],
};

const namespaces = [
  namespace("platform", "production"),
  namespace("team-a", "production"),
  namespace("team-b", "staging"),
];

function Harness({
  initial,
  context = ctx,
}: {
  initial: PlaneDraft;
  context?: PlaneContext;
}) {
  const flow = useCreateFlow<PlaneDraft>({
    steps: [{ id: "basics", label: "Basics" }],
    initial,
  });
  return (
    <>
      <BasicsStep
        draft={flow.draft}
        set={flow.set}
        ctx={context}
        namespaces={namespaces}
        clusters={[]}
        secrets={[]}
      />
      <output data-testid="draft">{JSON.stringify(flow.draft)}</output>
    </>
  );
}

const draftOf = () =>
  JSON.parse(screen.getByTestId("draft").textContent ?? "{}") as PlaneDraft;

const inProduction: PlaneDraft = {
  ...EMPTY_PLANE_DRAFT,
  environment: "production",
  namespace: { kind: "existing", name: "platform" },
};

const platformConfig = () =>
  screen.getByRole("combobox", {
    name: "Platform config",
  }) as HTMLSelectElement;

describe("BasicsStep", () => {
  it("names the chosen environment in the policy line", () => {
    render(<Harness initial={inProduction} />);
    expect(
      screen.getByText(
        "Policy from the Production environment (namespace platform).",
      ),
    ).toBeTruthy();
  });

  it("leaves the namespace out of the policy line until one is chosen", () => {
    render(<Harness initial={{ ...inProduction, namespace: undefined }} />);
    expect(
      screen.getByText("Policy from the Production environment."),
    ).toBeTruthy();
  });

  it("prefills the platform config from the environment", () => {
    render(<Harness initial={inProduction} />);
    expect(platformConfig().value).toBe("default-platform");
    expect(
      screen.getByText("Default from the Production environment."),
    ).toBeTruthy();
  });

  it("keeps the pick of another platform config in the draft", async () => {
    render(<Harness initial={inProduction} />);
    await userEvent.selectOptions(platformConfig(), "oidc-platform");
    expect(draftOf().platformConfig).toBe("oidc-platform");
    expect(screen.queryByText(/^Default from/)).toBeNull();
  });

  it("offers the namespaces of the chosen environment only", () => {
    render(<Harness initial={{ ...inProduction, namespace: undefined }} />);
    const select = screen.getByRole("combobox", { name: /namespace/i });
    const options = within(select)
      .getAllByRole("option")
      .map((o) => o.textContent);
    expect(options).toContain("team-a");
    expect(options).not.toContain("team-b");
  });

  it("drops the namespace when the environment changes", async () => {
    render(<Harness initial={inProduction} />);
    await userEvent.click(screen.getByRole("radio", { name: /staging/ }));
    expect(draftOf()).toMatchObject({ environment: "staging" });
    expect(draftOf().namespace).toBeUndefined();
  });

  it("says that a plane needs an Environment, and shows no fields, when none exists", () => {
    render(
      <Harness
        initial={EMPTY_PLANE_DRAFT}
        context={{ ...ctx, environments: [] }}
      />,
    );
    expect(screen.getByText(/No Environment exists/)).toBeTruthy();
    expect(
      screen.queryByRole("combobox", { name: "Platform config" }),
    ).toBeNull();
    expect(screen.queryByLabelText("Name")).toBeNull();
  });

  it("shows the External Keycloak fields in that mode only", async () => {
    render(<Harness initial={inProduction} />);
    expect(screen.queryByLabelText("Keycloak URL")).toBeNull();
    await userEvent.click(
      screen.getByRole("radio", { name: /External Keycloak/ }),
    );
    expect(screen.getByLabelText("Keycloak URL")).toBeTruthy();
    expect(draftOf().mode).toBe("externalKeycloak");
  });

  it("disables OIDC for the basic-auth default and enables it for an oidc pick", async () => {
    render(<Harness initial={inProduction} />);
    const oidc = () =>
      screen.getByRole("radio", {
        name: /OIDC from the platform config/,
      }) as HTMLInputElement;
    expect(oidc().disabled).toBe(true);
    await userEvent.selectOptions(platformConfig(), "oidc-platform");
    expect(oidc().disabled).toBe(false);
  });
});
