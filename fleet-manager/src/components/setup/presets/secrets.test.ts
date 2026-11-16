import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { presetFixtures } from "./fixtures";
import { missingSecrets, secretNamespaces, secretsUnread } from "./secrets";

describe("secretNamespaces", () => {
  it("takes the namespaces of the users", () => {
    const sources = presetFixtures();
    // The PostgreSQL preset small is named by the plane claim platform/plane.
    expect(secretNamespaces(sources.databasePresets[0], sources)).toEqual([
      "platform",
    ]);
    // The cluster and the claim of medium are both in team-orders.
    expect(secretNamespaces(sources.clusterPresets[0], sources)).toEqual([
      "team-orders",
    ]);
  });

  it("takes the namespaces of the allowing environments when nothing uses the preset", () => {
    const sources = presetFixtures();
    sources.environments[0].spec.presets = { camundaCluster: ["spare"] };
    sources.namespaces.push({
      apiVersion: "v1",
      kind: "Namespace",
      metadata: {
        name: "team-billing",
        labels: { [ENVIRONMENT_LABEL]: "production" },
      },
    } as Resource<"Namespace">);
    expect(secretNamespaces(sources.clusterPresets[1], sources)).toEqual([
      "team-billing",
      "team-orders",
    ]);
  });

  it("takes no namespace while a list of the users cannot be read, not the allowing environments", () => {
    const sources = presetFixtures();
    sources.environments[0].spec.presets = { camundaCluster: ["medium"] };
    sources.clusters = [];
    sources.clusterClaims = [];
    expect(
      secretNamespaces(sources.clusterPresets[0], sources, [
        { kind: "CamundaCluster", message: "forbidden" },
      ]),
    ).toEqual([]);
  });

  it("is empty when nothing uses the preset and no environment allows it", () => {
    const sources = presetFixtures();
    expect(secretNamespaces(sources.clusterPresets[1], sources)).toEqual([]);
  });
});

describe("secretsUnread", () => {
  it("names each list of the users or the namespaces that cannot be read", () => {
    const errors = [
      { kind: "Namespace", message: "forbidden" },
      { kind: "CamundaCluster", message: "forbidden" },
      { kind: "DatabaseServer", message: "forbidden" },
    ];
    expect(secretsUnread("CamundaClusterPreset", errors)).toEqual([
      "CamundaCluster",
      "Namespace",
    ]);
  });

  it("names nothing when every list that it needs is read", () => {
    expect(
      secretsUnread("ElasticsearchClusterPreset", [
        { kind: "CamundaRelease", message: "forbidden" },
      ]),
    ).toEqual([]);
  });
});

describe("missingSecrets", () => {
  const missingIn = (name: string, key: string) =>
    name === "db-credentials" && key === "password" ? ["team-b"] : [];

  it("lists a Secret key that a user namespace lacks with those namespaces", () => {
    const body = {
      extraEnv: [
        {
          name: "DB_PASSWORD",
          valueFrom: {
            secretKeyRef: { name: "db-credentials", key: "password" },
          },
        },
        {
          name: "DB_USER",
          valueFrom: { secretKeyRef: { name: "db-credentials", key: "user" } },
        },
      ],
    };
    expect(missingSecrets(body, missingIn)).toEqual([
      {
        name: "db-credentials",
        key: "password",
        namespaces: ["team-b"],
        env: true,
      },
    ]);
  });

  it("lists a SecretRef field outside the env lists once", () => {
    const body = {
      auth: { clientSecretRef: { name: "db-credentials", key: "password" } },
      zeebe: {
        extraEnv: [
          {
            name: "A",
            valueFrom: {
              secretKeyRef: { name: "db-credentials", key: "password" },
            },
          },
        ],
      },
    };
    expect(missingSecrets(body, missingIn)).toEqual([
      {
        name: "db-credentials",
        key: "password",
        namespaces: ["team-b"],
        env: false,
      },
    ]);
  });

  it("marks a key as env only when every occurrence is in an env list", () => {
    const body = {
      zeebe: {
        extraEnv: [
          {
            name: "A",
            valueFrom: {
              secretKeyRef: { name: "db-credentials", key: "password" },
            },
          },
        ],
      },
      auth: { clientSecretRef: { name: "db-credentials", key: "password" } },
    };
    expect(missingSecrets(body, missingIn).map((m) => m.env)).toEqual([false]);
  });

  it("marks an env entry outside the env lists of the top level and the process blocks as not env", () => {
    const body = {
      backup: {
        dump: {
          extraEnv: [
            {
              name: "PGPASSWORD",
              valueFrom: {
                secretKeyRef: { name: "db-credentials", key: "password" },
              },
            },
          ],
        },
      },
    };
    expect(missingSecrets(body, missingIn)).toEqual([
      {
        name: "db-credentials",
        key: "password",
        namespaces: ["team-b"],
        env: false,
      },
    ]);
  });

  it("lists nothing for a body without Secret keys", () => {
    expect(missingSecrets({ replicas: 3 }, () => ["team-b"])).toEqual([]);
    expect(missingSecrets(undefined, () => ["team-b"])).toEqual([]);
  });
});
