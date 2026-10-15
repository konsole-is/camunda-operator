// The Vitest project envtest and the Playwright page scenarios start the
// envtest control plane through this module, in plain Node, without Go.
import { spawn, type ChildProcess } from "node:child_process";
import { generateKeyPairSync, randomBytes } from "node:crypto";
import fs from "node:fs/promises";
import http from "node:http";
import https from "node:https";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { KubeConfig } from "@kubernetes/client-node";
import { parseAllDocuments } from "yaml";

/** CRD_DIR is config/crd/bases of the repository. */
export const CRD_DIR = fileURLToPath(
  new URL("../../../config/crd/bases", import.meta.url),
);

export interface EnvtestOptions {
  /** assetsDir holds etcd and kube-apiserver. The default is KUBEBUILDER_ASSETS. */
  assetsDir?: string;
  /** crdDirs are the directories whose CRDs the API server serves. The default is CRD_DIR. */
  crdDirs?: string[];
  /** startTimeoutMs bounds each wait of the start. The default is two minutes, as in test/envtest. */
  startTimeoutMs?: number;
}

/** Envtest is a running control plane. stop ends both processes and removes their files. */
export interface Envtest {
  /** kubeconfigPath is a kubeconfig file of a user in system:masters. */
  kubeconfigPath: string;
  kubeConfig: KubeConfig;
  stop(): Promise<void>;
}

interface Running {
  child: ChildProcess;
  name: string;
  stderr: string[];
  /** spawnError is set when the binary could not start, such as a missing file. */
  spawnError?: Error;
}

/**
 * startEnvtest starts etcd and kube-apiserver from the envtest binaries,
 * installs the CRDs of crdDirs, and resolves when each CRD is established
 * and its resources answer a list. It rejects when KUBEBUILDER_ASSETS is not
 * set, when a process exits during the start, or when a wait passes
 * startTimeoutMs. On a rejection nothing keeps running.
 */
export async function startEnvtest(
  options: EnvtestOptions = {},
): Promise<Envtest> {
  const assetsDir = options.assetsDir ?? process.env.KUBEBUILDER_ASSETS;
  if (!assetsDir) {
    throw new Error(
      "KUBEBUILDER_ASSETS is not set, so the tests cannot start the envtest API server. Run make setup-envtest, or run the tests with make fleet-test, which sets it.",
    );
  }
  const timeoutMs = options.startTimeoutMs ?? 120_000;
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), "fleet-envtest-"));
  const running: Running[] = [];
  // A test runner that exits without stop must not leave the processes behind.
  const killAll = () => running.forEach((r) => r.child.kill("SIGKILL"));
  process.once("exit", killAll);

  try {
    const [etcdPort, peerPort, apiPort] = [
      await freePort(),
      await freePort(),
      await freePort(),
    ];
    const etcdURL = `http://127.0.0.1:${etcdPort}`;
    const etcd = run(path.join(assetsDir, "etcd"), "etcd", [
      `--data-dir=${path.join(dir, "etcd")}`,
      `--listen-client-urls=${etcdURL}`,
      `--advertise-client-urls=${etcdURL}`,
      `--listen-peer-urls=http://127.0.0.1:${peerPort}`,
      "--unsafe-no-fsync=true",
    ]);
    running.push(etcd);
    await waitUntil(etcd, timeoutMs, () => answers(`${etcdURL}/health`));

    const token = randomBytes(24).toString("hex");
    await fs.writeFile(
      path.join(dir, "tokens.csv"),
      `${token},envtest-admin,envtest-admin,"system:masters"\n`,
      { mode: 0o600 },
    );
    const keys = generateKeyPairSync("rsa", {
      modulusLength: 2048,
      publicKeyEncoding: { type: "spki", format: "pem" },
      privateKeyEncoding: { type: "pkcs8", format: "pem" },
    });
    await fs.writeFile(path.join(dir, "sa.pub"), keys.publicKey);
    await fs.writeFile(path.join(dir, "sa.key"), keys.privateKey, {
      mode: 0o600,
    });

    const server = `https://127.0.0.1:${apiPort}`;
    const apiserver = run(
      path.join(assetsDir, "kube-apiserver"),
      "kube-apiserver",
      [
        "--bind-address=127.0.0.1",
        `--secure-port=${apiPort}`,
        `--cert-dir=${path.join(dir, "certs")}`,
        `--etcd-servers=${etcdURL}`,
        "--service-cluster-ip-range=10.0.0.0/24",
        "--allow-privileged=true",
        "--disable-admission-plugins=ServiceAccount",
        "--authorization-mode=RBAC",
        `--token-auth-file=${path.join(dir, "tokens.csv")}`,
        `--service-account-issuer=${server}/`,
        `--service-account-key-file=${path.join(dir, "sa.pub")}`,
        `--service-account-signing-key-file=${path.join(dir, "sa.key")}`,
      ],
    );
    running.push(apiserver);
    await waitUntil(apiserver, timeoutMs, () =>
      answers(`${server}/readyz`, token),
    );

    const kubeconfigPath = path.join(dir, "kubeconfig");
    await fs.writeFile(kubeconfigPath, kubeconfig(server, token), {
      mode: 0o600,
    });
    const kubeConfig = new KubeConfig();
    kubeConfig.loadFromFile(kubeconfigPath);

    await installCRDs(
      server,
      token,
      options.crdDirs ?? [CRD_DIR],
      apiserver,
      timeoutMs,
    );

    return {
      kubeconfigPath,
      kubeConfig,
      async stop() {
        process.off("exit", killAll);
        await stopAll(running);
        await fs.rm(dir, { recursive: true, force: true });
      },
    };
  } catch (error) {
    process.off("exit", killAll);
    await stopAll(running);
    await fs.rm(dir, { recursive: true, force: true });
    throw error;
  }
}

