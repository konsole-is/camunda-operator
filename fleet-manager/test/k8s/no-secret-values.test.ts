import { randomUUID } from "node:crypto";
import { readFileSync, readdirSync } from "node:fs";
import http from "node:http";
import type { AddressInfo } from "node:net";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  KubeConfig,
  type KubernetesObject,
  KubernetesObjectApi,
  PatchStrategy,
} from "@kubernetes/client-node";
import { afterAll, beforeAll, describe, expect, inject, it, vi } from "vitest";
import { ping } from "@/app/dev/watch-probe/actions";
import {
  applyBackupChange,
  applyClaimChange,
  applyClaimDelete,
  previewClaimChange,
  previewClaimDelete,
} from "@/app/fleet/clusters/[namespace]/[name]/actions";
import {
  applyPlaneClaimChange,
  applyPlaneClaimDelete,
  previewPlaneClaimChange,
  previewPlaneClaimDelete,
} from "@/app/fleet/planes/[namespace]/[name]/actions";
import {
  createCluster,
  previewClusterCreate,
} from "@/app/fleet/clusters/new/actions";
import {
  createPlane,
  previewPlaneCreate,
} from "@/app/fleet/planes/new/actions";
import {
  applyContractChange,
  previewContractChange,
} from "@/app/setup/connections/actions";
import {
  applyEnvironmentChange,
  applyNamespaceLabel,
  previewEnvironmentChange,
  previewNamespaceLabel,
} from "@/app/setup/environments/actions";
import * as pluginLoads from "@/app/setup/plugins/load";
import {
  applyPlatformConfigChange,
  previewPlatformConfigChange,
} from "@/app/setup/platform/actions";
import {
  applyPresetChange,
  previewPresetChange,
} from "@/app/setup/presets/actions";
import {
  applyReleaseChange,
  applyReleaseMove,
  previewReleaseChange,
  previewReleaseMove,
} from "@/app/setup/releases/actions";
import {
  saveSettings,
  testGitConnection,
  testPrometheusQuery,
} from "@/app/setup/settings/actions";
import { GET as controllerPodsRoute } from "@/app/api/controller-pods/route";
import { GET as healthz } from "@/app/api/healthz/route";
import { GET as logsRoute } from "@/app/api/logs/route";
import { GET as mergedLogsRoute } from "@/app/api/logs/merged/route";
import { GET as metricsRoute } from "@/app/api/metrics/route";
import { GET as partitionsRoute } from "@/app/api/partitions/route";
import { GET as usageRoute } from "@/app/api/usage/route";
import { GET as warningsRoute } from "@/app/api/warnings/route";
import { GET as watchRoute } from "@/app/api/watch/route";
import { defaultCache } from "@/lib/k8s/cache";
import { get } from "@/lib/k8s/read";
import * as secrets from "@/lib/secrets";
import * as settings from "@/lib/settings";
import { createChangeOf } from "@/lib/write/create";
import type { ApiErrorCode, Preview, WriteResult } from "@/lib/write/types";
import { kubeRequest } from "../scenarios/kube";

// The data layer reads KUBECONFIG through kubeConfig, on its first call.
process.env.KUBECONFIG = inject("kubeconfig");

const kc = new KubeConfig();
kc.loadFromFile(inject("kubeconfig"));
const api = KubernetesObjectApi.makeApiClient(kc);
const namespace = `leak-${randomUUID().slice(0, 8)}`;
const src = fileURLToPath(new URL("../../src/", import.meta.url));

const values = {
  username: randomUUID(),
  password: randomUUID(),
  token: randomUUID(),
};
const extraValue = randomUUID();
const secretValues = [...Object.values(values), extraValue];

function assertNoValue(text: string): void {
  for (const value of secretValues) {
    expect(text).not.toContain(value);
    expect(text).not.toContain(Buffer.from(value).toString("base64"));
  }
}

// echo refuses each request and sends back what it got, as a careless
// server would, so testPrometheus sees the credentials in the answer. The
// address is http://, so testGit stops before a request: connections.test.ts
// covers the Git answer.
let echo: http.Server;
const echoed: string[] = [];

