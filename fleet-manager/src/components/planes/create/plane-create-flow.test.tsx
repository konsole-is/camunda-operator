// @vitest-environment jsdom
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  createPlane,
  previewPlaneCreate,
} from "@/app/fleet/planes/new/actions";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import type { WriteResult } from "@/lib/write/types";
import { fakeWriter } from "../../../../test/fakes/writer";
import { type PlaneCreateData, PlaneCreateFlow } from "./plane-create-flow";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));

const watch = vi.hoisted(() => ({
  failing: "",
  added: {} as Record<string, unknown[]>,
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, _opts: unknown, initial: unknown[]) => ({
    items: [...initial, ...(watch.added[kind] ?? [])],
    ...(kind === watch.failing && { error: `${kind} is forbidden` }),
  }),
  useResource: () => ({}),
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

afterEach(() => {
  cleanup();
  watch.failing = "";
  watch.added = {};
  fakeWriter.reset();
  vi.restoreAllMocks();
});

function show(initial: PlaneCreateData = data) {
  render(
    <PlaneCreateFlow
      initial={initial}
      mode={fakeWriter.mode}
      preview={previewPlaneCreate}
      create={createPlane}
    />,
    { wrapper: GitPushesProvider },
  );
}

const environment = (name: string, platformConfigRef?: string) => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name },
  spec: {
    ...(platformConfigRef && { platformConfigRef }),
    releases: { default: "camunda-8-9-23" },
    namespaceCreation: { allowed: true },
    sections: {
      "management-database": {
        required: true,
        allowed: ["management-database-server"],
        default: "management-database-server",
      },
      access: { allowed: ["gateway"], default: "gateway" },
    },
  },
});

const namespace = (name: string, env: string) => ({
  apiVersion: "v1",
  kind: "Namespace",
  metadata: { name, labels: { [ENVIRONMENT_LABEL]: env } },
});

const section = (
  name: string,
  label: string,
  slot: string,
  emptyOptionLabel?: string,
) => ({
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginSection",
  metadata: { name },
  spec: {
    appliesTo: ["CamundaManagementClusterClaim"],
    label,
    selection: "One",
    slot,
    ...(emptyOptionLabel && { emptyOptionLabel }),
  },
});

