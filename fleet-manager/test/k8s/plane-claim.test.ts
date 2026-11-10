import { randomUUID } from "node:crypto";
import { KubeConfig, KubernetesObjectApi } from "@kubernetes/client-node";
import { beforeAll, describe, expect, inject, it } from "vitest";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import { EMPTY_ADMIN, EMPTY_MAIL } from "@/lib/planes/components";
import {
  EMPTY_PLANE_DRAFT,
  type PlaneContext,
  type PlaneDraft,
  planeClaimOf,
} from "@/lib/planes/draft";
import { directWriter } from "@/lib/write/writer";
import { parseImagesTable } from "@/startup/bundled-data";

const kc = new KubeConfig();
kc.loadFromFile(inject("kubeconfig"));
const api = KubernetesObjectApi.makeApiClient(kc);
const writer = directWriter(kc);
const namespace = `plane-${randomUUID().slice(0, 8)}`;

const ctx: PlaneContext = {
  environments: [
    {
      metadata: { name: "prod" },
      spec: { releases: { default: "camunda-8-9-23" } },
    } as Environment,
  ],
  platformConfigs: [],
  takenNames: [],
  namespaces: [{ name: namespace, environment: "prod" }],
  releases: [
    {
      metadata: { name: "camunda-8-9-23" },
      spec: { version: "8.9.23" },
    } as CamundaRelease,
  ],
  images: parseImagesTable({
    schemaVersion: 1,
    minors: {
      "8.9": {
        identity: { version: "8.9.10" },
        console: { version: "8.9.122" },
        "web-modeler-restapi": { version: "8.9.11" },
        keycloak: { tagPrefix: "quay-optimized-", version: "26.6.4" },
      },
    },
  }),
};

const base = (name: string): PlaneDraft => ({
  ...EMPTY_PLANE_DRAFT,
  environment: "prod",
  name,
  namespace: { kind: "existing", name: namespace },
});

const drafts: [string, PlaneDraft][] = [
  [
    "Keycloak with Console and Web Modeler",
    {
      ...base("plane-keycloak"),
      mode: "keycloak",
      admin: { ...EMPTY_ADMIN, username: "admin", email: "a@example.com" },
      console: true,
      webModeler: true,
      mail: {
        ...EMPTY_MAIL,
        smtpHost: "smtp.example.com",
        fromAddress: "modeler@example.com",
        credentialsSecret: "smtp",
      },
    },
  ],
  [
    "OIDC",
    {
      ...base("plane-oidc"),
      mode: "oidc",
      admin: { ...EMPTY_ADMIN, claimName: "sub", claimValue: "42" },
    },
  ],
  [
    "External Keycloak",
    {
      ...base("plane-external"),
      mode: "externalKeycloak",
      externalKeycloak: {
        url: "https://keycloak.example.com/auth",
        realm: "camunda-platform",
        adminSecret: "kc-admin",
      },
      admin: {
        ...EMPTY_ADMIN,
        username: "admin",
        password: { name: "pw", key: "password" },
      },
    },
  ],
];

beforeAll(async () => {
  await api.create({
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name: namespace },
  });
});

describe("a plane claim of each identity mode passes the CRD rules of the API server", () => {
  it.each(drafts)("%s", async (_, draft) => {
    const preview = await writer.preview({
      intent: "x",
      apply: [planeClaimOf(draft, ctx)],
    });
    expect(preview.errors).toEqual([]);
  });
});
