import { createVerify, generateKeyPairSync } from "node:crypto";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { type AppKeys, appJWT, appSession } from "./github-app";

const { privateKey, publicKey } = generateKeyPairSync("rsa", {
  modulusLength: 2048,
  privateKeyEncoding: { type: "pkcs1", format: "pem" },
  publicKeyEncoding: { type: "spki", format: "pem" },
});

const NOW = Date.parse("2026-10-30T12:00:00Z");
const MINUTE = 60_000;
const FALLBACK = {
  name: "fleet-manager",
  email: "fleet-manager@noreply.invalid",
};

type Call = { url: string; method: string; headers: Record<string, string> };
type Route = (call: Call) => Response | Promise<Response>;

let calls: Call[];

// route stubs fetch. Each request goes to the first route whose key
// "<METHOD> <url>" it matches; any other request fails the test.
function route(routes: Record<string, Route>) {
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    const call: Call = {
      url: String(input),
      method: init?.method ?? "GET",
      headers: Object.fromEntries(new Headers(init?.headers).entries()),
    };
    calls.push(call);
    const handler = routes[`${call.method} ${call.url}`];
    if (!handler)
      throw new Error(`unexpected request ${call.method} ${call.url}`);
    return handler(call);
  });
}

const json = (status: number, body: unknown, headers?: HeadersInit) =>
  new Response(JSON.stringify(body), { status, headers });

const token =
  (value: string, minutes = 60): Route =>
  () =>
    json(201, {
      token: value,
      expires_at: new Date(Date.now() + minutes * MINUTE).toISOString(),
    });

const keys = (installationId: string, appId = "123"): AppKeys => ({
  appId,
  installationId,
  privateKey,
});

const GITHUB = "https://github.com/acme/fleet.git";
const tokenURL = (installationId: string) =>
  `POST https://api.github.com/app/installations/${installationId}/access_tokens`;
const tokenRequests = () =>
  calls.filter((c) => c.url.endsWith("/access_tokens"));

const session = (installationId: string, url = GITHUB, k?: AppKeys) =>
  appSession({
    url,
    secret: "git",
    keys: k ?? keys(installationId),
    fallbackAuthor: FALLBACK,
  });

beforeEach(() => {
  calls = [];
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(NOW);
});
afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("appJWT", () => {
  it("signs RS256 with iss, iat 60 s back, and exp within 10 minutes", () => {
    const now = Math.floor(NOW / 1000);
    const jwt = appJWT("123", privateKey, now);

    const [header, payload, signature] = jwt.split(".");
    const verify = createVerify("RSA-SHA256");
    verify.update(`${header}.${payload}`);
    expect(verify.verify(publicKey, signature, "base64url")).toBe(true);
    expect(JSON.parse(Buffer.from(header, "base64url").toString())).toEqual({
      alg: "RS256",
      typ: "JWT",
    });
    const claims = JSON.parse(Buffer.from(payload, "base64url").toString());
    expect(claims.iss).toBe("123");
    expect(claims.iat).toBe(now - 60);
    expect(claims.exp - claims.iat).toBeLessThanOrEqual(600);
    expect(claims.exp).toBeGreaterThan(now);
  });
});