const registration = (name: string, label: string, spec: object) => ({
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginRegistration",
  metadata: { name },
  spec: {
    mode: "Optional",
    label,
    deletionPolicies: ["Delete"],
    serviceAccount: { name, namespace: "plugins" },
    ...spec,
  },
  status: {
    conditions: ["Accepted", "Ready"].map((type) => ({
      type,
      status: "True",
      reason: "Test",
      message: "",
      lastTransitionTime: "2026-11-10T00:00:00Z",
    })),
  },
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
  releases: [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaRelease",
      metadata: { name: "camunda-8-9-23" },
      spec: { version: "8.9.23" },
    },
  ],
  sections: [
    section("management-database", "Databases", "managementDatabases"),
    section("access", "Access", "externalAccess", "No access from outside"),
  ],
  registrations: [
    registration("management-database-server", "New PostgreSQL server", {
      sections: ["management-database"],
      controller: { name: "camunda-operator", version: "0.0.0" },
    }),
    registration("gateway", "Gateway", {
      sections: ["access"],
      parameterValues: {
        managementHostnamePattern: "{name}.{namespace}.example.com",
      },
    }),
  ],
  accessRequests: [],
  authConfigs: [],
  databaseServerPresets: [],
  databaseConfigs: [],
  planes: { clusters: [], claims: [], namespaces: [], environments: [] },
  images: {
    schemaVersion: 1,
    minors: {
      "8.9": {
        identity: { version: "8.9.10" },
        console: { version: "8.9.122" },
        "web-modeler-restapi": { version: "8.9.11" },
        keycloak: { tagPrefix: "quay-optimized-", version: "26.6.4" },
      },
    },
  },
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

async function toStorage() {
  await fillBasics();
  await userEvent.click(next());
  await userEvent.click(next());
  await userEvent.type(screen.getByLabelText("Username"), "admin");
  await userEvent.click(next());
}

async function toReview() {
  await toStorage();
  await userEvent.click(next());
}

const create = () =>
  screen.getByRole("button", { name: "Create" }) as HTMLButtonElement;

describe("PlaneCreateFlow", () => {
  it("blocks Next on step 1 until the basics are complete", async () => {
    show();
    expect(next().disabled).toBe(true);
    await fillBasics();
    expect(next().disabled).toBe(false);
  });

  it("moves to Environments served and back with the basics kept", async () => {
    show();
    await fillBasics();
    await userEvent.click(next());
    expect(screen.getByRole("group", { name: "Environments" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Next" })).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: /Basics/ }));
    expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe(
      "plane-a",
    );
  });

  it("lets the served list follow the environment of step 1 until the admin picks", async () => {
    show();
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
    show();
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
    show();
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
    show();
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
      show();
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
  it("shows a failed read of the Secrets and refuses a picked SMTP Secret while it lasts", async () => {
    watch.added.Secret = [
      { metadata: { name: "smtp" }, keys: ["username", "password"] },
    ];
    show();
    await fillBasics();
    await userEvent.click(next());
    await userEvent.click(next());
    await userEvent.type(screen.getByLabelText("Username"), "admin");
    await userEvent.type(screen.getByLabelText("Email"), "admin@example.com");
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Run Web Modeler" }),
    );
    await userEvent.type(
      screen.getByLabelText("SMTP host"),
      "smtp.example.com",
    );
    await userEvent.type(
      screen.getByLabelText("From address"),
      "modeler@example.com",
    );
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Credentials Secret" }),
      "smtp",
    );
    expect(next().disabled).toBe(false);
    watch.failing = "Secret";
    await userEvent.type(screen.getByLabelText("From name"), "M");
    expect(
      screen
        .getAllByRole("alert")
        .some(
          (a) => a.textContent === "Could not read Secret: Secret is forbidden",
        ),
    ).toBe(true);
    expect(next().disabled).toBe(true);
  });

  it("blocks Next on Components and versions until the admin username is set", async () => {
    show();
    await fillBasics();
    await userEvent.click(next());
    await userEvent.click(next());
    expect(screen.getByLabelText("Username")).toBeTruthy();
    expect(next().disabled).toBe(true);
    expect(
      screen.getByText("Enter the username of the first administrator."),
    ).toBeTruthy();
    await userEvent.type(screen.getByLabelText("Username"), "admin");
    expect(next().disabled).toBe(false);
  });

  it("blocks Next on a Keycloak version of 27.0.0", async () => {
    show();
    await fillBasics();
    await userEvent.click(next());
    await userEvent.click(next());
    await userEvent.type(screen.getByLabelText("Username"), "admin");
    const version = screen.getByLabelText("Keycloak version");
    await userEvent.clear(version);
    await userEvent.type(version, "27.0.0");
    expect(next().disabled).toBe(true);
    expect(
      screen.getByText(
        "Keycloak needs a version from 26.0.0 up to, but not including, 27.0.0.",
      ),
    ).toBeTruthy();
  });
  it("blocks Storage and access while the management host is taken", async () => {
    const taken = {
      apiVersion: "plugins.camunda.io/v1alpha1",
      kind: "PluginRequest",
      metadata: { namespace: "team-b", name: "plane-b-access" },
      spec: {
        claimRef: { kind: "CamundaManagementClusterClaim", name: "plane-b" },
        section: "access",
        registration: "gateway",
      },
      status: { outputs: { managementHost: "plane-a.platform.example.com" } },
    };
    show({ ...data, accessRequests: [taken] } as unknown as PlaneCreateData);
    await toStorage();
    expect(
      screen.getByText(
        "The host plane-a.platform.example.com is taken by team-b/plane-b. Choose another name.",
      ),
    ).toBeTruthy();
    expect(next().disabled).toBe(true);
  });

  it("blocks Storage and access while the contract name exists", async () => {
    show({
      ...data,
      authConfigs: [{ metadata: { name: "plane-a" } }],
    } as PlaneCreateData);
    await toStorage();
    expect(
      screen.getByText(
        "A contract plane-a exists already. Enter another name.",
      ),
    ).toBeTruthy();
    expect(next().disabled).toBe(true);
    await userEvent.type(screen.getByLabelText("Contract name"), "ops");
    expect(next().disabled).toBe(false);
  });

  it("Create writes the claim and shows the progress rows", async () => {
    show();
    await toReview();
    await waitFor(() => expect(create().disabled).toBe(false));
    await userEvent.click(create());
    expect(
      await screen.findByRole("heading", {
        name: "Creating the management plane plane-a",
      }),
    ).toBeTruthy();
    for (const row of [
      "Environment and policy",
      "Databases",
      "Access",
      "Management cluster",
    ]) {
      expect(screen.getByText(row)).toBeTruthy();
    }
    expect(screen.getByRole("status").textContent).toContain(
      "Created the management plane plane-a.",
    );
    expect(fakeWriter.applied).toHaveLength(1);
    expect(fakeWriter.applied[0].apply.map((o) => o.kind)).toEqual([
      "CamundaManagementClusterClaim",
    ]);
    expect(fakeWriter.applied[0].create).toEqual([
      "CamundaManagementClusterClaim/platform/plane-a",
    ]);
  });

  it("a Stale answer shows the toast, keeps the review, and shows no progress", async () => {
    fakeWriter.results = [
      {
        ok: false,
        errors: [
          {
            code: "Stale",
            message:
              "The CamundaManagementClusterClaim plane-a exists already. Pick another name.",
          },
        ],
      },
    ];
    show();
    await toReview();
    await waitFor(() => expect(create().disabled).toBe(false));
    await userEvent.click(create());
    expect((await screen.findByRole("alert")).textContent).toContain(
      "The CamundaManagementClusterClaim plane-a exists already. Pick another name.",
    );
    expect(create()).toBeTruthy();
    expect(
      screen.queryByRole("heading", { name: /Creating the management plane/ }),
    ).toBeNull();
  });

  it("a failed create that made its namespace moves the draft to that namespace, says so, and a second Create writes only the claim", async () => {
    vi.spyOn(fakeWriter, "apply").mockImplementationOnce(async (change) => {
      fakeWriter.applied.push(structuredClone(change));
      // The watch now sees the namespace that the write created.
      watch.added = { Namespace: [namespace("team-new", "production")] };
      return {
        ok: false,
        errors: [{ code: "Invalid", message: "The claim is invalid." }],
        createdNamespace: "team-new",
      };
    });
    show();
    await fillBasics();
    await userEvent.click(
      screen.getByRole("radio", { name: "Create a namespace" }),
    );
    await userEvent.type(screen.getAllByLabelText("Name")[1], "team-new");
    await userEvent.click(next());
    await userEvent.click(next());
    await userEvent.type(screen.getByLabelText("Username"), "admin");
    await userEvent.click(next());
    await userEvent.click(next());
    await waitFor(() => expect(create().disabled).toBe(false));
    await userEvent.click(create());

    expect(fakeWriter.applied[0].apply.map((o) => o.kind)).toEqual([
      "Namespace",
      "CamundaManagementClusterClaim",
    ]);
    expect((await screen.findByRole("note", { name: "" })).textContent).toBe(
      "The write created the namespace team-new, but not the claim. The namespace stays, and the plane now goes into it as an existing namespace. Fix the cause and create the plane again.",
    );
    expect(screen.getByRole("alert").textContent).toContain(
      "The claim is invalid.",
    );

    await waitFor(() => expect(create().disabled).toBe(false));
    await userEvent.click(create());
    expect(fakeWriter.applied).toHaveLength(2);
    expect(fakeWriter.applied[1].apply.map((o) => o.kind)).toEqual([
      "CamundaManagementClusterClaim",
    ]);
    expect(fakeWriter.applied[1].apply[0].metadata.namespace).toBe("team-new");
    expect(
      await screen.findByRole("heading", {
        name: "Creating the management plane plane-a",
      }),
    ).toBeTruthy();
  });

  it("keeps Create disabled while a create runs, also after Back and Next", async () => {
    let finish: (result: WriteResult) => void = () => {};
    vi.spyOn(fakeWriter, "apply").mockImplementationOnce((change) => {
      fakeWriter.applied.push(structuredClone(change));
      return new Promise((resolve) => {
        finish = resolve;
      });
    });
    show();
    await toReview();
    await waitFor(() => expect(create().disabled).toBe(false));
    await userEvent.click(create());

    await userEvent.click(screen.getByRole("button", { name: "Back" }));
    const previews = fakeWriter.previewed.length;
    await userEvent.click(next());
    await waitFor(() =>
      expect(fakeWriter.previewed.length).toBeGreaterThan(previews),
    );
    expect(create().disabled).toBe(true);
    await userEvent.click(create());
    expect(fakeWriter.applied).toHaveLength(1);

    finish({ ok: true, mode: "direct" });
    expect(
      await screen.findByRole("heading", {
        name: "Creating the management plane plane-a",
      }),
    ).toBeTruthy();
  });

  async function refusedCreateInNewNamespace() {
    await fillBasics();
    await userEvent.click(
      screen.getByRole("radio", { name: "Create a namespace" }),
    );
    await userEvent.type(screen.getAllByLabelText("Name")[1], "team-new");
    await userEvent.click(next());
    await userEvent.click(next());
    await userEvent.type(screen.getByLabelText("Username"), "admin");
    await userEvent.click(next());
    await userEvent.click(next());
    await waitFor(() => expect(create().disabled).toBe(false));
  }

  const refused = {
    ok: false as const,
    errors: [{ code: "Invalid" as const, message: "The claim is invalid." }],
    createdNamespace: "team-new",
  };

  async function backToBasics() {
    for (let i = 0; i < 4; i++) {
      await userEvent.click(screen.getByRole("button", { name: "Back" }));
    }
  }

  it("drops the note about the kept namespace once the admin picks another namespace", async () => {
    vi.spyOn(fakeWriter, "apply").mockImplementationOnce(async (change) => {
      fakeWriter.applied.push(structuredClone(change));
      watch.added = { Namespace: [namespace("team-new", "production")] };
      return refused;
    });
    show();
    await refusedCreateInNewNamespace();
    await userEvent.click(create());
    expect(await screen.findByRole("note", { name: "" })).toBeTruthy();

    await backToBasics();
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Namespace" }),
      "platform",
    );
    expect(
      screen.queryByText(/The write created the namespace team-new/),
    ).toBeNull();
  });

  it("keeps a namespace that the admin changed while the refused write ran", async () => {
    let finish: (result: WriteResult) => void = () => {};
    vi.spyOn(fakeWriter, "apply").mockImplementationOnce((change) => {
      fakeWriter.applied.push(structuredClone(change));
      return new Promise((resolve) => {
        finish = resolve;
      });
    });
    show();
    await refusedCreateInNewNamespace();
    await userEvent.click(create());

    await backToBasics();
    await userEvent.click(
      screen.getByRole("radio", { name: "Use an existing namespace" }),
    );
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Namespace" }),
      "platform",
    );
    watch.added = { Namespace: [namespace("team-new", "production")] };
    finish(refused);

    expect(await screen.findByText("The claim is invalid.")).toBeTruthy();
    expect(
      screen.queryByText(/The write created the namespace team-new/),
    ).toBeNull();

    for (let i = 0; i < 4; i++) await userEvent.click(next());
    await waitFor(() => expect(create().disabled).toBe(false));
    await userEvent.click(create());
    expect(fakeWriter.applied).toHaveLength(2);
    expect(
      fakeWriter.applied[1].apply.map((o) => o.metadata.namespace),
    ).toEqual(["platform"]);
  });
});