function run(binary: string, name: string, args: string[]): Running {
  const child = spawn(binary, args, { stdio: ["ignore", "ignore", "pipe"] });
  const running: Running = { child, name, stderr: [] };
  child.on("error", (error) => {
    running.spawnError = error;
  });
  child.stderr?.setEncoding("utf8").on("data", (chunk: string) => {
    running.stderr.push(...chunk.split("\n"));
    running.stderr.splice(0, Math.max(0, running.stderr.length - 20));
  });
  return running;
}

async function waitUntil(
  running: Running,
  timeoutMs: number,
  ready: () => Promise<boolean>,
): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  for (;;) {
    if (running.spawnError) {
      throw new Error(
        `${running.name} could not start: ${running.spawnError.message}`,
      );
    }
    if (running.child.exitCode !== null || running.child.signalCode !== null) {
      throw new Error(
        `${running.name} exited during the start of envtest:\n${running.stderr.join("\n")}`,
      );
    }
    if (await ready()) return;
    if (Date.now() > deadline) {
      throw new Error(
        `${running.name} was not ready after ${timeoutMs / 1000} seconds:\n${running.stderr.join("\n")}`,
      );
    }
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
}

interface Answer {
  status: number;
  body: string;
}

// The serving certificate of the API server is self-signed, so send does not
// verify it. A connection error, or no answer within ten seconds, gives
// status 0.
function send(
  method: string,
  url: string,
  token?: string,
  body?: string,
): Promise<Answer> {
  const transport = url.startsWith("https:") ? https : http;
  return new Promise((resolve) => {
    const request = transport.request(
      url,
      {
        method,
        rejectUnauthorized: false,
        headers: {
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
          ...(body ? { "Content-Type": "application/json" } : {}),
        },
      },
      (response) => {
        const chunks: Buffer[] = [];
        response.on("data", (chunk: Buffer) => chunks.push(chunk));
        response.on("error", () => resolve({ status: 0, body: "" }));
        response.on("end", () =>
          resolve({
            status: response.statusCode ?? 0,
            body: Buffer.concat(chunks).toString("utf8"),
          }),
        );
      },
    );
    request.on("error", () => resolve({ status: 0, body: "" }));
    request.setTimeout(10_000, () => request.destroy());
    request.end(body);
  });
}

