// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "@/components/fleet/namespaced-secret-ref-picker";
import type { Environment } from "@/generated/crds/environment";
import type { ReadError } from "@/lib/k8s/unread";
import { foreignFields } from "@/lib/write/ownership";
import type { KubeObject, Preview, WriteMode } from "@/lib/write/types";
import { platformFixtures, twoMinorTable } from "./fixtures";
import { DELETE_EFFECT, ROLL_NOTICE } from "./model";
import { type PlatformActions, PlatformDetail } from "./platform-detail";
import type { PlatformSources } from "./sources";

const router = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));
vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
  useResources: () => {
    throw new Error("the test reads Secrets through the static reader");
  },
  useWatchesFresh: () => [false],
}));

afterEach(cleanup);

const SECRETS = staticSecretRefs({
  "camunda-system": [
    { name: "oidc-credentials", keys: ["client-secret", "identity-secret"] },
  ],
});

function setup(
  opts: {
    name?: string;
    mode?: WriteMode;
    sources?: PlatformSources;
    errors?: ReadError[];
    stale?: string[];
    preview?: Preview;
  } = {},
) {
  const {
    name = "oidc-prod",
    sources = platformFixtures(),
    errors = [],
    stale = [],
    preview = { objects: [], errors: [] },
  } = opts;
  const mode = "mode" in opts ? opts.mode : "direct";
  const actions = {
    previewChange: vi.fn<PlatformActions["previewChange"]>(async () => preview),
    applyChange: vi.fn<PlatformActions["applyChange"]>(async () => ({
      ok: true as const,
      mode: "direct" as const,
    })),
  };
  const view = (s: PlatformSources, e: ReadError[], st: string[]) => (
    <GitPushesProvider>
      <SecretRefsReaderContext.Provider value={SECRETS}>
        <PlatformDetail
          sources={{ ...s, errors: e, stale: st }}
          imagesTable={twoMinorTable}
          name={name}
          mode={mode}
          actions={actions}
        />
      </SecretRefsReaderContext.Provider>
    </GitPushesProvider>
  );
  const { rerender } = render(view(sources, errors, stale));
  return {
    actions,
    rerender: (
      s: PlatformSources,
      e: ReadError[] = errors,
      st: string[] = stale,
    ) => rerender(view(s, e, st)),
  };
}

const CONFIG_REF = {
  apiVersion: "core.camunda.io/v1",
  kind: "CamundaPlatformConfig",
  name: "oidc-prod",
};

function foreignField(path: string) {
  return {
    path,
    steps: path.split(".").map((field) => ({ field })),
    managers: [
      {
        name: "kubectl-client-side-apply",
        operation: "Update",
        kind: "other" as const,
        known: true,
      },
    ],
  };
}

const input = (label: string) =>
  screen.getByLabelText(label) as HTMLInputElement;

async function openDelete(name: string) {
  fireEvent.change(screen.getByLabelText(`Type ${name} to confirm`), {
    target: { value: name },
  });
  fireEvent.click(screen.getByRole("button", { name: "Review the delete" }));
  return screen.findByRole("region", { name: "Review the delete" });
}

