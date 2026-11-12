// @vitest-environment jsdom
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { EMPTY_ADMIN } from "@/lib/planes/components";
import {
  EMPTY_PLANE_DRAFT,
  type PlaneContext,
  type PlaneDraft,
  planeClaimOf,
  planeCreateChange,
} from "@/lib/planes/draft";
import type {
  Change,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import { PlaneReviewStep } from "./plane-review-step";

afterEach(cleanup);

const ctx: PlaneContext = {
  environments: [
    {
      metadata: { name: "production" },
      spec: { namespaceCreation: { allowed: true } },
    } as Environment,
  ],
  platformConfigs: [],
  takenNames: [],
  namespaces: [{ name: "platform", environment: "production" }],
  releases: [],
  images: { schemaVersion: 1, minors: {} },
  sections: [
    {
      metadata: { name: "management-database" },
      spec: { label: "Databases", slot: "managementDatabases" },
    },
    {
      metadata: { name: "access" },
      spec: { label: "Access", slot: "externalAccess" },
    },
  ] as PluginSection[],
  registrations: [
    {
      metadata: { name: "management-database-server" },
      spec: { label: "New PostgreSQL server" },
    },
    { metadata: { name: "gateway" }, spec: { label: "Gateway" } },
  ] as PluginRegistration[],
  accessRequests: [],
  authConfigs: [],
  planeClaims: [],
};

const draft: PlaneDraft = {
  ...EMPTY_PLANE_DRAFT,
  environment: "production",
  name: "plane-a",
  namespace: { kind: "existing", name: "platform" },
  mode: "keycloak",
  admin: { ...EMPTY_ADMIN, username: "admin" },
  storage: {
    "management-database": [
      {
        section: "management-database",
        registration: "management-database-server",
      },
    ],
  },
};

const clean: Preview = { objects: [], errors: [] };

function review(
  d: PlaneDraft = draft,
  {
    mode = "direct",
    preview = async () => clean,
    create = vi.fn(async () => ({
      ok: true as const,
      mode: "direct" as const,
    })),
    onWritten = vi.fn(),
  }: {
    mode?: WriteMode;
    preview?: () => Promise<Preview>;
    create?: (change: Change) => Promise<WriteResult>;
    onWritten?: (result: WriteResult) => void;
  } = {},
) {
  const change = planeCreateChange(d, ctx);
  render(
    <PlaneReviewStep
      change={change}
      claim={planeClaimOf(d, ctx)}
      draft={d}
      ctx={ctx}
      mode={mode}
      preview={preview}
      create={create}
      onWritten={onWritten}
    />,
  );
  return { change, create, onWritten };
}

const effects = () =>
  within(screen.getByRole("list", { name: "What this change does" }))
    .getAllByRole("listitem")
    .map((li) => li.textContent);
const confirm = (name = "Create") =>
  screen.getByRole("button", { name }) as HTMLButtonElement;

describe("PlaneReviewStep", () => {
  it("shows the claim YAML and what the claim creates, with the namespace only when new", async () => {
    review();
    const yaml = screen.getByRole("figure", { name: "The claim" });
    expect(yaml.textContent).toContain("kind: CamundaManagementClusterClaim");
    expect(yaml.textContent).toContain("name: plane-a");
    expect(effects()).toEqual([
      "Creates the claim plane-a.",
      "Requests the databases: New PostgreSQL server.",
      "Creates the CamundaManagementCluster plane-a.",
      "The plane writes the contract plane-a.",
    ]);
    expect(
      screen.getByText(
        "Each cluster that attaches restarts its pods once, because it gets the Console settings.",
      ),
    ).toBeTruthy();
    await waitFor(() => expect(confirm().disabled).toBe(false));
  });

  it("names the new namespace and the access option when the draft has them", () => {
    review({
      ...draft,
      namespace: { kind: "new", name: "team-new", labels: {} },
      contractName: "ops",
      storage: {
        ...draft.storage,
        access: [{ section: "access", registration: "gateway" }],
      },
    });
    expect(effects()).toEqual([
      "Creates the namespace team-new.",
      "Creates the claim plane-a.",
      "Requests the databases: New PostgreSQL server.",
      "Requests access: Gateway.",
      "Creates the CamundaManagementCluster plane-a.",
      "The plane writes the contract ops.",
    ]);
  });

  it("Create reads Push to Git in Git mode", async () => {
    review(draft, { mode: "git" });
    await waitFor(() => expect(confirm("Push to Git").disabled).toBe(false));
  });

  it("a refused preview shows its message and blocks Create", async () => {
    const { create } = review(draft, {
      preview: async () => ({
        objects: [],
        errors: [
          {
            code: "Invalid",
            message:
              "versions.keycloak is required with the keycloak identity provider",
          },
        ],
      }),
    });
    expect((await screen.findByRole("alert")).textContent).toContain(
      "versions.keycloak is required with the keycloak identity provider",
    );
    expect(confirm().disabled).toBe(true);
    await userEvent.click(confirm());
    expect(create).not.toHaveBeenCalled();
  });

  it("Create sends the change once and gives the result to onWritten", async () => {
    const { change, create, onWritten } = review();
    await waitFor(() => expect(confirm().disabled).toBe(false));
    await userEvent.click(confirm());
    expect(create).toHaveBeenCalledTimes(1);
    expect(create).toHaveBeenCalledWith(change);
    expect(onWritten).toHaveBeenCalledWith({ ok: true, mode: "direct" });
  });
});