describe("appSession", () => {
  it("on github.com, asks api.github.com for an installation token with the JWT", async () => {
    route({ [tokenURL("456")]: token("ghs_one") });

    const result = await session("456");

    expect(result.ok && result.session.token).toBe("ghs_one");
    expect(tokenRequests()).toHaveLength(1);
    expect(tokenRequests()[0].headers.authorization).toMatch(/^Bearer /);
    expect(tokenRequests()[0].headers["x-github-api-version"]).toBe(
      "2022-11-28",
    );
    expect(calls.some((c) => c.url.includes("/meta"))).toBe(false);
  });

  it("uses the cached token until 5 minutes before expiry, then asks again", async () => {
    let next = "ghs_one";
    route({
      [tokenURL("457")]: () => {
        const answer = token(next)({} as Call);
        next = "ghs_two";
        return answer;
      },
    });

    await session("457");
    vi.setSystemTime(NOW + 54 * MINUTE);
    const cached = await session("457");
    expect(cached.ok && cached.session.token).toBe("ghs_one");
    expect(tokenRequests()).toHaveLength(1);

    vi.setSystemTime(NOW + 56 * MINUTE);
    const renewed = await session("457");
    expect(renewed.ok && renewed.session.token).toBe("ghs_two");
    expect(tokenRequests()).toHaveLength(2);
  });

  it("concurrent calls share one token request", async () => {
    route({ [tokenURL("458")]: token("ghs_one") });

    const [a, b] = await Promise.all([session("458"), session("458")]);

    expect(a.ok && b.ok).toBe(true);
    expect(tokenRequests()).toHaveLength(1);
  });

  it("a failed fill is not cached", async () => {
    let first = true;
    route({
      [tokenURL("459")]: () => {
        if (!first) return token("ghs_one")({} as Call);
        first = false;
        return json(500, {});
      },
    });

    expect((await session("459")).ok).toBe(false);
    expect((await session("459")).ok).toBe(true);
  });

  it("a new private key gets a new token", async () => {
    route({ [tokenURL("474")]: token("ghs_one") });
    const other = generateKeyPairSync("rsa", {
      modulusLength: 2048,
      privateKeyEncoding: { type: "pkcs1", format: "pem" },
      publicKeyEncoding: { type: "spki", format: "pem" },
    }).privateKey;

    await session("474");
    await session("474", GITHUB, { ...keys("474"), privateKey: other });

    expect(tokenRequests()).toHaveLength(2);
  });

  it("forget drops the cached token", async () => {
    route({ [tokenURL("460")]: token("ghs_one") });

    const first = await session("460");
    if (!first.ok) throw new Error("no session");
    first.forget();
    await session("460");

    expect(tokenRequests()).toHaveLength(2);
  });

  it("the author is the bot of the App", async () => {
    route({
      [tokenURL("461")]: token("ghs_one"),
      "GET https://api.github.com/app": () =>
        json(200, { slug: "fleet-bot", id: 1 }),
      "GET https://api.github.com/users/fleet-bot%5Bbot%5D": () =>
        json(200, { id: 4242 }),
    });

    const result = await session("461");

    expect(result.ok && result.session.author).toEqual({
      name: "fleet-bot[bot]",
      email: "4242+fleet-bot[bot]@users.noreply.github.com",
    });
  });

  it("a failed author lookup keeps the fallback author and the token", async () => {
    route({
      [tokenURL("462")]: token("ghs_one"),
      "GET https://api.github.com/app": () => json(500, {}),
    });

    const result = await session("462");

    expect(result.ok && result.session).toEqual({
      token: "ghs_one",
      author: FALLBACK,
    });
  });

  it("GitHub Enterprise Server: uses /api/v3 when meta answers with installed_version", async () => {
    route({
      "GET https://ghe.example.com/api/v3/meta": () =>
        json(200, { installed_version: "3.17.0" }),
      "POST https://ghe.example.com/api/v3/app/installations/463/access_tokens":
        token("ghs_ghe"),
      "GET https://ghe.example.com/api/v3/app": () =>
        json(200, { slug: "fleet-bot" }),
      "GET https://ghe.example.com/api/v3/users/fleet-bot%5Bbot%5D": () =>
        json(200, { id: 7 }),
    });

    const result = await session(
      "463",
      "https://ghe.example.com/acme/fleet.git",
    );

    expect(result.ok && result.session).toEqual({
      token: "ghs_ghe",
      author: {
        name: "fleet-bot[bot]",
        email: "7+fleet-bot[bot]@users.noreply.ghe.example.com",
      },
    });
    const meta = calls.find((c) => c.url.endsWith("/meta"));
    expect(meta?.headers.authorization).toBeUndefined();
  });

  it.each([
    ["meta 404", () => json(404, {})],
    ["meta without installed_version", () => json(200, {})],
    ["meta that is not JSON", () => new Response("<html>", { status: 200 })],
  ])(
    "sends no JWT to a host that does not answer as GitHub Enterprise Server: %s",
    async (_, meta) => {
      const url = "https://git.example.com/acme/fleet.git";
      route({ "GET https://git.example.com/api/v3/meta": meta });

      const result = await session("464", url);

      expect(result).toEqual({
        ok: false,
        error: {
          code: "Unavailable",
          message: `${url} is not on GitHub or GitHub Enterprise Server. The keys appId, installationId, and privateKey of the Secret git work only there.`,
        },
      });
      expect(calls.some((c) => c.headers.authorization)).toBe(false);
      expect(tokenRequests()).toHaveLength(0);
    },
  );

  it("a redirect from meta is not GitHub Enterprise", async () => {
    route({
      "GET https://ghe.example.com/api/v3/meta": () =>
        new Response(null, {
          status: 302,
          headers: { Location: "https://evil.example/api/v3/meta" },
        }),
    });

    const result = await session(
      "465",
      "https://ghe.example.com/acme/fleet.git",
    );

    expect(!result.ok && result.error.code).toBe("Unavailable");
    expect(calls.some((c) => c.url.includes("evil.example"))).toBe(false);
  });

  it("a refused token request holds no JWT, token, or private key", async () => {
    route({
      [tokenURL("466")]: (call) =>
        json(401, {
          message: "Bad credentials",
          echo: call.headers.authorization,
        }),
    });

    const result = await session("466");

    expect(result).toEqual({
      ok: false,
      error: {
        code: "Forbidden",
        message:
          "GitHub refused the token request of the App 123 for the installation 466 (HTTP 401).",
      },
    });
    const text = JSON.stringify(result);
    const jwt = tokenRequests()[0].headers.authorization.slice(
      "Bearer ".length,
    );
    expect(text).not.toContain(jwt);
    expect(text).not.toContain("Bad credentials");
    for (const line of privateKey.split("\n").filter((l) => l.length > 30)) {
      expect(text).not.toContain(line);
    }
  });

  it("a 201 without a token is Unavailable", async () => {
    route({
      [tokenURL("467")]: () =>
        json(201, { expires_at: new Date(NOW + 60 * MINUTE).toISOString() }),
    });

    expect(await session("467")).toEqual({
      ok: false,
      error: {
        code: "Unavailable",
        message:
          "GitHub did not answer the token request of the App 123 with a token.",
      },
    });
  });

  it("an expires_at that does not parse is Unavailable", async () => {
    route({
      [tokenURL("468")]: () =>
        json(201, { token: "ghs_one", expires_at: "soon" }),
    });

    const result = await session("468");

    expect(!result.ok && result.error.code).toBe("Unavailable");
    expect(JSON.stringify(result)).not.toContain("ghs_one");
  });

  it("an unreachable GitHub API is Unavailable and names it", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(
      Object.assign(new TypeError("fetch failed"), {
        cause: { code: "ECONNREFUSED" },
      }),
    );

    expect(await session("469")).toEqual({
      ok: false,
      error: {
        code: "Unavailable",
        message: "Could not reach https://api.github.com: ECONNREFUSED",
      },
    });
  });

  it.each([
    ["appId", keys("470", "12a")],
    ["installationId", keys("0")],
  ])("a non-number %s is Unavailable and sends nothing", async (key, k) => {
    route({});

    expect(await session("", GITHUB, k)).toEqual({
      ok: false,
      error: {
        code: "Unavailable",
        message: `The Secret git has a key ${key} that is not a number.`,
      },
    });
    expect(calls).toHaveLength(0);
  });

  it("a private key that is not RSA is Unavailable and sends nothing", async () => {
    route({});
    const ed25519 = generateKeyPairSync("ed25519", {
      privateKeyEncoding: { type: "pkcs8", format: "pem" },
      publicKeyEncoding: { type: "spki", format: "pem" },
    }).privateKey;

    const result = await session("", GITHUB, {
      ...keys("472"),
      privateKey: ed25519,
    });

    expect(!result.ok && result.error.message).toBe(
      "The Secret git has a key privateKey that is not a PEM private key.",
    );
    expect(calls).toHaveLength(0);
  });

  it("an unexpected failure is Unavailable and is not cached", async () => {
    let first = true;
    route({
      [tokenURL("473")]: () => {
        if (!first) return token("ghs_one")({} as Call);
        first = false;
        return undefined as unknown as Response;
      },
    });

    expect(await session("473")).toEqual({
      ok: false,
      error: {
        code: "Unavailable",
        message:
          "Could not get a token from GitHub: the server did not give a valid answer",
      },
    });
    expect((await session("473")).ok).toBe(true);
  });

  it("a private key that is not PEM is Unavailable and sends nothing", async () => {
    route({});

    const result = await session("", GITHUB, {
      ...keys("471"),
      privateKey: "not-a-key",
    });

    expect(result).toEqual({
      ok: false,
      error: {
        code: "Unavailable",
        message:
          "The Secret git has a key privateKey that is not a PEM private key.",
      },
    });
    expect(calls).toHaveLength(0);
  });
});