beforeAll(async () => {
  echo = http.createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on("data", (c: Buffer) => chunks.push(c));
    req.on("end", () => {
      echoed.push(req.url ?? "");
      res.writeHead(401, { "WWW-Authenticate": 'Basic realm="x"' });
      res.end(
        `${req.headers.authorization ?? ""} ${Buffer.concat(chunks).toString()}`,
      );
    });
  });
  await new Promise<void>((resolve) => echo.listen(0, "127.0.0.1", resolve));
  const url = `http://127.0.0.1:${(echo.address() as AddressInfo).port}`;

  await api.create({
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name: namespace },
  });
  await api.create({
    apiVersion: "v1",
    kind: "Secret",
    metadata: { name: "leak", namespace },
    stringData: values,
  });
  // A contract created by hand that reads its keys from the Secret leak.
  await api.create({
    apiVersion: "core.camunda.io/v1",
    kind: "ObjectStorageConfig",
    metadata: { name: "leak", namespace },
    spec: {
      type: "S3",
      s3: {
        bucketName: "leak",
        region: "eu-west-1",
        auth: {
          type: "credentials",
          credentials: {
            secretRef: {
              name: "leak",
              accessKeyIdKey: "username",
              secretAccessKeyKey: "password",
            },
          },
        },
      },
    },
  });
  await api.create({
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: { name: "leak", namespace },
    spec: { storageRef: "storage", platformConfigRef: "platform" },
  });
  // A version in the range of the backpressure query makes the warnings
  // route ask Prometheus.
  await kubeRequest(
    kc,
    "PATCH",
    `/apis/core.camunda.io/v1/namespaces/${namespace}/camundaclusters/leak/status`,
    {
      status: {
        management: {
          auth: { method: "none" },
          endpoint: "http://leak-gateway:9600",
          partitions: 1,
          version: "8.9.0",
        },
      },
    },
    "application/merge-patch+json",
  );
  await api.create({
    apiVersion: "v1",
    kind: "Pod",
    metadata: {
      name: "leak-zeebe-0",
      namespace,
      labels: { "camunda.io/cluster": "leak", "camunda.io/component": "zeebe" },
    },
    spec: { containers: [{ name: "zeebe", image: "camunda/zeebe" }] },
  });
  // The page of a registration reads the registration, its Grants, and its
  // events. The test writes no Grant and no event.
  await api.create({
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name: namespace },
    spec: {
      label: "Leak",
      sections: ["leak"],
      mode: "Optional",
      deletionPolicies: ["Delete"],
      serviceAccount: { name: "leak", namespace },
    },
  });
  process.env.FLEET_NAMESPACE = namespace;
  Object.assign(process.env, {
    FLEET_SETTINGS_GIT_REPOSITORY: `${url}/fleet.git`,
    FLEET_SETTINGS_GIT_CREDENTIALS_SECRET_REF: "leak",
    FLEET_SETTINGS_PROMETHEUS_URL: url,
    FLEET_SETTINGS_PROMETHEUS_CREDENTIALS_SECRET_REF: "leak",
  });
});
afterAll(async () => {
  // A registration is cluster-scoped, so it outlives the namespace of the
  // test and other suites of the same API server can list it.
  await api.delete({
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name: namespace },
  });
  await defaultCache().stop();
  await new Promise((resolve) => echo.close(resolve));
});

// watchText reads the stream of /api/watch until it holds the Secret leak,
// writes a key to it, reads until a frame names that key, and then stops.
async function watchText(watch: string): Promise<string> {
  const abort = new AbortController();
  const response = watchRoute(
    new Request(
      `http://localhost/api/watch?${new URLSearchParams({ watch })}`,
      {
        signal: abort.signal,
      },
    ),
  );
  expect(response.status).toBe(200);
  const reader = response
    .body!.pipeThrough(new TextDecoderStream())
    .getReader();
  let text = "";
  const readUntil = async (done: () => boolean) => {
    while (!done()) {
      const chunk = await reader.read();
      if (chunk.done) throw new Error(`The watch stream ended: ${text}`);
      text += chunk.value;
    }
  };
  await readUntil(() => text.includes('"leak"'));
  const seen = text.length;
  await api.patch(
    {
      apiVersion: "v1",
      kind: "Secret",
      metadata: { name: "leak", namespace },
      stringData: { extra: extraValue },
    },
    undefined,
    undefined,
    undefined,
    undefined,
    PatchStrategy.MergePatch,
  );
  await readUntil(() => text.slice(seen).includes('"extra"'));
  abort.abort();
  await reader.cancel().catch(() => {});
  return text;
}

