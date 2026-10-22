import { randomUUID } from "node:crypto";
import {
  KubeConfig,
  KubernetesObjectApi,
  type V1ConfigMap,
} from "@kubernetes/client-node";
import { beforeAll, describe, expect, inject, it } from "vitest";
import { getSettings, updateSettings } from "@/lib/settings";

// The settings read KUBECONFIG through kubeConfig, on their first call.
process.env.KUBECONFIG = inject("kubeconfig");

const kc = new KubeConfig();
kc.loadFromFile(inject("kubeconfig"));
const api = KubernetesObjectApi.makeApiClient(kc);

let namespace = "";

// Each test has its own namespace, because the first read of the settings
// creates the ConfigMap once per namespace.
async function useNewNamespace(): Promise<void> {
  namespace = `settings-${randomUUID().slice(0, 8)}`;
  await api.create({
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name: namespace },
  });
  process.env.FLEET_NAMESPACE = namespace;
}

function setSeed(values: Record<string, string>): void {
  for (const name of Object.keys(process.env)) {
    if (name.startsWith("FLEET_SETTINGS_")) delete process.env[name];
  }
  Object.assign(process.env, values);
}

function readConfigMap(): Promise<V1ConfigMap> {
  return api.read<V1ConfigMap>({
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: { name: "fleet-manager-settings", namespace },
  });
}

// An admin edits the ConfigMap by hand, as kubectl edit does.
async function editByHand(data: Record<string, string>): Promise<void> {
  const live = await readConfigMap();
  live.data = { ...live.data, ...data };
  await api.replace(live, undefined, undefined, "kubectl-edit");
}

beforeAll(() => {
  setSeed({});
});

describe("settings against envtest", () => {
  it("a first start creates the ConfigMap from the environment variables", async () => {
    await useNewNamespace();
    setSeed({
      FLEET_SETTINGS_KUBERNETES_CLUSTER_NAME: "prod-eu",
      FLEET_SETTINGS_GIT_BRANCH: "main",
    });

    const settings = await getSettings();

    expect(settings.kubernetesClusterName).toBe("prod-eu");
    expect(settings["git.branch"]).toBe("main");
    expect((await readConfigMap()).data).toEqual(
      expect.objectContaining({
        kubernetesClusterName: "prod-eu",
        "git.branch": "main",
      }),
    );
  });

  it("a second start with other variables leaves the ConfigMap unchanged", async () => {
    await useNewNamespace();
    setSeed({ FLEET_SETTINGS_KUBERNETES_CLUSTER_NAME: "prod-eu" });
    await getSettings();
    const before = await readConfigMap();

    setSeed({ FLEET_SETTINGS_KUBERNETES_CLUSTER_NAME: "other" });

    expect((await getSettings()).kubernetesClusterName).toBe("prod-eu");
    const after = await readConfigMap();
    expect(after.data).toEqual(before.data);
    expect(after.metadata?.resourceVersion).toBe(
      before.metadata?.resourceVersion,
    );
  });

  it("two first reads at once create one ConfigMap", async () => {
    await useNewNamespace();
    setSeed({ FLEET_SETTINGS_KUBERNETES_CLUSTER_NAME: "prod-eu" });

    const [a, b] = await Promise.all([getSettings(), getSettings()]);

    expect(a).toEqual(b);
    expect(a.kubernetesClusterName).toBe("prod-eu");
  });

  it("updateSettings changes one key and keeps the others", async () => {
    await useNewNamespace();
    setSeed({
      FLEET_SETTINGS_KUBERNETES_CLUSTER_NAME: "prod-eu",
      FLEET_SETTINGS_GIT_BRANCH: "main",
    });
    await getSettings();
    await editByHand({ "custom.note": "kept" });

    expect(
      await updateSettings({ "prometheus.url": "http://prom:9090" }),
    ).toEqual({ ok: true, mode: "direct" });

    const data = (await readConfigMap()).data ?? {};
    expect(data["prometheus.url"]).toBe("http://prom:9090");
    expect(data.kubernetesClusterName).toBe("prod-eu");
    expect(data["git.branch"]).toBe("main");
    expect(data["custom.note"]).toBe("kept");
  });

  it("a second update keeps the key of the first update", async () => {
    await useNewNamespace();
    await getSettings();

    await updateSettings({ "git.path": "fleet" });
    await updateSettings({ "git.branch": "main" });

    const data = (await readConfigMap()).data ?? {};
    expect(data["git.path"]).toBe("fleet");
    expect(data["git.branch"]).toBe("main");
  });

  it("updateSettings changes a key that an admin set by hand", async () => {
    await useNewNamespace();
    await getSettings();
    await editByHand({ "git.branch": "release" });

    expect(await updateSettings({ "git.branch": "main" })).toEqual({
      ok: true,
      mode: "direct",
    });
    expect((await readConfigMap()).data?.["git.branch"]).toBe("main");
  });

  it("updateSettings before the first read creates the ConfigMap with the patch", async () => {
    await useNewNamespace();
    setSeed({ FLEET_SETTINGS_KUBERNETES_CLUSTER_NAME: "prod-eu" });

    await updateSettings({ "git.path": "fleet" });

    const data = (await readConfigMap()).data ?? {};
    expect(data.kubernetesClusterName).toBe("prod-eu");
    expect(data["git.path"]).toBe("fleet");
  });

  it("an invalid patch changes nothing", async () => {
    await useNewNamespace();
    await getSettings();
    const before = await readConfigMap();

    expect(await updateSettings({ "secretStore.kind": "Vault" })).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Invalid" })],
    });
    expect((await readConfigMap()).metadata?.resourceVersion).toBe(
      before.metadata?.resourceVersion,
    );
  });

  it("with git.repository set, updateSettings still writes the ConfigMap", async () => {
    await useNewNamespace();
    setSeed({
      FLEET_SETTINGS_GIT_REPOSITORY: "https://git.example.com/fleet.git",
    });
    await getSettings();

    expect(await updateSettings({ "git.path": "fleet" })).toEqual({
      ok: true,
      mode: "direct",
    });
    expect((await readConfigMap()).data?.["git.path"]).toBe("fleet");
  });
});
