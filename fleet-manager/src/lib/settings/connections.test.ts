import http from "node:http";
import type { AddressInfo } from "node:net";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { readSecretValues } from "@/lib/secrets/values";
import { SETTING_KEYS, type Settings } from "./keys";
import { getSettings } from "./store";
import { testGit, testPrometheus } from "./connections";

vi.mock("./store", () => ({ getSettings: vi.fn() }));
vi.mock("@/lib/secrets/values", () => ({ readSecretValues: vi.fn() }));

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
});

describe("testGit", () => {
  it("connects with the token of the Secret", async () => {
    const url =
      (await serve(gitServer(["main"], basic("bot", "tok-1")))) + "/fleet.git";
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
      (await serve(gitServer(["main"], basic("bot", "right")))) + "/fleet.git";
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
      (await serve((req, res) => {
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
});

// promServer answers the query vector(1) when the Authorization header is
// auth, and 401 with the header it got otherwise.
function promServer(auth: string | undefined): Handler {
  return (req, res) => {
    if (req.headers.authorization !== auth) {
      res.writeHead(401).end(`bad ${req.headers.authorization}`);
      return;
    }
    if (req.url !== "/api/v1/query?query=vector(1)") {
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

  it("no address set", async () => {
    vi.mocked(getSettings).mockResolvedValue(settings({}));

    expect(await testPrometheus()).toEqual({
      ok: false,
      message: "No Prometheus address is set.",
    });
  });
});