const ROUTE_CALLS: Record<string, () => Promise<string>> = {
  "api/controller-pods": async () =>
    (
      await controllerPodsRoute(
        new Request(
          `http://localhost/api/controller-pods?registration=${namespace}-missing`,
        ),
      )
    ).text(),
  "api/healthz": async () => healthz().text(),
  // envtest runs no kubelet, so no pod has a log: the call reads a pod that
  // does not exist.
  "api/logs": async () =>
    (
      await logsRoute(
        new Request(
          `http://localhost/api/logs?namespace=${namespace}&pod=missing`,
        ),
      )
    ).text(),
  "api/logs/merged": async () =>
    (
      await mergedLogsRoute(
        new Request(
          `http://localhost/api/logs/merged?log=${namespace}%2Fmissing`,
        ),
      )
    ).text(),
  // envtest serves no metrics.k8s.io, so the usage is unavailable.
  "api/usage": async () =>
    (
      await usageRoute(
        new Request(
          `http://localhost/api/usage?namespace=${namespace}&selector=app%3Dx`,
        ),
      )
    ).text(),
  // The cluster has a broker pod, so the route asks Prometheus with the
  // credentials of the Secret and answers with the result.
  "api/metrics": async () => {
    const response = await metricsRoute(
      new Request(
        `http://localhost/api/metrics?namespace=${namespace}&cluster=leak&component=zeebe`,
      ),
    );
    expect(response.status).toBe(200);
    return response.text();
  },
  "api/partitions": async () =>
    (
      await partitionsRoute(
        new Request(
          `http://localhost/api/partitions?namespace=${namespace}&name=missing`,
        ),
      )
    ).text(),
  "api/warnings": async () => {
    await vi.waitFor(async () =>
      expect(
        (await get("CamundaCluster", namespace, "leak"))?.status?.management
          ?.version,
      ).toBe("8.9.0"),
    );
    const before = echoed.length;
    const text = await (await warningsRoute()).text();
    expect(
      echoed.slice(before).some((url) => url.includes("zeebe_received")),
    ).toBe(true);
    return text;
  },
  "api/watch": () => watchText(`kind=Secret&namespace=${namespace}`),
};

// A change that the schema of its action refuses never reaches the writer,
// so its answer holds no Secret value only because nothing ran. previewed,
// written, and refused fail on such an answer.
function previewed(call: () => Promise<Preview>): () => Promise<Preview> {
  return async () => {
    const preview = await call();
    expect(preview.errors).toEqual([]);
    expect(preview.objects).not.toEqual([]);
    return preview;
  };
}

function written(call: () => Promise<WriteResult>): () => Promise<WriteResult> {
  return async () => {
    const result = await call();
    expect(result).toMatchObject({ ok: true });
    return result;
  };
}

function refused<R extends WriteResult | Preview>(
  code: ApiErrorCode,
  call: () => Promise<R>,
): () => Promise<R> {
  return async () => {
    const result = await call();
    const seen: WriteResult | Preview = result;
    const errors = "ok" in seen ? (seen.ok ? [] : seen.errors) : seen.errors;
    expect(errors).not.toEqual([]);
    for (const error of errors) {
      // A refusal of the schema of an action names no object.
      expect(error).toMatchObject({ code, ref: expect.anything() });
    }
    return result;
  };
}

