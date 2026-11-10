// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, describe, expect, it } from "vitest";
import type { DatabaseConfig } from "@/generated/crds/databaseconfig";
import type { Environment } from "@/generated/crds/environment";
import type {
  PluginRegistration,
  PluginRegistrationSpec,
} from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { useCreateFlow } from "@/components/create/use-create-flow";
import { EMPTY_ADMIN } from "@/lib/planes/components";
import {
  EMPTY_PLANE_DRAFT,
  type PlaneContext,
  type PlaneDraft,
} from "@/lib/planes/draft";
import { storageErrors } from "@/lib/planes/storage";
import type { ValidationError } from "@/lib/schema";
import { StorageAccessStep } from "./storage-access-step";

afterEach(cleanup);

const section = (
  name: string,
  label: string,
  spec: Partial<PluginSection["spec"]>,
): PluginSection => ({
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginSection",
  metadata: { name },
  spec: {
    appliesTo: ["CamundaManagementClusterClaim"],
    label,
    selection: "One",
    ...spec,
  },
});

const registration = (
  name: string,
  label: string,
  spec: Partial<PluginRegistrationSpec>,
): PluginRegistration => ({
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginRegistration",
  metadata: { name },
  spec: {
    sections: [],
    mode: "Optional",
    label,
    deletionPolicies: ["Delete"],
    serviceAccount: { name, namespace: "plugins" },
    ...spec,
  },
  status: {
    conditions: ["Accepted", "Ready"].map((type) => ({
      type,
      status: "True" as const,
      reason: "Test",
      message: "",
      lastTransitionTime: "2026-11-10T00:00:00Z",
    })),
  },
});

const builtin = { name: "camunda-operator", version: "0.0.0" };
const app = (title: string) => ({ type: "string", title });

const sections = [
  section("management-database", "Databases", {
    slot: "managementDatabases",
    order: 80,
  }),
  section("access", "Access", {
    slot: "externalAccess",
    order: 60,
    emptyOptionLabel: "No access from outside",
  }),
];

const registrations = [
  registration("management-database-server", "New PostgreSQL server", {
    sections: ["management-database"],
    controller: builtin,
    parameters: {
      type: "object",
      properties: {
        preset: { type: "string" },
        databases: { type: "array", items: { type: "string" } },
      },
    },
  }),
  registration("existing-database-configs", "Existing databases", {
    sections: ["management-database"],
    controller: builtin,
    parameters: {
      type: "object",
      properties: {
        keycloak: app("Keycloak"),
        identity: app("Management Identity"),
        webModeler: app("Web Modeler"),
      },
    },
  }),
  registration("custom-database", "Custom database", {
    sections: ["management-database"],
  }),
  registration("gateway", "Gateway", {
    sections: ["access"],
    parameterValues: {
      managementHostnamePattern: "{name}.{namespace}.example.com",
    },
  }),
];

const environment = (access: boolean): Environment => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "production" },
  spec: {
    sections: {
      "management-database": {
        required: true,
        allowed: ["existing-database-configs", "management-database-server"],
        default: "existing-database-configs",
      },
      access: access ? { allowed: ["gateway"] } : { allowed: [] },
    },
    parameters: {
      "existing-database-configs": {
        keycloak: { claimEditable: true },
        identity: { claimEditable: true },
        webModeler: { claimEditable: true },
      },
    },
  },
});

const config = (name: string): DatabaseConfig =>
  ({
    metadata: { namespace: "platform", name },
    spec: {},
  }) as unknown as DatabaseConfig;
const objects = {
  DatabaseConfig: [config("keycloak-db"), config("identity-db")],
};

const ctxOf = (access: boolean): PlaneContext => ({
  environments: [environment(access)],
  platformConfigs: [],
  takenNames: [],
  namespaces: [{ name: "platform", environment: "production" }],
  releases: [],
  images: { schemaVersion: 1, minors: {} },
  sections,
  registrations,
  accessRequests: [],
  authConfigs: [],
  planeClaims: [],
});

const keycloak: PlaneDraft = {
  ...EMPTY_PLANE_DRAFT,
  environment: "production",
  name: "plane-a",
  namespace: { kind: "existing", name: "platform" },
  mode: "keycloak",
  admin: { ...EMPTY_ADMIN, username: "admin" },
};

function Harness({
  initial = keycloak,
  access = false,
}: {
  initial?: PlaneDraft;
  access?: boolean;
}) {
  const ctx = ctxOf(access);
  const flow = useCreateFlow<PlaneDraft>({
    steps: [{ id: "storage", label: "Storage and access" }],
    initial,
  });
  const [errors, setErrors] = useState<Record<string, ValidationError[]>>({});
  return (
    <>
      <StorageAccessStep
        draft={flow.draft}
        set={flow.set}
        ctx={ctx}
        objects={objects}
        onSectionErrors={(name, e) =>
          setErrors((current) => ({ ...current, [name]: e }))
        }
      />
      <ul aria-label="Step errors">
        {storageErrors(flow.draft, ctx, Object.values(errors).flat()).map(
          (e) => (
            <li key={e}>{e}</li>
          ),
        )}
      </ul>
    </>
  );
}

const stepErrors = () =>
  within(screen.getByRole("list", { name: "Step errors" }))
    .queryAllByRole("listitem")
    .map((li) => li.textContent);
const databases = () => screen.getByRole("group", { name: /Databases/ });

