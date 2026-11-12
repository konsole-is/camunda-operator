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
  planeClaimOf,
} from "@/lib/planes/draft";
import { planeSectionViews, storageErrors } from "@/lib/planes/storage";
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
  ctx = ctxOf(access),
}: {
  initial?: PlaneDraft;
  access?: boolean;
  ctx?: PlaneContext;
}) {
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
        {storageErrors(
          flow.draft,
          ctx,
          errors,
          planeSectionViews(flow.draft, ctx, objects),
          objects,
        ).map((e) => (
          <li key={e}>{e}</li>
        ))}
      </ul>
      <output aria-label="Claim deletion policy">
        {planeClaimOf(flow.draft, ctx).spec.deletionPolicy ?? "default"}
      </output>
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

  it("without an access section shows the URL fields, and the typed URLs clear the URL errors", async () => {
    const ctx = {
      ...ctxOf(false),
      sections: sections.filter((s) => s.metadata.name !== "access"),
    };
    render(<Harness ctx={ctx} />);
    expect(stepErrors()).toContain("Enter the Keycloak URL.");
    await userEvent.type(
      screen.getByLabelText("Keycloak URL"),
      "https://m.example.com/auth",
    );
    await userEvent.type(
      screen.getByLabelText("Management Identity URL"),
      "https://m.example.com/identity",
    );
    expect(stepErrors().filter((e) => e?.startsWith("Enter the"))).toEqual([]);
  });

  it("a pick of an access section that is gone can be removed, and then the URL fields show", async () => {
    const ctx = {
      ...ctxOf(false),
      sections: sections.filter((s) => s.metadata.name !== "access"),
    };
    render(
      <Harness
        ctx={ctx}
        initial={{
          ...keycloak,
          storage: { access: [{ section: "access", registration: "gateway" }] },
        }}
      />,
    );
    expect(stepErrors()).toContain("Choose another option for access.");
    expect(screen.queryByLabelText("Keycloak URL")).toBeNull();
    await userEvent.click(
      screen.getByRole("button", { name: "Remove the access pick" }),
    );
    expect(stepErrors()).not.toContain("Choose another option for access.");
    expect(screen.getByLabelText("Keycloak URL")).toBeTruthy();
  });

  it("shows each other section that applies to a plane after the access section", () => {
    const base = environment(true);
    const ctx: PlaneContext = {
      ...ctxOf(true),
      sections: [
        ...sections,
        section("dns-zone", "DNS zone", { order: 90 }),
        section("backup-storage", "Backup storage", {
          appliesTo: ["CamundaClusterClaim"],
        }),
      ],
      registrations: [
        ...registrations,
        registration("route53", "Route 53", { sections: ["dns-zone"] }),
      ],
      environments: [
        {
          ...base,
          spec: {
            ...base.spec,
            sections: {
              ...base.spec.sections,
              "dns-zone": { allowed: ["route53"] },
            },
          },
        },
      ],
    };
    render(<Harness ctx={ctx} />);
    const dns = screen.getByRole("group", { name: /DNS zone/ });
    expect(within(dns).getByRole("radio", { name: /Route 53/ })).toBeTruthy();
    const access = screen.getByRole("group", { name: /Access/ });
    expect(
      access.compareDocumentPosition(dns) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.queryByRole("group", { name: /Backup storage/ })).toBeNull();
  });

  it("offers to remove a pick of a section that no longer applies to a plane", () => {
    const ctx: PlaneContext = {
      ...ctxOf(false),
      sections: [
        ...sections,
        section("dns-zone", "DNS zone", { appliesTo: ["CamundaClusterClaim"] }),
      ],
    };
    render(
      <Harness
        ctx={ctx}
        initial={{
          ...keycloak,
          storage: {
            "dns-zone": [{ section: "dns-zone", registration: "route53" }],
          },
        }}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Remove the dns-zone pick" }),
    ).toBeTruthy();
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

describe("StorageAccessStep and the deletion choice", () => {
  const withDefault = (
    deletionPolicy: "Delete" | "Orphan",
    deletionPolicies: ("Delete" | "Orphan")[],
  ): PlaneContext => {
    const ctx = ctxOf(false);
    return {
      ...ctx,
      environments: ctx.environments.map((e) => ({
        ...e,
        spec: { ...e.spec, deletionPolicy },
      })),
      registrations: ctx.registrations.map((r) => ({
        ...r,
        spec: { ...r.spec, deletionPolicies },
      })),
    };
  };
  const server: PlaneDraft = {
    ...keycloak,
    storage: {
      "management-database": [
        {
          section: "management-database",
          registration: "management-database-server",
        },
      ],
    },
    urls: {
      keycloak: "https://m.example.com/auth",
      identity: "https://m.example.com/identity",
    },
  };
  const choice = () =>
    screen.getByRole("group", { name: "When the management plane is deleted" });
  const policy = () =>
    screen.getByRole("status", { name: "Claim deletion policy" }).textContent;

  it("lets the user keep the resources, and the claim carries the pick", async () => {
    render(
      <Harness
        initial={server}
        ctx={withDefault("Delete", ["Delete", "Orphan"])}
      />,
    );
    expect(policy()).toBe("default");

    await userEvent.click(
      within(choice()).getByRole("radio", { name: /Keep the resources/ }),
    );

    expect(policy()).toBe("Orphan");
    expect(stepErrors()).toEqual([]);
  });

  it("shows the choice with Keep off when the default keeps and a pick supports only Delete, and the step passes with Delete", async () => {
    render(
      <Harness initial={server} ctx={withDefault("Orphan", ["Delete"])} />,
    );
    const keep = within(choice()).getByRole("radio", {
      name: /Keep the resources/,
    }) as HTMLInputElement;
    const remove = within(choice()).getByRole("radio", {
      name: /Delete the resources/,
    }) as HTMLInputElement;
    expect(keep.disabled).toBe(true);

    await userEvent.click(remove);

    expect(remove.checked).toBe(true);
    expect(policy()).toBe("Delete");
    expect(stepErrors()).toEqual([]);
  });
});