// "<file under src>#<export>".
const ACTION_CALLS: Record<string, () => Promise<unknown>> = {
  "app/dev/watch-probe/actions.ts#ping": () => ping(),
  // The backup does not exist, so the delete removes nothing.
  "app/fleet/clusters/[namespace]/[name]/actions.ts#applyBackupChange": written(
    () =>
      applyBackupChange(
        { namespace, name: "leak" },
        {
          intent: "Delete backup leak",
          apply: [],
          delete: [
            {
              apiVersion: "core.camunda.io/v1",
              kind: "LogicalBackupElasticsearch",
              namespace,
              name: "leak",
            },
          ],
          directOnly: true,
        },
      ),
  ),
  "app/fleet/clusters/[namespace]/[name]/actions.ts#previewClaimChange":
    previewed(() => previewClaimChange(claimChange())),
  // The precondition names a claim that does not exist, so the write
  // refuses with Stale and leaves nothing behind.
  "app/fleet/clusters/[namespace]/[name]/actions.ts#applyClaimChange": refused(
    "Stale",
    () =>
      applyClaimChange({
        ...claimChange(),
        preconditions: { [`CamundaClusterClaim/${namespace}/leak`]: "1" },
      }),
  ),
  // The change passes the schema, so the dry run and the delete read the
  // claim, refuse with Stale, and remove nothing.
  "app/fleet/clusters/[namespace]/[name]/actions.ts#previewClaimDelete":
    refused("Stale", async () => previewClaimDelete(await staleClaimDelete())),
  "app/fleet/clusters/[namespace]/[name]/actions.ts#applyClaimDelete": refused(
    "Stale",
    async () => applyClaimDelete(await staleClaimDelete()),
  ),
  "app/fleet/planes/[namespace]/[name]/actions.ts#previewPlaneClaimChange":
    previewed(() => previewPlaneClaimChange(planeClaimChange())),
  // The precondition names a claim that does not exist, so the write
  // refuses with Stale and leaves nothing behind.
  "app/fleet/planes/[namespace]/[name]/actions.ts#applyPlaneClaimChange":
    refused("Stale", () =>
      applyPlaneClaimChange({
        ...planeClaimChange(),
        preconditions: {
          [`CamundaManagementClusterClaim/${namespace}/leak`]: "1",
        },
      }),
    ),
  // The precondition is older than the live plane claim, so the dry run and
  // the delete reach the writer, refuse with Stale, and remove nothing.
  "app/fleet/planes/[namespace]/[name]/actions.ts#previewPlaneClaimDelete":
    refused("Stale", async () =>
      previewPlaneClaimDelete(await stalePlaneClaimDelete()),
    ),
  "app/fleet/planes/[namespace]/[name]/actions.ts#applyPlaneClaimDelete":
    refused("Stale", async () =>
      applyPlaneClaimDelete(await stalePlaneClaimDelete()),
    ),
  "app/fleet/clusters/new/actions.ts#previewClusterCreate": previewed(() =>
    previewClusterCreate(clusterCreate({ presetRef: "leak" })),
  ),
  // The claim has no presetRef, which the API server requires, so the
  // write refuses it with Invalid and leaves nothing behind.
  "app/fleet/clusters/new/actions.ts#createCluster": refused("Invalid", () =>
    createCluster(clusterCreate({})),
  ),
  "app/fleet/planes/new/actions.ts#previewPlaneCreate": previewed(() =>
    previewPlaneCreate(planeCreate(planeSpec())),
  ),
  // The claim has no spec that the API server accepts, so the write
  // refuses it with Invalid and leaves nothing behind.
  "app/fleet/planes/new/actions.ts#createPlane": refused("Invalid", () =>
    createPlane(planeCreate({})),
  ),
  "app/setup/connections/actions.ts#previewContractChange": previewed(
    async () => previewContractChange(await contractChange()),
  ),
  // The precondition is not the resourceVersion of the contract, so the
  // write refuses with Stale and leaves the contract as it is.
  "app/setup/connections/actions.ts#applyContractChange": refused(
    "Stale",
    async () =>
      applyContractChange({
        ...(await contractChange()),
        preconditions: { [`ObjectStorageConfig/${namespace}/leak`]: "1" },
      }),
  ),
  "app/setup/environments/actions.ts#previewEnvironmentChange": previewed(() =>
    previewEnvironmentChange({
      ...environmentChange(),
      create: [`Environment//${namespace}`],
    }),
  ),
  // A precondition for an object that does not exist refuses with Stale,
  // so these writes leave nothing behind.
  "app/setup/environments/actions.ts#applyEnvironmentChange": refused(
    "Stale",
    () =>
      applyEnvironmentChange({
        ...environmentChange(),
        preconditions: { [`Environment//${namespace}`]: "1" },
      }),
  ),
  "app/setup/environments/actions.ts#previewNamespaceLabel": previewed(
    async () => {
      const live = await api.read({
        apiVersion: "v1",
        kind: "Namespace",
        metadata: { name: namespace },
      });
      return previewNamespaceLabel({
        ...labelChange(namespace),
        preconditions: {
          [`Namespace//${namespace}`]: live.metadata!.resourceVersion!,
        },
      });
    },
  ),
  "app/setup/environments/actions.ts#applyNamespaceLabel": refused(
    "Stale",
    () =>
      applyNamespaceLabel({
        ...labelChange(`${namespace}-gone`),
        preconditions: { [`Namespace//${namespace}-gone`]: "1" },
      }),
  ),
  "app/setup/platform/actions.ts#previewPlatformConfigChange": previewed(() =>
    previewPlatformConfigChange({
      ...platformConfigChange(),
      create: [`CamundaPlatformConfig//${namespace}`],
    }),
  ),
  // A precondition for an object that does not exist refuses with Stale,
  // so this write leaves nothing behind.
  "app/setup/platform/actions.ts#applyPlatformConfigChange": refused(
    "Stale",
    () =>
      applyPlatformConfigChange({
        ...platformConfigChange(),
        preconditions: { [`CamundaPlatformConfig//${namespace}`]: "1" },
      }),
  ),
  "app/setup/presets/actions.ts#previewPresetChange": previewed(() =>
    previewPresetChange(presetCreate()),
  ),
  // The preset does not exist, so the delete refuses with Stale and
  // removes nothing.
  "app/setup/presets/actions.ts#applyPresetChange": refused("Stale", () =>
    applyPresetChange(presetDelete()),
  ),
  "app/setup/releases/actions.ts#previewReleaseChange": previewed(() =>
    previewReleaseChange({
      ...releaseChange(),
      create: [`CamundaRelease//${namespace}`],
    }),
  ),
  // A precondition for an object that does not exist refuses with Stale,
  // so this write leaves nothing behind.
  "app/setup/releases/actions.ts#applyReleaseChange": refused("Stale", () =>
    applyReleaseChange({
      ...releaseChange(),
      preconditions: { [`CamundaRelease//${namespace}`]: "1" },
    }),
  ),
  // A live claim, so the preview passes the checks of the move and reaches
  // the writer.
  "app/setup/releases/actions.ts#previewReleaseMove": previewed(async () => {
    const name = `move-${randomUUID().slice(0, 8)}`;
    const claim = {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaClusterClaim",
      metadata: { name, namespace },
      spec: { presetRef: "medium" },
    };
    const live: KubernetesObject = await api.create(claim);
    return previewReleaseMove({
      intent: `Move the cluster ${namespace}/${name} to the release leak`,
      apply: [{ ...claim, spec: { ...claim.spec, releaseRef: "leak" } }],
      preconditions: {
        [`CamundaClusterClaim/${namespace}/${name}`]:
          live.metadata?.resourceVersion ?? "",
      },
    });
  }),
  // The precondition names a claim that does not exist, so the write
  // refuses with Stale and leaves nothing behind.
  "app/setup/releases/actions.ts#applyReleaseMove": refused("Stale", () =>
    applyReleaseMove({
      ...claimChange(),
      preconditions: { [`CamundaClusterClaim/${namespace}/leak`]: "1" },
    }),
  ),
  "app/setup/settings/actions.ts#saveSettings": written(async () =>
    saveSettings(
      { "git.path": "y" },
      (await settings.readSettings()).resourceVersion,
    ),
  ),
  "app/setup/settings/actions.ts#testGitConnection": () => testGitConnection(),
  "app/setup/settings/actions.ts#testPrometheusQuery": () =>
    testPrometheusQuery(),
};