async function answers(url: string, token?: string): Promise<boolean> {
  const { status } = await send("GET", url, token);
  return status >= 200 && status < 300;
}

interface CRD {
  kind?: string;
  metadata: { name: string };
  spec: {
    group: string;
    names: { plural: string };
    versions: { name: string; served: boolean; storage: boolean }[];
  };
  status?: { conditions?: { type: string; status: string }[] };
}

// installCRDs posts the CRDs as raw JSON. The typed client of
// @kubernetes/client-node drops schema keys that it does not model, such as
// x-kubernetes-int-or-string, and the API server then refuses the CRD.
async function installCRDs(
  server: string,
  token: string,
  dirs: string[],
  apiserver: Running,
  timeoutMs: number,
): Promise<void> {
  const crds: CRD[] = [];
  for (const dir of dirs) {
    const files = (await fs.readdir(dir))
      .filter((f) => f.endsWith(".yaml"))
      .sort();
    for (const file of files) {
      const text = await fs.readFile(path.join(dir, file), "utf8");
      for (const doc of parseAllDocuments(text)) {
        const object = doc.toJS() as CRD | null;
        if (object?.kind === "CustomResourceDefinition") crds.push(object);
      }
    }
  }
  const crdsURL = `${server}/apis/apiextensions.k8s.io/v1/customresourcedefinitions`;
  for (const crd of crds) {
    const answer = await send("POST", crdsURL, token, JSON.stringify(crd));
    if (answer.status !== 201) {
      throw new Error(
        `The API server refused the CRD ${crd.metadata.name} with status ${answer.status}: ${answer.body}`,
      );
    }
  }
  // A CRD can be established a moment before its resources answer a list.
  // The data layer reads a 404 as a missing CRD, so the start waits for both.
  for (const crd of crds) {
    const version = crd.spec.versions.find((v) => v.served && v.storage)!.name;
    const collection = `${server}/apis/${crd.spec.group}/${version}/${crd.spec.names.plural}?limit=1`;
    await waitUntil(apiserver, timeoutMs, async () => {
      const answer = await send(
        "GET",
        `${crdsURL}/${crd.metadata.name}`,
        token,
      );
      if (answer.status !== 200) return false;
      const live = JSON.parse(answer.body) as CRD;
      const established = live.status?.conditions?.some(
        (c) => c.type === "Established" && c.status === "True",
      );
      return Boolean(established) && (await answers(collection, token));
    });
  }
}

function kubeconfig(server: string, token: string): string {
  return [
    "apiVersion: v1",
    "kind: Config",
    "clusters:",
    "  - name: envtest",
    "    cluster:",
    `      server: ${server}`,
    "      insecure-skip-tls-verify: true",
    "users:",
    "  - name: envtest-admin",
    "    user:",
    `      token: ${token}`,
    "contexts:",
    "  - name: envtest",
    "    context:",
    "      cluster: envtest",
    "      user: envtest-admin",
    "current-context: envtest",
    "",
  ].join("\n");
}

// The API server stops before etcd, so it does not log a lost etcd. A
// process that does not exit within ten seconds of SIGTERM gets SIGKILL.
async function stopAll(running: Running[]): Promise<void> {
  for (const r of [...running].reverse()) {
    if (
      r.spawnError ||
      r.child.exitCode !== null ||
      r.child.signalCode !== null
    ) {
      continue;
    }
    const exited = new Promise((resolve) => r.child.once("exit", resolve));
    r.child.kill("SIGTERM");
    const timer = setTimeout(() => r.child.kill("SIGKILL"), 10_000);
    await exited;
    clearTimeout(timer);
  }
}

function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const probe = net.createServer();
    probe.unref();
    probe.on("error", reject);
    probe.listen(0, "127.0.0.1", () => {
      const { port } = probe.address() as net.AddressInfo;
      probe.close(() => resolve(port));
    });
  });
}