describe("PlatformDetail", () => {
  it("shows the auth method, the OIDC fields, the management plane clients, the license, and the image repositories of the config", () => {
    setup();

    expect(
      screen.getByRole("heading", { level: 1, name: "oidc-prod" }),
    ).toBeDefined();
    expect(screen.getByRole("radio", { name: "OIDC" })).toHaveProperty(
      "checked",
      true,
    );
    expect(input("Issuer URL").value).toBe(
      "https://login.example.com/realms/camunda",
    );
    const identity = screen.getByRole("group", { name: "Management Identity" });
    expect(
      (within(identity).getByLabelText("Client ID") as HTMLInputElement).value,
    ).toBe("management-identity");
    const license = screen.getByRole("group", { name: "License key" });
    expect(
      (
        within(license).getByRole("combobox", {
          name: "Secret",
        }) as HTMLSelectElement
      ).value,
    ).toBe("camunda-license");
    expect(input("Orchestration cluster").value).toBe(
      "registry.example.com/camunda/camunda",
    );
    expect(
      within(screen.getByRole("list", { name: "Database servers" })).getByText(
        "team-a/orders-db from the preset pg-standard",
      ),
    ).toBeDefined();
  });

  it("says that a change rolls clusters, their Optimize, and management planes and changes a database server only on the postgres image", () => {
    setup();

    expect(screen.getByRole("note").textContent).toBe(ROLL_NOTICE);
    expect(ROLL_NOTICE).toBe(
      "A change rolls the pods of every cluster and management plane that uses this config, and of the Optimize of each such cluster. A database server changes only when the postgres image changes.",
    );
  });

  it("opens a config with a missing Secret with no change", () => {
    setup({ name: "broken" });

    expect(
      screen
        .getAllByRole("alert")
        .map((a) => a.textContent)
        .join(" "),
    ).toContain(
      "spec.licenseSecretRef: Secret camunda-system/camunda-license not found",
    );
    expect(screen.getByText("camunda-license (not found)")).toBeDefined();
    expect(screen.queryByRole("button", { name: "Review" })).toBeNull();
  });

  it("reviews an edit as an update with the roll notice", async () => {
    const { actions } = setup();

    fireEvent.change(input("Keycloak"), {
      target: { value: "mirror.example.com/keycloak" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));

    await waitFor(() => expect(actions.previewChange).toHaveBeenCalled());
    const change = actions.previewChange.mock.lastCall![0];
    expect(change.force).toBeUndefined();
    expect(change.preconditions).toEqual({
      "CamundaPlatformConfig//oidc-prod": "21",
    });
    expect(
      (change.apply[0].spec as { images: Record<string, string> }).images
        .keycloak,
    ).toBe("mirror.example.com/keycloak");
    const review = screen.getByRole("region", {
      name: "Review the platform config",
    });
    expect(within(review).getByText(ROLL_NOTICE)).toBeDefined();
  });

  it("a save that changes a field of another tool shows the field and asks", async () => {
    const { actions } = setup();
    const path = "spec.images.keycloak";
    const ownership = [
      {
        ref: CONFIG_REF,
        changed: [foreignField(path)],
        cleared: [],
      },
    ];
    actions.previewChange.mockImplementation(async (change) => ({
      objects: [],
      errors: change.takeOver
        ? []
        : [
            {
              code: "Conflict",
              message: 'conflict with "kubectl-client-side-apply"',
              path,
              ref: CONFIG_REF,
            },
          ],
      ownership,
    }));

    fireEvent.change(input("Keycloak"), {
      target: { value: "mirror.example.com/keycloak" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));

    const review = screen.getByRole("region", {
      name: "Review the platform config",
    });
    const choice = await within(review).findByRole("checkbox", {
      name: "Take over these fields",
    });
    expect(
      within(review).getByRole("group", {
        name: "Fields that another tool set",
      }).textContent,
    ).toContain(
      `${path}: kubectl-client-side-apply set it. The save changes it.`,
    );
    const save = within(review).getByRole("button", { name: "Save" });
    expect(save).toHaveProperty("disabled", true);

    fireEvent.click(choice);
    await waitFor(() => expect(save).toHaveProperty("disabled", false));
    fireEvent.click(save);
    await waitFor(() => expect(actions.applyChange).toHaveBeenCalledTimes(1));
    expect(actions.applyChange.mock.calls[0][0]).toMatchObject({
      takeOver: { "CamundaPlatformConfig//oidc-prod": [path] },
    });
    expect(actions.applyChange.mock.calls[0][0].force).toBeUndefined();
  });

  it("a switch to Basic with the take-over of the OIDC block of another tool saves the method and the removal", async () => {
    const { actions } = setup();
    const path = "spec.auth.oidc";
    const ownership = [
      {
        ref: CONFIG_REF,
        changed: [foreignField("spec.auth.method")],
        cleared: [foreignField(path)],
      },
    ];
    actions.previewChange.mockImplementation(async (change) => ({
      objects: [],
      errors: change.takeOver
        ? []
        : [
            {
              code: "Conflict",
              message: 'conflict with "kubectl-client-side-apply"',
              path: "spec.auth.method",
              ref: CONFIG_REF,
            },
          ],
      ownership,
    }));

    fireEvent.click(screen.getByRole("radio", { name: "Basic" }));
    fireEvent.click(screen.getByRole("button", { name: "Review" }));

    const review = screen.getByRole("region", {
      name: "Review the platform config",
    });
    const choice = await within(review).findByRole("checkbox", {
      name: "Take over these fields",
    });
    expect(
      within(review).getByRole("group", {
        name: "Fields that another tool set",
      }).textContent,
    ).toContain(
      `${path}: kubectl-client-side-apply set it. It stays unless you take it over.`,
    );
    const save = within(review).getByRole("button", { name: "Save" });
    expect(save).toHaveProperty("disabled", true);

    fireEvent.click(choice);
    await waitFor(() => expect(save).toHaveProperty("disabled", false));
    fireEvent.click(save);
    await waitFor(() => expect(actions.applyChange).toHaveBeenCalledTimes(1));
    const applied = actions.applyChange.mock.calls[0][0];
    expect(applied).toMatchObject({
      takeOver: { "CamundaPlatformConfig//oidc-prod": ["spec.auth.method"] },
      remove: { "CamundaPlatformConfig//oidc-prod": [path] },
    });
    expect(applied.force).toBeUndefined();
    expect(applied.apply[0].spec.auth).toEqual({ method: "basic" });
  });

  it("blocks the review while a field is invalid", () => {
    setup();

    fireEvent.change(input("Issuer URL"), {
      target: { value: "login.example.com" },
    });

    expect(
      screen.getAllByText("The issuer URL is an http or https URL.").length,
    ).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Review" })).toHaveProperty(
      "disabled",
      true,
    );
  });

  it("offers no delete while the config has users", () => {
    setup();

    expect(
      screen.getByText(
        "Delete is possible when no cluster, management plane, database server, claim, or environment uses this config.",
      ),
    ).toBeDefined();
    expect(
      screen.queryByRole("button", { name: "Review the delete" }),
    ).toBeNull();
  });

  it("offers the delete of an unused config with the InvalidReference warning", async () => {
    const { actions } = setup({ name: "basic-lab" });

    expect(screen.getByText(DELETE_EFFECT)).toBeDefined();
    expect(DELETE_EFFECT).toBe(
      "Users of a deleted config report InvalidReference and keep running.",
    );
    const review = await openDelete("basic-lab");

    await waitFor(() => expect(actions.previewChange).toHaveBeenCalled());
    expect(actions.previewChange.mock.lastCall![0]).toMatchObject({
      intent: "Delete the platform config basic-lab",
      delete: [{ kind: "CamundaPlatformConfig", name: "basic-lab" }],
      uids: { "CamundaPlatformConfig//basic-lab": "uid-basic-lab" },
    });
    expect(
      within(review).getByRole("button", {
        name: "Delete the platform config",
      }),
    ).toBeDefined();
    expect(within(review).getByText(DELETE_EFFECT)).toBeDefined();
  });

  it("offers no delete while a kind that can use the config cannot be read", () => {
    setup({
      name: "basic-lab",
      errors: [{ kind: "DatabaseServer", message: "forbidden" }],
    });

    expect(
      screen.getByText(
        "Delete is possible when the fleet manager can read DatabaseServer.",
      ),
    ).toBeDefined();
    expect(
      screen.queryByRole("button", { name: "Review the delete" }),
    ).toBeNull();
  });

  it("says that the users are not known while a kind that can use the config cannot be read", () => {
    setup({
      name: "basic-lab",
      errors: [{ kind: "DatabaseServer", message: "forbidden" }],
    });

    expect(
      screen.getByText(
        "The users are not known while the fleet manager cannot read DatabaseServer.",
      ),
    ).toBeDefined();
    expect(screen.queryByText("Nothing uses this config.")).toBeNull();
    expect(
      screen.queryByText(/^No cluster or management plane uses/),
    ).toBeNull();
  });

  it("offers no delete while the watch of a kind that can use the config has not answered", () => {
    setup({ name: "basic-lab", stale: ["Environment", "Namespace"] });

    expect(
      screen.getByText(
        "Delete is possible when the fleet manager can read Environment.",
      ),
    ).toBeDefined();
    expect(
      screen.queryByRole("button", { name: "Review the delete" }),
    ).toBeNull();
    expect(
      screen.getByText(
        "The users are not known while the fleet manager cannot read Environment.",
      ),
    ).toBeDefined();
  });

  it("offers the delete while only a kind that cannot use the config has no snapshot", () => {
    setup({ name: "basic-lab", stale: ["Namespace", "CamundaRelease"] });

    expect(
      screen.getByRole("button", { name: "Review the delete" }),
    ).toBeDefined();
  });

  it("closes the delete review when a user appears", async () => {
    const { rerender } = setup({ name: "basic-lab" });
    await openDelete("basic-lab");

    const sources = platformFixtures();
    rerender({
      ...sources,
      environments: [
        ...sources.environments,
        {
          apiVersion: "fleet.camunda.io/v1alpha1",
          kind: "Environment",
          metadata: { name: "lab" },
          spec: { platformConfigRef: "basic-lab" },
        } as Environment,
      ],
    });

    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
    expect(
      screen.queryByRole("button", { name: "Review the delete" }),
    ).toBeNull();
  });

  it("closes the delete review when a kind that can use the config turns unreadable", async () => {
    const { rerender } = setup({ name: "basic-lab" });
    await openDelete("basic-lab");

    rerender(platformFixtures(), [
      { kind: "CamundaClusterClaim", message: "forbidden" },
    ]);

    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
  });

  it("closes the delete review when the watch of a kind that can use the config loses its snapshot", async () => {
    const { rerender } = setup({ name: "basic-lab" });
    await openDelete("basic-lab");

    rerender(platformFixtures(), [], ["DatabaseServerPreset"]);

    expect(
      screen.queryByRole("region", { name: "Review the delete" }),
    ).toBeNull();
  });

  it("says in the review that a cleared field of another tool stays unless you take it over", async () => {
    const path = "spec.images.camunda";
    setup({
      preview: {
        objects: [],
        errors: [],
        ownership: [
          { ref: CONFIG_REF, changed: [], cleared: [foreignField(path)] },
        ],
      },
    });

    fireEvent.change(input("Orchestration cluster"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));

    const review = await screen.findByRole("region", {
      name: "Review the platform config",
    });
    expect(
      (
        await within(review).findByRole("group", {
          name: "Fields that another tool set",
        })
      ).textContent,
    ).toContain(
      `${path}: kubectl-client-side-apply set it. It stays unless you take it over.`,
    );
    expect(
      within(review).getByRole("checkbox", { name: "Take over these fields" }),
    ).toBeDefined();
  });

  it("warns in the Git review that a cleared field of another field manager stays", async () => {
    const live = platformFixtures().platformConfigs.find(
      (c) => c.metadata.name === "oidc-prod",
    ) as unknown as KubeObject;
    setup({
      mode: "git",
      preview: {
        objects: [
          {
            ref: {
              apiVersion: "core.camunda.io/v1",
              kind: "CamundaPlatformConfig",
              name: "oidc-prod",
            },
            before: live,
            after: live,
            diff: [],
          },
        ],
        errors: [],
      },
    });

    fireEvent.change(input("Orchestration cluster"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));

    const review = await screen.findByRole("region", {
      name: "Review the platform config",
    });
    expect(
      await within(review).findByText(
        "spec.images stays, because another tool set it.",
      ),
    ).toBeDefined();
  });

  it("labels the delete Push the delete to Git in Git mode", async () => {
    setup({ name: "basic-lab", mode: "git" });

    const review = await openDelete("basic-lab");

    expect(
      within(review).getByRole("button", { name: "Push the delete to Git" }),
    ).toBeDefined();
  });

  it("disables the form without a write mode", () => {
    setup({ mode: undefined });

    expect(input("Issuer URL").closest("fieldset:disabled")).not.toBeNull();
    expect(
      screen.getByText(/^Changes are off/, { selector: "[role=alert]" }),
    ).toBeDefined();
  });

  it("says when the config does not exist", () => {
    setup({ name: "nope" });

    expect(screen.getByRole("alert").textContent).toBe(
      "The platform config nope does not exist.",
    );
  });

  it("shows the defaults of the minors that the users run", () => {
    const sources = platformFixtures();
    sources.clusters.push({
      ...sources.clusters[0],
      metadata: { namespace: "team-c", name: "next" },
      status: { management: { version: "8.10.1" } },
    } as (typeof sources.clusters)[number]);
    setup({ sources });

    expect(
      within(
        screen.getByRole("list", { name: "Defaults of Web Modeler REST API" }),
      )
        .getAllByRole("listitem")
        .map((li) => li.textContent),
    ).toEqual([
      "camunda/web-modeler-restapi for Camunda 8.9",
      "camunda/hub for Camunda 8.10",
    ]);
  });

  it("does not call the config unused when its users have no known version", () => {
    const base = platformFixtures();
    setup({
      sources: {
        ...base,
        clusters: base.clusters.map((c) => ({ ...c, status: undefined })),
        planes: [],
      },
    });

    expect(
      screen.queryByText(/^No cluster or management plane uses/),
    ).toBeNull();
    expect(
      screen.getByText(
        "No Camunda version is known for the clusters and management planes that use this config. The defaults are those of Camunda 8.10, the newest in the images table.",
      ),
    ).toBeDefined();
  });

  it("says that the minors can be incomplete while the clusters cannot be read", () => {
    setup({
      sources: { ...platformFixtures(), clusters: [] },
      errors: [{ kind: "CamundaCluster", message: "forbidden" }],
    });

    expect(
      screen.getByText(
        "The Camunda minors of the users can be incomplete while the fleet manager cannot read CamundaCluster.",
      ),
    ).toBeDefined();
    expect(
      within(
        screen.getByRole("list", { name: "Defaults of Web Modeler REST API" }),
      )
        .getAllByRole("listitem")
        .map((li) => li.textContent),
    ).toEqual(["camunda/web-modeler-restapi"]);
  });

  it("calls the config unused while only a preset or a release cannot be read", () => {
    setup({
      name: "basic-lab",
      errors: [{ kind: "CamundaRelease", message: "forbidden" }],
      stale: ["CamundaClusterPreset"],
    });

    expect(
      screen.getByText(
        "No cluster or management plane uses this config. The defaults are those of Camunda 8.10, the newest in the images table.",
      ),
    ).toBeDefined();
    expect(screen.queryByText(/can be incomplete/)).toBeNull();
  });

  it("does not say that the minors can be incomplete while every kind that feeds them is read", () => {
    setup({ stale: ["Namespace", "DatabaseServer"] });

    expect(screen.queryByText(/can be incomplete/)).toBeNull();
  });
});

describe("PlatformDetail over the empty values of another tool", () => {
  it("shows no row for empty management clients when the edit picks and clears a client secret", async () => {
    const sources = platformFixtures();
    const config = sources.platformConfigs.find(
      (c) => c.metadata.name === "oidc-prod",
    )!;
    config.spec.auth!.oidc!.management = { clients: {} };
    config.metadata.managedFields = [
      {
        manager: "kubectl-client-side-apply",
        operation: "Update",
        fieldsV1: {
          "f:spec": {
            "f:auth": {
              "f:oidc": {
                "f:issuerUrl": {},
                "f:management": { "f:clients": {} },
              },
            },
          },
        },
      },
    ];
    const { actions } = setup({ sources });
    const secret = within(
      screen.getByRole("group", { name: "Management Identity" }),
    ).getByRole("group", { name: "Client secret" });
    const namespace = within(secret).getByLabelText("Namespace");
    fireEvent.change(namespace, { target: { value: "camunda-system" } });
    fireEvent.change(within(secret).getByLabelText("Secret"), {
      target: { value: "oidc-credentials" },
    });
    fireEvent.change(within(secret).getByLabelText("Key"), {
      target: { value: "identity-secret" },
    });
    fireEvent.change(namespace, { target: { value: "" } });
    fireEvent.change(input("Issuer URL"), {
      target: { value: "https://login.example.com/realms/other" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Review" }));
    await waitFor(() => expect(actions.previewChange).toHaveBeenCalled());
    const change = actions.previewChange.mock.lastCall![0];
    const { changed, cleared } = foreignFields(
      config as unknown as KubeObject,
      change.apply[0],
      change.keep?.["CamundaPlatformConfig//oidc-prod"],
    );
    expect(changed.map((f) => f.path)).toEqual(["spec.auth.oidc.issuerUrl"]);
    expect(cleared).toEqual([]);
  });
});