function presetCreate() {
  return createChangeOf("Create the preset leak", {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaClusterPreset",
    metadata: { name: namespace },
    spec: { cluster: {} },
  });
}

function presetDelete() {
  return {
    intent: "Delete the preset leak",
    apply: [],
    delete: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaClusterPreset",
        name: namespace,
      },
    ],
    preconditions: { [`CamundaClusterPreset//${namespace}`]: "1" },
  };
}

// contractChange changes the region of the contract leak, with the
// resourceVersion of its copy in the cache of the server, which the action
// reads.
async function contractChange() {
  const live = await vi.waitFor(async () => {
    const found = await get("ObjectStorageConfig", namespace, "leak");
    expect(found).toBeDefined();
    expect(await get("Namespace", undefined, namespace)).toBeDefined();
    return found!;
  });
  return {
    intent: "Change the object storage contract leak",
    apply: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "ObjectStorageConfig",
        metadata: { name: "leak", namespace },
        spec: {
          ...live.spec,
          s3: { ...live.spec.s3, region: "eu-central-1" },
        },
      },
    ],
    preconditions: {
      [`ObjectStorageConfig/${namespace}/leak`]: live.metadata.resourceVersion!,
    },
    force: true,
  };
}

function environmentChange() {
  return {
    intent: "Create the environment leak",
    apply: [
      {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "Environment",
        metadata: { name: namespace },
        spec: { displayName: "Leak" },
      },
    ],
  };
}

