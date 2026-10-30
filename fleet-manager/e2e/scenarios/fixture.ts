import path from "node:path";
import type { KubeConfig } from "@kubernetes/client-node";
import { test as base } from "@playwright/test";
import { OBSERVE_FILE_VARIABLE } from "../../src/lib/observe/observe-variable";
import { KubeError, kubeRequest } from "../../test/scenarios/kube";
import { type Scenario, openScenario } from "../../test/scenarios/loader";
import { bindChartRBAC } from "../../test/scenarios/rbac";
import { type EnvtestApp, startEnvtestApp } from "../envtest-app";
import { type PrometheusStub, startPrometheusStub } from "./prometheus-stub";

export { expect } from "@playwright/test";

/** APP_USER is the user of the app. It holds the RBAC of the chart and nothing more. */
export const APP_USER = "fleet-manager-scenarios";

/** APP_NAMESPACE is the namespace of the app, as the chart installs it into the operator namespace. */
export const APP_NAMESPACE = "camunda-system";

/**
 * ScenarioHandle is the page scenario of a test file, loaded into envtest.
 *
 * advance stands in for the controllers between two states. An object that
 * the app created between them makes advance fail with a 409 that names
 * it. advance does not recreate a Pod that a rollout replaced, because the
 * scenario UID of an object comes from its kind, namespace, and name.
 */
export interface ScenarioHandle extends Scenario {
  /** admin is the kubeconfig of envtest-admin, to assert what a page wrote. */
  admin: KubeConfig;
  prometheus: PrometheusStub;
}

type ScenarioApp = { file: string; app: EnvtestApp; scenario: ScenarioHandle };

// Playwright reuses a worker for the next file with the same worker
// options, so the worker keeps the app of one file and starts a new one
// when the file changes.
type ScenarioApps = { open(file: string): Promise<ScenarioApp> };

/**
 * test runs a page test against the page scenario e2e/scenarios/<name>,
 * which the file sets with test.use({ scenarioName: "<name>" }). Each file
 * gets its own envtest, the scenario, and a server of the gallery build
 * that reads the scenario as APP_USER. The tests of a file share them and
 * run in order. baseURL is the app.
 */
export const test = base.extend<
  { scenario: ScenarioHandle; scenarioApp: ScenarioApp },
  { scenarioName: string; scenarioApps: ScenarioApps }
>({
  scenarioName: [
    undefined as unknown as string,
    { scope: "worker", option: true },
  ],
  scenarioApps: [
    async ({ scenarioName }, provide) => {
      let current: ScenarioApp | undefined;
      await provide({
        async open(file) {
          if (current?.file === file) return current;
          if (current) await stopScenarioApp(current);
          current = undefined;
          current = await startScenarioApp(scenarioName, file);
          return current;
        },
      });
      if (current) await stopScenarioApp(current);
    },
    { scope: "worker" },
  ],
  scenarioApp: [
    async ({ scenarioApps }, provide, testInfo) => {
      await provide(await scenarioApps.open(testInfo.file));
    },
    { timeout: 180_000 },
  ],
  scenario: async ({ scenarioApp }, provide) => {
    await provide(scenarioApp.scenario);
  },
  baseURL: async ({ scenarioApp }, provide) => {
    await provide(scenarioApp.app.baseURL);
  },
});

async function startScenarioApp(
  scenarioName: string | undefined,
  file: string,
): Promise<ScenarioApp> {
  if (!scenarioName) {
    throw new Error("Set test.use({ scenarioName }) in the test file");
  }
  const dir = path.resolve("e2e/scenarios", scenarioName);
  let handle: ScenarioHandle | undefined;
  const app = await startEnvtestApp({
    envtest: { users: [{ name: APP_USER }] },
    readyPath: "/api/healthz",
    prepare: async (envtest) => {
      const admin = envtest.kubeConfig;
      const scenario = await openScenario(admin, dir);
      await ensureNamespace(admin, APP_NAMESPACE);
      await bindChartRBAC(admin, APP_USER, APP_NAMESPACE);
      const prometheus = await startPrometheusStub();
      handle = { ...scenario, admin, prometheus };
      return {
        kubeconfigPath: envtest.users[APP_USER].kubeconfigPath,
        env: {
          FLEET_NAMESPACE: APP_NAMESPACE,
          FLEET_SETTINGS_PROMETHEUS_URL: prometheus.url,
          [OBSERVE_FILE_VARIABLE]: path.join(dir, "observe.yaml"),
        },
      };
    },
  }).catch(async (error: unknown) => {
    await handle?.prometheus.close();
    throw error;
  });
  return { file, app, scenario: handle! };
}

async function stopScenarioApp({ app, scenario }: ScenarioApp): Promise<void> {
  await scenario.prometheus.close();
  await app.stop();
}

async function ensureNamespace(admin: KubeConfig, name: string): Promise<void> {
  try {
    await kubeRequest(admin, "POST", "/api/v1/namespaces", {
      apiVersion: "v1",
      kind: "Namespace",
      metadata: { name },
    });
  } catch (error) {
    if (error instanceof KubeError && error.status === 409) return;
    throw error;
  }
}
