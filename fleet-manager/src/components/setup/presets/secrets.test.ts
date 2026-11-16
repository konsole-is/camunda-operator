import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { presetFixtures } from "./fixtures";
import { missingSecrets, secretNamespaces } from "./secrets";

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

  it("is empty when nothing uses the preset and no environment allows it", () => {
    const sources = presetFixtures();
    expect(secretNamespaces(sources.clusterPresets[1], sources)).toEqual([]);
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

  it("lists nothing for a body without Secret keys", () => {
    expect(missingSecrets({ replicas: 3 }, () => ["team-b"])).toEqual([]);
    expect(missingSecrets(undefined, () => ["team-b"])).toEqual([]);
  });
});
