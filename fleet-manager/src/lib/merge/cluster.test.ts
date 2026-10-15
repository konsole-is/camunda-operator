import { describe, expect, it } from "vitest";
import type { CamundaClusterPresetSpec } from "@/generated/crds/camundaclusterpreset";
import type { CamundaReleaseSpec } from "@/generated/crds/camundarelease";
import { mergeCluster } from "./cluster";

describe("mergeCluster", () => {
  it("gives each environment entry the layer that set it last", () => {
    const preset: CamundaClusterPresetSpec = {
      cluster: {
        extraEnv: [
          { name: "TZ", value: "UTC" },
          { name: "KEEP", value: "preset" },
        ],
        extraEnvFrom: [{ configMapRef: { name: "preset-env" } }],
      },
    };
    const release: CamundaReleaseSpec = {
      version: "8.9.4",
      extraEnv: [
        { name: "KEEP", value: "release" },
        { name: "RELEASE_ONLY", value: "release" },
      ],
      extraEnvFrom: [{ configMapRef: { name: "release-env" } }],
      zeebe: { extraEnv: [{ name: "JAVA_OPTS", value: "-Xmx1g" }] },
    };

    const { spec, source } = mergeCluster(preset, release, {
      extraEnv: [{ name: "KEEP", value: "cluster" }],
      extraEnvFrom: [{ secretRef: { name: "cluster-env" } }],
    });

    expect(spec).toEqual({
      version: "8.9.4",
      extraEnv: [
        { name: "TZ", value: "UTC" },
        { name: "KEEP", value: "cluster" },
        { name: "RELEASE_ONLY", value: "release" },
      ],
      extraEnvFrom: [
        { configMapRef: { name: "preset-env" } },
        { configMapRef: { name: "release-env" } },
        { secretRef: { name: "cluster-env" } },
      ],
      zeebe: { extraEnv: [{ name: "JAVA_OPTS", value: "-Xmx1g" }] },
    });
    expect(source).toEqual({
      "spec.version": "release",
      "spec.extraEnv[name=TZ]": "preset",
      "spec.extraEnv[name=KEEP]": "overrides",
      "spec.extraEnv[name=RELEASE_ONLY]": "release",
      "spec.extraEnvFrom[0]": "preset",
      "spec.extraEnvFrom[1]": "release",
      "spec.extraEnvFrom[2]": "overrides",
      "spec.zeebe.extraEnv[name=JAVA_OPTS]": "release",
    });
  });

  it("a cluster version replaces the release version", () => {
    const { spec, source } = mergeCluster(
      undefined,
      { version: "8.9.4", connectors: { version: "8.9.7" } },
      { version: "8.9.5" },
    );

    expect(spec).toEqual({
      version: "8.9.5",
      connectors: { version: "8.9.7" },
    });
    expect(source).toEqual({
      "spec.version": "overrides",
      "spec.connectors.version": "release",
    });
  });

  it("merges resources per entry, with a source for each entry", () => {
    const { spec, source } = mergeCluster(
      {
        cluster: {
          zeebe: { resources: { requests: { cpu: "1", memory: "2Gi" } } },
        },
      },
      undefined,
      { zeebe: { resources: { requests: { cpu: "2" } } } },
    );

    expect(spec.zeebe?.resources).toEqual({
      requests: { cpu: "2", memory: "2Gi" },
    });
    expect(source).toEqual({
      "spec.zeebe.resources.requests.cpu": "overrides",
      "spec.zeebe.resources.requests.memory": "preset",
    });
  });

  it("podLabels merge by key, and each key has its own source", () => {
    const { spec, source } = mergeCluster(
      {
        cluster: {
          podLabels: { "app.kubernetes.io/part-of": "camunda", team: "preset" },
        },
      },
      undefined,
      { podLabels: { team: "cluster" } },
    );

    expect(spec.podLabels).toEqual({
      "app.kubernetes.io/part-of": "camunda",
      team: "cluster",
    });
    expect(source).toEqual({
      "spec.podLabels.app.kubernetes.io/part-of": "preset",
      "spec.podLabels.team": "overrides",
    });
  });

  it("a scheduling block of the cluster replaces the preset block", () => {
    const { spec, source } = mergeCluster(
      {
        cluster: {
          scheduling: {
            nodeAffinity: {
              requiredDuringSchedulingIgnoredDuringExecution: {
                nodeSelectorTerms: [
                  {
                    matchExpressions: [
                      { key: "pool", operator: "In", values: ["a"] },
                    ],
                  },
                ],
              },
            },
            tolerations: [{ key: "pool", operator: "Exists" }],
          },
        },
      },
      undefined,
      {
        scheduling: { tolerations: [{ key: "dedicated", operator: "Exists" }] },
      },
    );

    expect(spec.scheduling).toEqual({
      tolerations: [{ key: "dedicated", operator: "Exists" }],
    });
    expect(source).toEqual({ "spec.scheduling": "overrides" });
  });

  it("a cluster client id replaces the whole client of the preset", () => {
    const preset: CamundaClusterPresetSpec = {
      cluster: {
        auth: {
          clientId: "preset-client",
          audience: "preset-audience",
          clientSecretRef: { name: "preset-secret", key: "secret" },
        },
      },
    };

    const { spec, source } = mergeCluster(preset, undefined, {
      auth: { clientId: "mine" },
    });

    expect(spec.auth).toEqual({ clientId: "mine" });
    expect(source).toEqual({ "spec.auth.clientId": "overrides" });
  });

  it("without a client id, the audience and the secret replace one by one", () => {
    const preset: CamundaClusterPresetSpec = {
      cluster: {
        auth: {
          clientId: "preset-client",
          clientSecretRef: { name: "preset-secret", key: "secret" },
        },
      },
    };

    const { spec, source } = mergeCluster(preset, undefined, {
      auth: { audience: "mine" },
    });

    expect(spec.auth).toEqual({
      clientId: "preset-client",
      audience: "mine",
      clientSecretRef: { name: "preset-secret", key: "secret" },
    });
    expect(source).toEqual({
      "spec.auth.clientId": "preset",
      "spec.auth.audience": "overrides",
      "spec.auth.clientSecretRef": "preset",
    });
  });

  it("takes the instance-bound fields from the cluster only", () => {
    const { spec, source } = mergeCluster(
      {
        cluster: {
          storageRef: "preset-storage",
          suspend: true,
          indexReplicas: 1,
        },
      },
      undefined,
      { storageRef: "mine" },
    );

    expect(spec).toEqual({ storageRef: "mine", indexReplicas: 1 });
    expect(source).toEqual({
      "spec.storageRef": "overrides",
      "spec.indexReplicas": "preset",
    });
  });

  it("does not change its inputs or share memory with them", () => {
    const freeze = <T>(value: T): T => {
      if (typeof value === "object" && value !== null) {
        Object.values(value).forEach(freeze);
        Object.freeze(value);
      }
      return value;
    };
    const preset = freeze<CamundaClusterPresetSpec>({
      cluster: {
        zeebe: { resources: { requests: { cpu: "1" } } },
        podLabels: { team: "a" },
      },
    });
    const release = freeze<CamundaReleaseSpec>({
      version: "8.9.4",
      extraEnv: [{ name: "A", value: "r" }],
    });

    const { spec } = mergeCluster(preset, release, freeze({}));

    expect(() => {
      spec.zeebe!.resources!.requests!.cpu = "changed";
      spec.podLabels!.team = "changed";
      spec.extraEnv![0].value = "changed";
    }).not.toThrow();
  });
});