describe("StorageAccessStep", () => {
  it("offers only the allowed database options of the environment, with the Default label of the policy", () => {
    render(<Harness />);
    const options = within(databases()).getAllByRole("radio");
    expect(options.map((o) => o.closest("label")?.textContent)).toEqual([
      expect.stringContaining("Existing databases"),
      expect.stringContaining("New PostgreSQL server"),
    ]);
    expect(options[0].closest("label")?.textContent).toContain("Default");
    expect(within(databases()).queryByText("Custom database")).toBeNull();
  });

  it("names what the new PostgreSQL server makes for each app that runs", async () => {
    render(<Harness />);
    await userEvent.click(
      within(databases()).getByRole("radio", { name: /New PostgreSQL server/ }),
    );
    expect(
      screen.getByText(
        "Creates one DatabaseServer plane-a-db and one Database for each app that runs: plane-a-keycloak and plane-a-identity.",
      ),
    ).toBeTruthy();
  });

  it("with an access option the hostname is locked with the pattern note and the URLs end in /auth, /identity, /console, /modeler, and /modeler-ws", async () => {
    render(
      <Harness
        access
        initial={{ ...keycloak, console: true, webModeler: true }}
      />,
    );
    await userEvent.click(screen.getByRole("radio", { name: /Gateway/ }));
    const host = screen.getByRole("group", { name: "Hostname" });
    expect(host.textContent).toContain("plane-a.platform.example.com");
    expect(host.textContent).toContain(
      "From the pattern {name}.{namespace}.example.com.",
    );
    const urls = within(screen.getByRole("list", { name: "URLs" }))
      .getAllByRole("listitem")
      .map((li) => li.textContent);
    expect(urls).toEqual([
      "Keycloakhttps://plane-a.platform.example.com/auth",
      "Management Identityhttps://plane-a.platform.example.com/identity",
      "Consolehttps://plane-a.platform.example.com/console",
      "Web Modelerhttps://plane-a.platform.example.com/modeler",
      "Web Modeler websocketshttps://plane-a.platform.example.com/modeler-ws",
    ]);
    expect(
      screen.getByText("Paths are fixed. Each app serves under its path."),
    ).toBeTruthy();
    expect(screen.queryByLabelText("Keycloak URL")).toBeNull();
  });

  it("shows only the URLs of the apps that run", async () => {
    render(<Harness access initial={{ ...keycloak, mode: "oidc" }} />);
    await userEvent.click(screen.getByRole("radio", { name: /Gateway/ }));
    expect(
      within(screen.getByRole("list", { name: "URLs" }))
        .getAllByRole("listitem")
        .map((li) => li.textContent),
    ).toEqual([
      "Management Identityhttps://plane-a.platform.example.com/identity",
    ]);
  });

  it("No access from outside shows one empty required URL field per running app with a placeholder", () => {
    render(<Harness initial={{ ...keycloak, console: true }} />);
    const fields = [
      "Keycloak URL",
      "Management Identity URL",
      "Console URL",
    ].map((label) => screen.getByLabelText(label) as HTMLInputElement);
    expect(fields.map((f) => [f.value, f.required, f.placeholder])).toEqual([
      ["", true, "https://management.example.com/auth"],
      ["", true, "https://management.example.com/identity"],
      ["", true, "https://management.example.com/console"],
    ]);
    expect(screen.queryByLabelText("Web Modeler URL")).toBeNull();
    expect(stepErrors()).toContain("Enter the Keycloak URL.");
  });

  it("keeps the typed URLs across an access round trip", async () => {
    render(<Harness access />);
    await userEvent.type(
      screen.getByLabelText("Keycloak URL"),
      "https://m.example.com/auth",
    );
    await userEvent.click(screen.getByRole("radio", { name: /Gateway/ }));
    await userEvent.click(
      screen.getByRole("radio", { name: /No access from outside/ }),
    );
    expect(
      (screen.getByLabelText("Keycloak URL") as HTMLInputElement).value,
    ).toBe("https://m.example.com/auth");
  });

  it("existing database configs refuses two equal names before the save", async () => {
    render(<Harness />);
    await userEvent.selectOptions(
      within(databases()).getByRole("combobox", { name: "Keycloak" }),
      "identity-db",
    );
    await userEvent.selectOptions(
      within(databases()).getByRole("combobox", {
        name: "Management Identity",
      }),
      "identity-db",
    );
    expect(stepErrors()).toContain(
      "Pick a different DatabaseConfig for each app. Keycloak and Management Identity both use identity-db.",
    );
  });

  it("a component that is off gets no database field", () => {
    render(<Harness initial={{ ...keycloak, mode: "oidc" }} />);
    expect(
      within(databases()).queryByRole("combobox", { name: "Keycloak" }),
    ).toBeNull();
    expect(
      within(databases()).queryByRole("combobox", { name: "Web Modeler" }),
    ).toBeNull();
    expect(
      within(databases()).getByRole("combobox", {
        name: "Management Identity",
      }),
    ).toBeTruthy();
  });

  it("takes a contract name, with the claim name as its placeholder", async () => {
    render(<Harness />);
    const field = screen.getByLabelText("Contract name") as HTMLInputElement;
    expect(field.placeholder).toBe("plane-a");
    await userEvent.type(field, "Ops");
    expect(stepErrors()).toContain(
      "The contract name must be lower case letters, digits, and dashes.",
    );
  });
});
