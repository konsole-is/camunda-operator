import { describe, expect, it } from "vitest";
import { externalSecretFor } from "./external-secret";

describe("externalSecretFor", () => {
  it("reads every property of the store entry <namespace>/<name> into the Secret", () => {
    expect(
      externalSecretFor(
        { namespace: "orders", name: "db" },
        { kind: "ClusterSecretStore", name: "vault" },
      ),
    ).toEqual({
      apiVersion: "external-secrets.io/v1",
      kind: "ExternalSecret",
      metadata: { name: "db", namespace: "orders" },
      spec: {
        refreshInterval: "1h",
        secretStoreRef: { kind: "ClusterSecretStore", name: "vault" },
        target: { name: "db", creationPolicy: "Owner" },
        dataFrom: [{ extract: { key: "orders/db" } }],
      },
    });
  });
});
