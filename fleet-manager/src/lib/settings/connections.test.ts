import { generateKeyPairSync } from "node:crypto";
import http from "node:http";
import type { AddressInfo } from "node:net";
import type { HttpClient } from "isomorphic-git";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { KubernetesRequestError } from "@/lib/k8s/client";
import { readSecretValues } from "@/lib/secrets/values";
import { SETTING_KEYS, type Settings } from "./keys";
import { getSettings } from "./store";
import { testGit, testPrometheus } from "./connections";

vi.mock("./store", () => ({ getSettings: vi.fn() }));
vi.mock("@/lib/secrets/values", () => ({ readSecretValues: vi.fn() }));
// The test servers speak plain HTTP. This Git transport sends an https://
// address to them over HTTP, so testGit sees an HTTPS repository.
vi.mock("isomorphic-git/http/node", async (real) => {
  const transport = (await real<{ default: HttpClient }>()).default;
  return {
    default: {
      request: (r: Parameters<HttpClient["request"]>[0]) =>
        transport.request({ ...r, url: r.url.replace(/^https:/, "http:") }),
    },
  };
});

const settings = (values: Partial<Settings>): Settings => ({
  ...(Object.fromEntries(SETTING_KEYS.map((k) => [k, ""])) as Settings),
  ...values,
});

type Handler = (req: http.IncomingMessage, res: http.ServerResponse) => void;
let server: http.Server | undefined;

