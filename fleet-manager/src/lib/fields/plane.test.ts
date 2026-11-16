import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import {
  IDENTITY_INITIAL_CLAIM_ANNOTATION,
  pathFrom,
  planeFields,
  type PlaneField,
} from ".";

type Claim = Resource<"CamundaManagementClusterClaim">;

const DATABASES =
  "spec.plugins[section=management-database,registration=existing-database-configs]";

function claimOf(
  spec: Partial<Claim["spec"]>,
  status?: Claim["status"],
): Claim {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { name: "plane-a", namespace: "platform", resourceVersion: "7" },
    spec: {
      environments: ["platform"],
      versions: {
        identity: "8.9.14",
        console: "8.9.133",
        webModeler: "8.9.15",
      },
      plugins: [
        {
          section: "management-database",
          registration: "existing-database-configs",
          parameters: { identity: "identity-db", webModeler: "modeler-db" },
        },
      ],
      ...spec,
      management: {
        identityProvider: { oidc: {} },
        identity: {
          externalUrl: "https://management.example.com/identity",
          admin: { claimName: "email", claimValue: "admin@example.com" },
        },
        console: { externalUrl: "https://management.example.com/console" },
        webModeler: {
          externalUrl: "https://management.example.com/modeler",
          websocketsExternalUrl: "https://management.example.com/modeler-ws",
          mail: {
            fromAddress: "modeler@example.com",
            smtpHost: "smtp.example.com",
          },
        },
        ...spec.management,
      },
    } as Claim["spec"],
    ...(status && { status }),
  } as Claim;
}

function planeOf(
  annotations?: Record<string, string>,
): Resource<"CamundaManagementCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: {
      name: "plane-a",
      namespace: "platform",
      labels: { [CLAIM_LABEL]: "plane-a" },
      ...(annotations && { annotations }),
    },
    spec: {},
  } as unknown as Resource<"CamundaManagementCluster">;
}

const registrations = [
  {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name: "existing-database-configs" },
    spec: {
      section: "management-database",
      deletionPolicies: ["Delete", "Orphan"],
      parameterValues: {},
      parameters: {
        type: "object",
        properties: {
          keycloak: { type: "string" },
          identity: { type: "string" },
          webModeler: { type: "string" },
        },
      },
    },
  },
] as unknown as PluginRegistration[];

const environment = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "platform" },
  spec: {
    platformConfigRef: "oidc-platform",
    parameters: {
      "existing-database-configs": { identity: { claimEditable: true } },
    },
  },
} as unknown as Environment;

const oidcClaim = claimOf({});
const keycloakClaim = claimOf({
  management: {
    identityProvider: {
      keycloak: { externalUrl: "https://management.example.com/auth" },
    },
    identity: {
      externalUrl: "https://management.example.com/identity",
      admin: { username: "admin", email: "admin@example.com" },
    },
    webModeler: undefined,
  } as Claim["spec"]["management"],
  versions: { identity: "8.9.14", keycloak: "26.6.4" },
});
const oidcClaimWithoutConsole = claimOf({
  management: { console: undefined } as unknown as Claim["spec"]["management"],
});

function byKey(fields: readonly PlaneField[], key: string): PlaneField {
  const field = fields.find((f) => f.key === key);
  if (!field) throw new Error(`no field ${key}`);
  return field;
}