function platformConfigChange() {
  return {
    intent: "Change the platform config leak",
    apply: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaPlatformConfig",
        metadata: { name: namespace },
        spec: { auth: { method: "basic" } },
      },
    ],
  };
}

function releaseChange() {
  return {
    intent: "Create the release leak",
    apply: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "CamundaRelease",
        metadata: { name: namespace },
        spec: { version: "8.9.0" },
      },
    ],
  };
}

function labelChange(name: string) {
  return {
    intent: "Add the namespace leak to the environment leak",
    apply: [
      {
        apiVersion: "v1",
        kind: "Namespace",
        metadata: {
          name,
          labels: { "fleet.camunda.io/environment": namespace },
        },
      },
    ],
    directOnly: true,
  };
}

function claimChange() {
  return {
    intent: "Change the claim leak",
    apply: [
      {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "CamundaClusterClaim",
        metadata: { name: "leak", namespace },
        spec: { presetRef: "medium" },
      },
    ],
  };
}
// staleClaimDelete makes a claim and returns the change that deletes it,
// with its UID and a generation that it does not have.
async function staleClaimDelete() {
  const name = `delete-${randomUUID().slice(0, 8)}`;
  const live: KubernetesObject = await api.create({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { name, namespace },
    spec: { presetRef: "medium" },
  });
  const key = `CamundaClusterClaim/${namespace}/${name}`;
  return {
    intent: `Delete the claim ${name}`,
    apply: [],
    delete: [
      {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "CamundaClusterClaim",
        namespace,
        name,
      },
    ],
    uids: { [key]: live.metadata?.uid ?? "" },
    generations: { [key]: (live.metadata?.generation ?? 1) + 1 },
  };
}
// stalePlaneClaimDelete makes a plane claim and returns the change that
// deletes it, with its UID and a resourceVersion that it no longer has.
async function stalePlaneClaimDelete() {
  const name = `delete-${randomUUID().slice(0, 8)}`;
  const live: KubernetesObject = await api.create({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { name, namespace },
    spec: planeSpec(),
  });
  const key = `CamundaManagementClusterClaim/${namespace}/${name}`;
  return {
    intent: `Delete the management plane ${name}`,
    apply: [],
    delete: [
      {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "CamundaManagementClusterClaim",
        namespace,
        name,
      },
    ],
    uids: { [key]: live.metadata?.uid ?? "" },
    preconditions: { [key]: "1" },
  };
}
function planeClaimChange() {
  return {
    intent: "Change the plane claim leak",
    apply: [
      {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "CamundaManagementClusterClaim",
        metadata: { name: "leak", namespace },
        spec: planeSpec(),
      },
    ],
  };
}
function clusterCreate(spec: object) {
  return createChangeOf("Create the cluster leak", {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { name: "leak", namespace },
    spec,
  });
}
function planeCreate(spec: object) {
  return createChangeOf("Create the management plane leak", {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { name: "leak", namespace },
    spec,
  });
}
function planeSpec() {
  return {
    environments: [namespace],
    management: {
      identityProvider: { oidc: {} },
      identity: {
        externalUrl: "https://management.example.com/identity",
        admin: { claimName: "sub", claimValue: "42" },
      },
    },
    versions: { identity: "8.9.14" },
  };
}

// "<file under src>#<export>". An answer that read nothing holds no Secret
// value, so each call fails on it.
const LOADER_CALLS: Record<string, () => Promise<unknown>> = {
  "app/setup/plugins/load.ts#loadRegistrations": async () => {
    const sources = await pluginLoads.loadRegistrations();
    expect(sources.registrations.map((r) => r.metadata.name)).toContain(
      namespace,
    );
    expect(sources.namespaces.map((n) => n.metadata?.name)).toContain(
      namespace,
    );
    return sources;
  },
  "app/setup/plugins/load.ts#loadCatalog": async () => {
    const catalog = await pluginLoads.loadCatalog();
    expect(catalog).toMatchObject({
      charts: expect.arrayContaining([expect.anything()]),
    });
    return catalog;
  },
  "app/setup/plugins/load.ts#loadRegistration": async () => {
    const data = await pluginLoads.loadRegistration(namespace);
    expect(data?.registration.metadata.name).toBe(namespace);
    return data;
  },
};