async function serve(handler: Handler): Promise<string> {
  server = http.createServer(handler);
  await new Promise<void>((resolve) => server!.listen(0, "127.0.0.1", resolve));
  return `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
}

// serveGit is serve with an https:// address, for the Git transport above.
async function serveGit(handler: Handler): Promise<string> {
  return (await serve(handler)).replace(/^http:/, "https:");
}

async function closedAddress(): Promise<string> {
  const url = await serve(() => {});
  await new Promise((resolve) => server!.close(resolve));
  server = undefined;
  return url;
}

const pkt = (s: string) => (s.length + 4).toString(16).padStart(4, "0") + s;
const advertisement = (branches: string[]) =>
  pkt("# service=git-upload-pack\n") +
  "0000" +
  branches
    .map((b, i) =>
      pkt(
        `${"a".repeat(40)} refs/heads/${b}${i === 0 ? "\0side-band-64k" : ""}\n`,
      ),
    )
    .join("") +
  "0000";

const basic = (user: string, secret: string) =>
  "Basic " + Buffer.from(`${user}:${secret}`).toString("base64");

// gitServer serves fleet.git with the branches, and asks for the basic
// credentials of auth when they are given.
function gitServer(branches: string[], auth?: string): Handler {
  return (req, res) => {
    if (auth && req.headers.authorization !== auth) {
      res.writeHead(401, { "WWW-Authenticate": 'Basic realm="git"' });
      res.end(`denied for ${req.headers.authorization ?? "nobody"}`);
      return;
    }
    if (req.url !== "/fleet.git/info/refs?service=git-upload-pack") {
      res.writeHead(404).end();
      return;
    }
    res.writeHead(200, {
      "Content-Type": "application/x-git-upload-pack-advertisement",
    });
    res.end(advertisement(branches));
  };
}

beforeEach(() => {
  vi.resetAllMocks();
  process.env.FLEET_NAMESPACE = "fleet";
});
afterEach(async () => {
  if (server) await new Promise((resolve) => server!.close(resolve));
  server = undefined;
  vi.restoreAllMocks();
});

// These tests start a local Git server. On a busy machine they need more
// than the default 5 s.
const slow = { timeout: 30_000 };

describe("testGit", slow, () => {
  it("connects with the token of the Secret", async () => {
    const url =
      (await serveGit(gitServer(["main"], basic("bot", "tok-1")))) +
      "/fleet.git";
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "git.repository": url,
        "git.branch": "main",
        "git.credentialsSecretRef": "git",
      }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({
      username: "bot",
      token: "tok-1",
    });

    expect(await testGit()).toEqual({
      ok: true,
      message: `Connected to ${url}.`,
    });
    expect(readSecretValues).toHaveBeenCalledWith("fleet", "git");
  });

  it("connects without a Secret to a repository that needs none", async () => {
    const url = (await serve(gitServer(["main"]))) + "/fleet.git";
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "git.repository": url }),
    );

    expect(await testGit()).toEqual({
      ok: true,
      message: `Connected to ${url}.`,
    });
  });

  it("names a branch the repository does not have", async () => {
    const url = (await serve(gitServer(["main"]))) + "/fleet.git";
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "git.repository": url, "git.branch": "fleet" }),
    );

    expect(await testGit()).toEqual({
      ok: false,
      message: `${url} has no branch fleet.`,
    });
  });

  it("a wrong token is refused", async () => {
    const url =
      (await serveGit(gitServer(["main"], basic("bot", "right")))) +
      "/fleet.git";
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "git.repository": url, "git.credentialsSecretRef": "git" }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({
      username: "bot",
      token: "wrong",
    });

    expect(await testGit()).toEqual({
      ok: false,
      message: `${url} refused the credentials.`,
    });
  });

  it("a refusal message holds no credential", async () => {
    const url =
      (await serveGit((req, res) => {
        res.writeHead(403);
        res.end(`token tok-secret in ${req.headers.authorization}`);
      })) + "/fleet.git";
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "git.repository": url, "git.credentialsSecretRef": "git" }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({
      username: "bot",
      token: "tok-secret",
    });

    const { message } = await testGit();
    expect(message).not.toContain("tok-secret");
    expect(message).not.toContain(basic("bot", "tok-secret").slice(6));
  });

  it("an answer that is not Git holds no credential in the message", async () => {
    const token = "tok-" + "x".repeat(400);
    const url =
      (await serveGit((req, res) => {
        res.writeHead(200, { "Content-Type": "text/plain" });
        res.end(`${req.headers.authorization} ${token} bot`);
      })) + "/fleet.git";
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "git.repository": url, "git.credentialsSecretRef": "git" }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({ username: "bot", token });

    const result = await testGit();

    expect(result.ok).toBe(false);
    expect(result.message).toMatch(new RegExp(`^Could not reach ${url}: .+`));
    expect(result.message).not.toContain("tok-");
    expect(result.message).not.toContain("Basic");
    expect(result.message).not.toContain("bot");
  });

  it("an unreachable repository names it", async () => {
    const url = (await closedAddress()) + "/fleet.git";
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "git.repository": url }),
    );

    const result = await testGit();
    expect(result.ok).toBe(false);
    expect(result.message).toMatch(new RegExp(`^Could not reach ${url}: .+`));
  });

  it("a repository that never ends its answer gets its connection closed", async () => {
    let closed = false;
    const url =
      (await serve((req, res) => {
        // One byte each second keeps the socket busy, so only the deadline
        // of the test ends the request.
        res.writeHead(200, {
          "Content-Type": "application/x-git-upload-pack-advertisement",
        });
        const timer = setInterval(() => res.write("0"), 1000);
        req.socket.on("close", () => {
          clearInterval(timer);
          closed = true;
        });
      })) + "/fleet.git";
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "git.repository": url }),
    );

    const result = await testGit();

    expect(result.ok).toBe(false);
    expect(result.message).toMatch(new RegExp(`^Could not reach ${url}: .+`));
    await vi.waitFor(() => expect(closed).toBe(true));
  }, 20_000);

  it("does not follow a redirect with the credentials", async () => {
    const paths: string[] = [];
    const base = await serve((req, res) => {
      paths.push(req.url ?? "");
      if (!req.headers.authorization) {
        res.writeHead(401, { "WWW-Authenticate": 'Basic realm="git"' }).end();
        return;
      }
      res.writeHead(302, { Location: `${base}/moved${req.url}` }).end();
    });
    const url = base.replace(/^http:/, "https:") + "/fleet.git";
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "git.repository": url, "git.credentialsSecretRef": "git" }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({
      username: "bot",
      token: "tok-1",
    });

    expect((await testGit()).ok).toBe(false);
    expect(paths.filter((p) => p.startsWith("/moved"))).toEqual([]);
  });

  it("refuses to send credentials over plain HTTP", async () => {
    let requests = 0;
    const url =
      (await serve((_req, res) => {
        requests++;
        res.writeHead(401).end();
      })) + "/fleet.git";
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "git.repository": url, "git.credentialsSecretRef": "git" }),
    );

    expect(await testGit()).toEqual({
      ok: false,
      message: `${url} must start with https:// to use the credentials of the Secret git.`,
    });
    expect(readSecretValues).not.toHaveBeenCalled();
    expect(requests).toBe(0);
  });

  it("a Secret that cannot be read gives a failed result", async () => {
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "git.repository": "https://git.example.com/fleet.git",
        "git.credentialsSecretRef": "git",
      }),
    );
    vi.mocked(readSecretValues).mockRejectedValue(
      new KubernetesRequestError(403, 'secrets "git" is forbidden: tok-1'),
    );

    expect(await testGit()).toEqual({
      ok: false,
      message:
        "Could not read the Secret git in namespace fleet: the API server answered HTTP 403",
    });
  });

  it("settings that cannot be read give a failed result", async () => {
    vi.mocked(getSettings).mockRejectedValue(
      Object.assign(new Error("connect ECONNREFUSED 10.0.0.1:443"), {
        code: "ECONNREFUSED",
      }),
    );

    expect(await testGit()).toEqual({
      ok: false,
      message: "Could not read the settings: ECONNREFUSED",
    });
  });

  it("no repository set", async () => {
    vi.mocked(getSettings).mockResolvedValue(settings({}));

    expect(await testGit()).toEqual({
      ok: false,
      message: "No Git repository is set.",
    });
  });

  it("a missing Secret names it", async () => {
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "git.repository": "https://git.example.com/fleet.git",
        "git.credentialsSecretRef": "git",
      }),
    );
    vi.mocked(readSecretValues).mockResolvedValue(undefined);

    expect(await testGit()).toEqual({
      ok: false,
      message: "The Secret git does not exist in namespace fleet.",
    });
  });

  it("a Secret without token", async () => {
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "git.repository": "https://git.example.com/fleet.git",
        "git.credentialsSecretRef": "git",
      }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({ username: "bot" });

    expect(await testGit()).toEqual({
      ok: false,
      message: "The Secret git has no key token.",
    });
  });

  it("connects with a complete GitHub App Secret", async () => {
    const base = await serveGit(
      gitServer(["main"], basic("x-access-token", "ghs_test")),
    );
    const url = base + "/fleet.git";
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "git.repository": url, "git.credentialsSecretRef": "git" }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({
      appId: "123",
      installationId: "701",
      privateKey: generateKeyPairSync("rsa", {
        modulusLength: 2048,
        privateKeyEncoding: { type: "pkcs1", format: "pem" },
        publicKeyEncoding: { type: "spki", format: "pem" },
      }).privateKey,
    });
    // The Git server runs on 127.0.0.1, so it has to answer as GitHub
    // Enterprise Server. The bot lookup fails, which a test does not need.
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const key = `${init?.method ?? "GET"} ${String(input)}`;
      if (key === `GET ${base}/api/v3/meta`) {
        return Response.json({ installed_version: "3.17.0" });
      }
      if (key === `POST ${base}/api/v3/app/installations/701/access_tokens`) {
        return Response.json(
          {
            token: "ghs_test",
            expires_at: new Date(Date.now() + 3_600_000).toISOString(),
          },
          { status: 201 },
        );
      }
      throw new Error(`unexpected request ${key}`);
    });

    expect(await testGit()).toEqual({
      ok: true,
      message: `Connected to ${url}.`,
    });
  });

  it("a refused App token is not used again", async () => {
    const base = await serveGit(
      gitServer(["main"], basic("x-access-token", "ghs_new")),
    );
    const url = base + "/fleet.git";
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "git.repository": url, "git.credentialsSecretRef": "git" }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({
      appId: "123",
      installationId: "702",
      privateKey: generateKeyPairSync("rsa", {
        modulusLength: 2048,
        privateKeyEncoding: { type: "pkcs1", format: "pem" },
        publicKeyEncoding: { type: "spki", format: "pem" },
      }).privateKey,
    });
    const tokens = ["ghs_old", "ghs_new"];
    let tokenRequests = 0;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const key = `${init?.method ?? "GET"} ${String(input)}`;
      if (key === `GET ${base}/api/v3/meta`) {
        return Response.json({ installed_version: "3.17.0" });
      }
      if (key === `POST ${base}/api/v3/app/installations/702/access_tokens`) {
        tokenRequests++;
        return Response.json(
          {
            token: tokens.shift(),
            expires_at: new Date(Date.now() + 3_600_000).toISOString(),
          },
          { status: 201 },
        );
      }
      throw new Error(`unexpected request ${key}`);
    });

    expect(await testGit()).toEqual({
      ok: false,
      message: `${url} refused the credentials.`,
    });
    expect(await testGit()).toEqual({
      ok: true,
      message: `Connected to ${url}.`,
    });
    expect(tokenRequests).toBe(2);
  });

  it("a Secret with token keys and App keys gives the message that names both sets", async () => {
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "git.repository": "https://git.example.com/fleet.git",
        "git.credentialsSecretRef": "git",
      }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({
      username: "bot",
      token: "tok",
      appId: "123",
    });

    expect(await testGit()).toEqual({
      ok: false,
      message:
        "The Secret git has keys for a token and keys for a GitHub App. Keep username and token, or appId, installationId, and privateKey.",
    });
  });
});