describe("planeFields", () => {
  it("IDENTITY_INITIAL_CLAIM_ANNOTATION matches InitialClaimAnnotation of the plane components", () => {
    const source = readFileSync(
      new URL(
        "../../../../pkg/components/camundamanagementcluster/names.go",
        import.meta.url,
      ),
      "utf8",
    );
    expect(IDENTITY_INITIAL_CLAIM_ANNOTATION).toBe(
      /InitialClaimAnnotation\s*=\s*"([^"]*)"/.exec(source)?.[1],
    );
  });

  it("locks the name and the namespace at creation", () => {
    const fields = planeFields({
      claim: oidcClaim,
      registrations,
      environment,
    });
    expect(byKey(fields, "name").lock).toEqual({ reason: "atCreation" });
    expect(byKey(fields, "namespace").lock).toEqual({ reason: "atCreation" });
  });

  it("leaves the platform config open and gives the one of the environment when the claim names none", () => {
    const fields = planeFields({
      claim: oidcClaim,
      registrations,
      environment,
    });
    const field = byKey(fields, "platformConfig");
    expect(field).toMatchObject({
      claimPath: "spec.platformConfigRef",
      value: "oidc-platform",
      note: "From the environment",
    });
    expect(field.lock).toBeUndefined();
  });

  it("gives the platform config of the claim, open and without a note, when the claim sets it", () => {
    const fields = planeFields({
      claim: claimOf({ platformConfigRef: "other" }),
      registrations,
      environment,
    });
    expect(byKey(fields, "platformConfig").value).toBe("other");
    expect(byKey(fields, "platformConfig").note).toBeUndefined();
    expect(byKey(fields, "platformConfig").lock).toBeUndefined();
  });

  it("locks the OIDC claim pair once Identity recorded it", () => {
    const fields = planeFields({
      claim: oidcClaim,
      plane: planeOf({
        [IDENTITY_INITIAL_CLAIM_ANNOTATION]: "email=admin@example.com",
      }),
      registrations,
      environment,
    });
    for (const key of ["admin.claimName", "admin.claimValue"]) {
      expect(byKey(fields, key)).toMatchObject({
        lock: { reason: "immutableAfterStart" },
        note: "Management Identity keeps the first administrator in its database, so a change here has no effect.",
      });
    }
  });

  it("leaves the OIDC claim pair open until Identity recorded it", () => {
    for (const plane of [planeOf(), undefined]) {
      const fields = planeFields({
        claim: oidcClaim,
        plane,
        registrations,
        environment,
      });
      expect(byKey(fields, "admin.claimName").lock).toBeUndefined();
      expect(byKey(fields, "admin.claimValue").lock).toBeUndefined();
      expect(byKey(fields, "admin.claimName").note).toBeUndefined();
    }
  });

  it("keeps username and email open in a Keycloak mode, with the note on the username", () => {
    const fields = planeFields({
      claim: keycloakClaim,
      plane: planeOf({ [IDENTITY_INITIAL_CLAIM_ANNOTATION]: "x=y" }),
      registrations,
      environment,
    });
    expect(byKey(fields, "admin.username")).toMatchObject({
      value: "admin",
      note: "A new username creates a second user.",
    });
    expect(byKey(fields, "admin.username").lock).toBeUndefined();
    expect(byKey(fields, "admin.email").lock).toBeUndefined();
  });

  it("gives the admin fields of the claim mode only", () => {
    const keys = (claim: Claim) =>
      planeFields({ claim, registrations, environment })
        .filter((f) => f.key.startsWith("admin."))
        .map((f) => f.key);
    expect(keys(oidcClaim)).toEqual(["admin.claimName", "admin.claimValue"]);
    expect(keys(keycloakClaim)).toEqual(["admin.username", "admin.email"]);
  });

  it("names the identity provider of the claim", () => {
    const mode = (claim: Claim) =>
      byKey(
        planeFields({ claim, registrations, environment }),
        "identityProvider",
      ).value;
    expect(mode(oidcClaim)).toBe("oidc");
    expect(mode(keycloakClaim)).toBe("keycloak");
    expect(
      mode(
        claimOf({
          management: {
            identityProvider: { externalKeycloak: {} },
          } as unknown as Claim["spec"]["management"],
        }),
      ),
    ).toBe("externalKeycloak");
  });

  it("gives Web Modeler mail settings no lock", () => {
    const mail = byKey(
      planeFields({ claim: oidcClaim, registrations, environment }),
      "webModeler.mail",
    );
    expect(mail.claimPath).toBe("spec.management.webModeler.mail");
    expect(mail.value).toEqual({
      fromAddress: "modeler@example.com",
      smtpHost: "smtp.example.com",
    });
    expect(mail.lock).toBeUndefined();
  });

  it("locks the option of a selection at creation and keeps its claim-editable parameters open", () => {
    const fields = planeFields({
      claim: oidcClaim,
      registrations,
      environment,
    });
    expect(
      byKey(fields, "plugins.management-database.existing-database-configs"),
    ).toMatchObject({
      group: "databases",
      claimPath: `${DATABASES}.registration`,
      value: "existing-database-configs",
      lock: { reason: "chosenAtCreation" },
    });
    expect(
      byKey(
        fields,
        "plugins.management-database.existing-database-configs.identity",
      ),
    ).toMatchObject({
      claimPath: `${DATABASES}.parameters.identity`,
      value: "identity-db",
    });
    expect(
      byKey(
        fields,
        "plugins.management-database.existing-database-configs.identity",
      ).lock,
    ).toBeUndefined();
    expect(
      byKey(
        fields,
        "plugins.management-database.existing-database-configs.keycloak",
      ).lock,
    ).toEqual({ reason: "admin" });
  });

  it("shows the policy value of a parameter that the claim cannot edit, not the value that the claim sets", () => {
    const fixed = {
      ...environment,
      spec: {
        ...environment.spec,
        parameters: {
          "existing-database-configs": {
            identity: { claimEditable: true },
            webModeler: { value: "admin-db" },
          },
        },
      },
    } as unknown as Environment;
    const fields = planeFields({
      claim: oidcClaim,
      registrations,
      environment: fixed,
    });
    const parameter = "plugins.management-database.existing-database-configs";
    expect(byKey(fields, `${parameter}.webModeler`)).toMatchObject({
      value: "admin-db",
      lock: { reason: "admin" },
    });
    expect(byKey(fields, `${parameter}.identity`).value).toBe("identity-db");
  });

  it("reads the recorded policy of a parameter before the environment", () => {
    const claim = claimOf({}, {
      resolvedPolicy: {
        parameters: {
          "existing-database-configs": { keycloak: { claimEditable: true } },
        },
      },
    } as Claim["status"]);
    const fields = planeFields({ claim, registrations, environment });
    const parameter = "plugins.management-database.existing-database-configs";
    expect(byKey(fields, `${parameter}.keycloak`).lock).toBeUndefined();
    expect(byKey(fields, `${parameter}.identity`).lock).toEqual({
      reason: "admin",
    });
  });

  it("locks the typed URLs while the claim has an access selection", () => {
    const access = claimOf({
      plugins: [
        ...(oidcClaim.spec.plugins ?? []),
        { section: "access", registration: "gateway-access" },
      ],
    });
    const locked = planeFields({ claim: access, registrations, environment });
    expect(byKey(locked, "url.identity")).toMatchObject({
      group: "access",
      claimPath: "spec.management.identity.externalUrl",
      lock: { reason: "section" },
    });
    expect(byKey(locked, "plugins.access.gateway-access")).toMatchObject({
      group: "access",
      lock: { reason: "chosenAtCreation" },
    });
    const open = planeFields({ claim: oidcClaim, registrations, environment });
    expect(byKey(open, "url.identity")).toMatchObject({
      value: "https://management.example.com/identity",
    });
    expect(byKey(open, "url.identity").lock).toBeUndefined();
  });

  it("gives versions and URLs only for the apps that run", () => {
    const keys = planeFields({
      claim: oidcClaimWithoutConsole,
      registrations,
      environment,
    }).map((f) => f.key);
    expect(keys).not.toContain("versions.console");
    expect(keys).not.toContain("url.console");
    expect(keys).not.toContain("versions.keycloak");
    expect(keys).not.toContain("url.keycloak");
    expect(keys).toContain("versions.identity");
    expect(keys).toContain("url.webModelerWebsockets");
    const keycloak = planeFields({
      claim: keycloakClaim,
      registrations,
      environment,
    }).map((f) => f.key);
    expect(keycloak).toContain("versions.keycloak");
    expect(keycloak).toContain("url.keycloak");
    expect(keycloak).not.toContain("webModeler.mail");
    expect(keycloak).not.toContain("url.webModeler");
  });

  it("gives the deletion policy of the claim, else the recorded default, else Delete", () => {
    const policy = (claim: Claim) =>
      byKey(
        planeFields({ claim, registrations, environment }),
        "deletionPolicy",
      ).value;
    expect(policy(claimOf({ deletionPolicy: "Orphan" }))).toBe("Orphan");
    expect(
      policy(
        claimOf({}, {
          resolvedPolicy: { deletionPolicy: "Orphan" },
        } as Claim["status"]),
      ),
    ).toBe("Orphan");
    expect(policy(oidcClaim)).toBe("Delete");
  });

  it("lists the groups in their order", () => {
    const groups = [
      ...new Set(
        planeFields({ claim: oidcClaim, registrations, environment }).map(
          (f) => f.group,
        ),
      ),
    ];
    expect(groups).toEqual([
      "basics",
      "identity",
      "environments",
      "components",
      "versions",
      "databases",
      "access",
      "deletion",
    ]);
  });

  it("gives each key once", () => {
    const keys = planeFields({
      claim: oidcClaim,
      registrations,
      environment,
    }).map((f) => f.key);
    expect(new Set(keys).size).toBe(keys.length);
  });

  it("each claimPath is the path of its claimSteps", () => {
    for (const claim of [oidcClaim, keycloakClaim]) {
      for (const f of planeFields({ claim, registrations, environment })) {
        expect(pathFrom("", f.claimSteps)).toBe(f.claimPath);
      }
    }
  });
});
