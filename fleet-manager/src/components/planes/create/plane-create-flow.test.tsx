// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { type PlaneCreateData, PlaneCreateFlow } from "./plane-create-flow";

const watch = vi.hoisted(() => ({ failing: "" }));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
    ...(kind === watch.failing && { error: `${kind} is forbidden` }),
  }),
}));

afterEach(() => {
  cleanup();
  watch.failing = "";
});

const environment = (name: string, platformConfigRef?: string) => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name },
  spec: {
    ...(platformConfigRef && { platformConfigRef }),
    namespaceCreation: { allowed: true },
  },
});

const namespace = (name: string, env: string) => ({
  apiVersion: "v1",
  kind: "Namespace",
  metadata: { name, labels: { [ENVIRONMENT_LABEL]: env } },
});

const data = {
  environments: [
    environment("production", "default-platform"),
    environment("staging", "default-platform"),
  ],
  namespaces: [
    namespace("platform", "production"),
    namespace("ops", "staging"),
  ],
  clusters: [],
  platformConfigs: [
    { metadata: { name: "default-platform" }, spec: {} },
    {
      metadata: { name: "oidc-platform" },
      spec: { auth: { method: "oidc", oidc: { issuerUrl: "https://idp" } } },
    },
  ],
  planes: { clusters: [], claims: [], namespaces: [] },
} as unknown as PlaneCreateData;

const next = () =>
  screen.getByRole("button", { name: "Next" }) as HTMLButtonElement;

async function fillBasics(env = /production/, ns = "platform") {
  await userEvent.click(screen.getByRole("radio", { name: env }));
  await userEvent.type(screen.getByLabelText("Name"), "plane-a");
  await userEvent.selectOptions(
    screen.getByRole("combobox", { name: "Namespace" }),
    ns,
  );
  await userEvent.click(
    screen.getByRole("radio", { name: /Keycloak, run by this plane/ }),
  );
}

describe("PlaneCreateFlow", () => {
  it("blocks Next on step 1 until the basics are complete", async () => {
    render(<PlaneCreateFlow initial={data} />);
    expect(next().disabled).toBe(true);
    await fillBasics();
    expect(next().disabled).toBe(false);
  });

  it("moves to Environments served and back with the basics kept", async () => {
    render(<PlaneCreateFlow initial={data} />);
    await fillBasics();
    await userEvent.click(next());
    expect(screen.getByRole("group", { name: "Environments" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Next" })).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: /Basics/ }));
    expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe(
      "plane-a",
    );
  });

  it("lets the served list follow the environment of step 1 until the admin picks", async () => {
    render(<PlaneCreateFlow initial={data} />);
    await fillBasics();
    await userEvent.click(next());
    const box = (name: RegExp) =>
      screen.getByRole("checkbox", { name }) as HTMLInputElement;
    expect(box(/production/).checked).toBe(true);
    await userEvent.click(screen.getByRole("button", { name: /Basics/ }));
    await fillBasics(/staging/, "ops");
    await userEvent.click(next());
    expect(box(/staging/).checked).toBe(true);
    expect(box(/production/).checked).toBe(false);
  });

  it("keeps the served list that the admin picked when the environment changes", async () => {
    render(<PlaneCreateFlow initial={data} />);
    await fillBasics();
    await userEvent.click(next());
    const box = (name: RegExp) =>
      screen.getByRole("checkbox", { name }) as HTMLInputElement;
    await userEvent.click(box(/staging/));
    await userEvent.click(screen.getByRole("button", { name: /Basics/ }));
    await fillBasics(/staging/, "ops");
    await userEvent.click(next());
    expect(box(/staging/).checked).toBe(true);
    expect(box(/production/).checked).toBe(true);
  });

  it("blocks the step when OIDC was picked and the platform config switches to basic", async () => {
    render(<PlaneCreateFlow initial={data} />);
    await fillBasics();
    const config = screen.getByRole("combobox", { name: "Platform config" });
    await userEvent.selectOptions(config, "oidc-platform");
    await userEvent.click(
      screen.getByRole("radio", { name: /OIDC from the platform config/ }),
    );
    expect(next().disabled).toBe(false);
    await userEvent.selectOptions(config, "default-platform");
    expect(
      screen.getByText(
        "OIDC needs a platform config with OIDC authentication.",
      ),
    ).toBeTruthy();
    expect(next().disabled).toBe(true);
  });

  it("keeps the new namespace fields when the admin switches from an existing namespace", async () => {
    render(<PlaneCreateFlow initial={data} />);
    await fillBasics();
    const create = screen.getByRole("radio", {
      name: "Create a namespace",
    }) as HTMLInputElement;
    await userEvent.click(create);
    expect(
      (
        screen.getByRole("radio", {
          name: "Create a namespace",
        }) as HTMLInputElement
      ).checked,
    ).toBe(true);
    expect(screen.getAllByLabelText("Name")).toHaveLength(2);
  });

  it.each(["CamundaManagementClusterClaim", "Environment"])(
    "shows a failed read of %s above the steps and blocks step 1",
    async (kind) => {
      watch.failing = kind;
      render(<PlaneCreateFlow initial={data} />);
      expect(screen.getByRole("alert").textContent).toBe(
        `Could not read ${kind}: ${kind} is forbidden`,
      );
      await fillBasics();
      expect(next().disabled).toBe(true);
      expect(
        screen.getByText(
          `The fleet manager could not read the ${kind} objects: ${kind} is forbidden`,
        ),
      ).toBeTruthy();
    },
  );
});