// promServer answers the query vector(1) when the Authorization header is
// auth, and 401 with the header it got otherwise.
function promServer(auth: string | undefined): Handler {
  return (req, res) => {
    if (req.headers.authorization !== auth) {
      res.writeHead(401).end(`bad ${req.headers.authorization}`);
      return;
    }
    const asked = new URL(req.url ?? "", "http://x");
    if (
      asked.pathname !== "/api/v1/query" ||
      asked.searchParams.get("query") !== "vector(1)"
    ) {
      res.writeHead(404).end();
      return;
    }
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(
      JSON.stringify({
        status: "success",
        data: { resultType: "vector", result: [] },
      }),
    );
  };
}

describe("testPrometheus", () => {
  it("basic auth from username and password", async () => {
    const url = await serve(promServer(basic("admin", "pw-1")));
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "prometheus.url": url + "/",
        "prometheus.credentialsSecretRef": "prom",
      }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({
      username: "admin",
      password: "pw-1",
    });

    expect(await testPrometheus()).toEqual({
      ok: true,
      message: `Prometheus at ${url}/ answered.`,
    });
  });

  it("bearer auth from token", async () => {
    const url = await serve(promServer("Bearer tok-1"));
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "prometheus.url": url,
        "prometheus.credentialsSecretRef": "prom",
      }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({ token: "tok-1" });

    expect(await testPrometheus()).toEqual({
      ok: true,
      message: `Prometheus at ${url} answered.`,
    });
  });

  it("connects without a Secret to a Prometheus that needs none", async () => {
    const url = await serve(promServer(undefined));
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "prometheus.url": url }),
    );

    expect((await testPrometheus()).ok).toBe(true);
  });

  it("does not follow a redirect with the credentials", async () => {
    const paths: string[] = [];
    const url = await serve((req, res) => {
      paths.push(req.url ?? "");
      res.writeHead(302, { Location: `/moved${req.url}` }).end();
    });
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "prometheus.url": url,
        "prometheus.credentialsSecretRef": "prom",
      }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({ token: "tok-1" });

    expect(await testPrometheus()).toEqual({
      ok: false,
      message: `Prometheus at ${url} answered HTTP 302.`,
    });
    expect(paths.filter((p) => p.startsWith("/moved"))).toEqual([]);
  });

  it.each([
    [
      "/prom/?tenant=orders",
      "/prom/api/v1/query?tenant=orders&query=vector%281%29",
    ],
    ["/prom#top", "/prom/api/v1/query?query=vector%281%29"],
  ])("keeps the path and the query of the address %s", async (suffix, want) => {
    let seen = "";
    const base = await serve((req, res) => {
      seen = req.url ?? "";
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ status: "success" }));
    });
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "prometheus.url": base + suffix }),
    );

    expect((await testPrometheus()).ok).toBe(true);
    expect(seen).toBe(want);
  });

  it("an HTTP 500 names the status", async () => {
    const url = await serve((_req, res) => res.writeHead(500).end("boom"));
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "prometheus.url": url }),
    );

    expect(await testPrometheus()).toEqual({
      ok: false,
      message: `Prometheus at ${url} answered HTTP 500.`,
    });
  });

  it("an answer without status success is not a success", async () => {
    const url = await serve((_req, res) =>
      res.writeHead(200).end(JSON.stringify({ status: "error" })),
    );
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "prometheus.url": url }),
    );

    expect((await testPrometheus()).ok).toBe(false);
  });

  it("a refusal message holds no credential", async () => {
    const url = await serve(promServer("Bearer right"));
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "prometheus.url": url,
        "prometheus.credentialsSecretRef": "prom",
      }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({ token: "tok-secret" });

    expect(await testPrometheus()).toEqual({
      ok: false,
      message: `Prometheus at ${url} refused the credentials.`,
    });
  });

  it("a Secret with neither pair names the keys", async () => {
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "prometheus.url": "http://prom:9090",
        "prometheus.credentialsSecretRef": "prom",
      }),
    );
    vi.mocked(readSecretValues).mockResolvedValue({ username: "admin" });

    expect(await testPrometheus()).toEqual({
      ok: false,
      message:
        "The Secret prom needs the keys username and password, or token.",
    });
  });

  it("an unreachable address names it", async () => {
    const url = await closedAddress();
    vi.mocked(getSettings).mockResolvedValue(
      settings({ "prometheus.url": url }),
    );

    const result = await testPrometheus();
    expect(result.ok).toBe(false);
    expect(result.message).toMatch(
      new RegExp(`^Could not reach Prometheus at ${url}: .+`),
    );
  });

  it("a Secret that cannot be read gives a failed result", async () => {
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "prometheus.url": "http://prom:9090",
        "prometheus.credentialsSecretRef": "prom",
      }),
    );
    vi.mocked(readSecretValues).mockRejectedValue(new Error("socket hang up"));

    expect(await testPrometheus()).toEqual({
      ok: false,
      message:
        "Could not read the Secret prom in namespace fleet: the server did not give a valid answer",
    });
  });

  it("settings that cannot be read give a failed result", async () => {
    vi.mocked(getSettings).mockRejectedValue(
      new KubernetesRequestError(403, "configmaps is forbidden"),
    );

    expect(await testPrometheus()).toEqual({
      ok: false,
      message: "Could not read the settings: the API server answered HTTP 403",
    });
  });

  it("no address set", async () => {
    vi.mocked(getSettings).mockResolvedValue(settings({}));

    expect(await testPrometheus()).toEqual({
      ok: false,
      message: "No Prometheus address is set.",
    });
  });
});