const LIBRARY_CALLS: Record<string, () => Promise<unknown>> = {
  "settings#appPermissions": () => settings.appPermissions(),
  "settings#getSettings": () => settings.getSettings(),
  "settings#readSettings": () => settings.readSettings(),
  "settings#updateSettings": () => settings.updateSettings({ "git.path": "x" }),
  "settings#testGit": () => settings.testGit(),
  "settings#testPrometheus": () => settings.testPrometheus(),
  "secrets#listSecretRefs": () => secrets.listSecretRefs(namespace),
  "secrets#listSecretRefsIn": () => secrets.listSecretRefsIn([namespace]),
  "secrets#writeSecretValue": () =>
    secrets.writeSecretValue(
      { namespace, name: "leak", key: "token" },
      extraValue,
    ),
};

function sourceFiles(): string[] {
  return readdirSync(src, { withFileTypes: true, recursive: true })
    .filter(
      (e) => e.isFile() && /\.tsx?$/.test(e.name) && !/\.test\./.test(e.name),
    )
    .map((e) => path.join(e.parentPath, e.name));
}

function discoverRoutes(): string[] {
  const app = path.join(src, "app");
  return sourceFiles()
    .filter(
      (f) => f.startsWith(app + path.sep) && path.basename(f) === "route.ts",
    )
    .map((f) => path.relative(app, path.dirname(f)))
    .sort();
}

async function discoverActions(): Promise<string[]> {
  const found: string[] = [];
  for (const file of sourceFiles()) {
    const first = readFileSync(file, "utf8")
      .replace(/^\s*(\/\/.*\n|\/\*[\s\S]*?\*\/)\s*/g, "")
      .trimStart();
    if (!/^["']use server["']/.test(first)) continue;
    const loaded = (await import(file)) as Record<string, unknown>;
    for (const [name, value] of Object.entries(loaded)) {
      if (typeof value === "function") {
        found.push(`${path.relative(src, file)}#${name}`);
      }
    }
  }
  return found.sort();
}

function libraryFunctions(): string[] {
  return Object.entries({ settings, secrets })
    .flatMap(([module, exports]) =>
      Object.entries(exports)
        .filter(([, value]) => typeof value === "function")
        .map(([name]) => `${module}#${name}`),
    )
    .sort();
}

describe("no response holds a Secret value", () => {
  it("every route has a call in this test", () => {
    expect(Object.keys(ROUTE_CALLS).sort()).toEqual(discoverRoutes());
  });

  it("every server action has a call in this test", async () => {
    expect(Object.keys(ACTION_CALLS).sort()).toEqual(await discoverActions());
  });

  it("every loader of the plugins pages has a call in this test", () => {
    expect(Object.keys(LOADER_CALLS).sort()).toEqual(
      Object.keys(pluginLoads)
        .map((name) => `app/setup/plugins/load.ts#${name}`)
        .sort(),
    );
  });

  it.each(Object.keys(LOADER_CALLS))(
    "%s returns no Secret value",
    async (call) => {
      assertNoValue(JSON.stringify(await LOADER_CALLS[call]()));
    },
  );

  it("every function of settings and secrets has a call in this test", () => {
    expect(Object.keys(LIBRARY_CALLS).sort()).toEqual(libraryFunctions());
  });

  it.each(Object.keys(ROUTE_CALLS))(
    "route %s returns no Secret value",
    async (route) => {
      assertNoValue(await ROUTE_CALLS[route]());
    },
  );

  it.each(Object.keys(ACTION_CALLS))(
    "server action %s returns no Secret value",
    async (action) => {
      assertNoValue(JSON.stringify(await ACTION_CALLS[action]()));
    },
  );

  it.each(Object.keys(LIBRARY_CALLS))(
    "%s returns no Secret value",
    async (call) => {
      assertNoValue(JSON.stringify(await LIBRARY_CALLS[call]()));
    },
  );
});
