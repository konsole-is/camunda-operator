import { randomUUID } from "node:crypto";
import { readFileSync, readdirSync } from "node:fs";
import http from "node:http";
import type { AddressInfo } from "node:net";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  KubeConfig,
  KubernetesObjectApi,
  PatchStrategy,
} from "@kubernetes/client-node";
import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import { ping } from "@/app/dev/watch-probe/actions";
import { GET as healthz } from "@/app/api/healthz/route";
import { GET as logsRoute } from "@/app/api/logs/route";
import { GET as watchRoute } from "@/app/api/watch/route";
import { defaultCache } from "@/lib/k8s/cache";
import * as secrets from "@/lib/secrets";
import * as settings from "@/lib/settings";

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

beforeAll(async () => {
  echo = http.createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on("data", (c: Buffer) => chunks.push(c));
    req.on("end", () => {
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
  process.env.FLEET_NAMESPACE = namespace;
  Object.assign(process.env, {
    FLEET_SETTINGS_GIT_REPOSITORY: `${url}/fleet.git`,
    FLEET_SETTINGS_GIT_CREDENTIALS_SECRET_REF: "leak",
    FLEET_SETTINGS_PROMETHEUS_URL: url,
    FLEET_SETTINGS_PROMETHEUS_CREDENTIALS_SECRET_REF: "leak",
  });
});
afterAll(async () => {
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
  "api/watch": () => watchText(`kind=Secret&namespace=${namespace}`),
};
// "<file under src>#<export>".
const ACTION_CALLS: Record<string, () => Promise<unknown>> = {
  "app/dev/watch-probe/actions.ts#ping": () => ping(),
};
const LIBRARY_CALLS: Record<string, () => Promise<unknown>> = {
  "settings#appPermissions": () => settings.appPermissions(),
  "settings#getSettings": () => settings.getSettings(),
  "settings#readSettings": () => settings.readSettings(),
  "settings#updateSettings": () => settings.updateSettings({ "git.path": "x" }),
  "settings#testGit": () => settings.testGit(),
  "settings#testPrometheus": () => settings.testPrometheus(),
  "secrets#listSecretRefs": () => secrets.listSecretRefs(namespace),
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

  it("every function of settings and secrets has a call in this test", () => {
    expect(Object.keys(LIBRARY_CALLS).sort()).toEqual(libraryFunctions());
  });

  it.each(Object.keys(ROUTE_CALLS))(
    "route %s returns no Secret value",
    async (route) => {
      assertNoValue(await ROUTE_CALLS[route]());
    },
  );

  it("no server action returns a Secret value", async () => {
    for (const call of Object.values(ACTION_CALLS)) {
      assertNoValue(JSON.stringify(await call()));
    }
  });

  it.each(Object.keys(LIBRARY_CALLS))(
    "%s returns no Secret value",
    async (call) => {
      assertNoValue(JSON.stringify(await LIBRARY_CALLS[call]()));
    },
  );
});
