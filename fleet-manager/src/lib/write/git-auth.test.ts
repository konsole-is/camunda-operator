import { generateKeyPairSync } from "node:crypto";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { readSecretValues } from "@/lib/secrets/values";
import { GIT_AUTHOR, gitCredentials } from "./git-auth";

vi.mock("@/lib/secrets/values", () => ({ readSecretValues: vi.fn() }));

const { privateKey } = generateKeyPairSync("rsa", {
  modulusLength: 2048,
  privateKeyEncoding: { type: "pkcs1", format: "pem" },
  publicKeyEncoding: { type: "spki", format: "pem" },
});

const URL_ = "https://github.com/acme/fleet.git";
const both =
  "The Secret git has keys for a token and keys for a GitHub App. Keep username and token, or appId, installationId, and privateKey.";

const unavailable = (message: string) => ({
  ok: false,
  error: { code: "Unavailable", message },
});

beforeEach(() => {
  vi.resetAllMocks();
  process.env.FLEET_NAMESPACE = "fleet";
});
afterEach(() => {
  vi.restoreAllMocks();
});

describe("gitCredentials", () => {
  it("without a Secret, no auth and the fleet-manager author", async () => {
    const result = await gitCredentials(URL_, "");

    expect(result).toEqual({
      ok: true,
      author: GIT_AUTHOR,
      forget: expect.any(Function),
    });
    expect(readSecretValues).not.toHaveBeenCalled();
  });

  it("an http:// address is Invalid and reads no Secret", async () => {
    const url = "http://git.example.com/fleet.git";

    expect(await gitCredentials(url, "git")).toEqual({
      ok: false,
      error: {
        code: "Invalid",
        path: "git.repository",
        message: `${url} must start with https:// to use the credentials of the Secret git.`,
      },
    });
    expect(readSecretValues).not.toHaveBeenCalled();
  });

  it("a Secret that cannot be read or does not exist is Unavailable", async () => {
    vi.mocked(readSecretValues).mockRejectedValueOnce(
      Object.assign(new Error("boom"), { code: "ECONNREFUSED" }),
    );
    expect(await gitCredentials(URL_, "git")).toEqual(
      unavailable(
        "Could not read the Secret git in namespace fleet: ECONNREFUSED",
      ),
    );

    vi.mocked(readSecretValues).mockResolvedValueOnce(undefined);
    expect(await gitCredentials(URL_, "git")).toEqual(
      unavailable("The Secret git does not exist in namespace fleet."),
    );
  });

  it("a token Secret gives the username and token and the fleet-manager author", async () => {
    vi.mocked(readSecretValues).mockResolvedValue({
      username: "fleet",
      token: "tok",
    });

    expect(await gitCredentials(URL_, "git")).toEqual({
      ok: true,
      auth: { username: "fleet", password: "tok" },
      author: GIT_AUTHOR,
      forget: expect.any(Function),
    });
    expect(readSecretValues).toHaveBeenCalledWith("fleet", "git");
  });

  it("a token Secret without token keeps the message that names the first missing key", async () => {
    vi.mocked(readSecretValues).mockResolvedValueOnce({ username: "fleet" });
    expect(await gitCredentials(URL_, "git")).toEqual(
      unavailable("The Secret git has no key token."),
    );

    vi.mocked(readSecretValues).mockResolvedValueOnce({});
    expect(await gitCredentials(URL_, "git")).toEqual(
      unavailable("The Secret git has no key username."),
    );
  });

  it.each([
    [
      "all keys",
      {
        username: "fleet",
        token: "tok",
        appId: "1",
        installationId: "2",
        privateKey,
      },
    ],
    ["token and appId", { token: "tok", appId: "1" }],
  ])(
    "a Secret with token keys and App keys is Unavailable and names the Secret and both sets: %s",
    async (_, values) => {
      const fetch = vi.spyOn(globalThis, "fetch");
      vi.mocked(readSecretValues).mockResolvedValue(values);

      expect(await gitCredentials(URL_, "git")).toEqual(unavailable(both));
      expect(fetch).not.toHaveBeenCalled();
    },
  );

  it.each([
    [{ appId: "1" }, "installationId and privateKey"],
    [{ appId: "1", installationId: "2" }, "privateKey"],
    [{ privateKey: "pem" }, "appId and installationId"],
  ])(
    "a Secret with only some App keys names the missing keys: %j",
    async (values, missing) => {
      vi.mocked(readSecretValues).mockResolvedValue(values);

      expect(await gitCredentials(URL_, "git")).toEqual(
        unavailable(
          `The Secret git needs the keys appId, installationId, and privateKey for a GitHub App. It has no key ${missing}.`,
        ),
      );
    },
  );

  it("a key with an empty value counts as missing", async () => {
    vi.mocked(readSecretValues).mockResolvedValue({
      username: "",
      token: "",
      appId: "1",
    });

    expect(await gitCredentials(URL_, "git")).toEqual(
      unavailable(
        "The Secret git needs the keys appId, installationId, and privateKey for a GitHub App. It has no key installationId and privateKey.",
      ),
    );
  });

  it("a complete App Secret on github.com gives x-access-token with the installation token and the bot author", async () => {
    vi.mocked(readSecretValues).mockResolvedValue({
      appId: "123",
      installationId: "801",
      privateKey,
    });
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const key = `${init?.method ?? "GET"} ${String(input)}`;
      switch (key) {
        case "POST https://api.github.com/app/installations/801/access_tokens":
          return Response.json(
            {
              token: "ghs_test",
              expires_at: new Date(Date.now() + 3_600_000).toISOString(),
            },
            { status: 201 },
          );
        case "GET https://api.github.com/app":
          return Response.json({ slug: "fleet-bot" });
        case "GET https://api.github.com/users/fleet-bot%5Bbot%5D":
          return Response.json({ id: 4242 });
      }
      throw new Error(`unexpected request ${key}`);
    });

    expect(await gitCredentials(URL_, "git")).toEqual({
      ok: true,
      auth: { username: "x-access-token", password: "ghs_test" },
      author: {
        name: "fleet-bot[bot]",
        email: "4242+fleet-bot[bot]@users.noreply.github.com",
      },
      forget: expect.any(Function),
    });
  });

  it("App keys with enterpriseHost get the token from that GitHub Enterprise Server", async () => {
    vi.mocked(readSecretValues).mockResolvedValue({
      appId: "123",
      installationId: "803",
      privateKey,
      enterpriseHost: "ghe.example.com",
    });
    const requests: string[] = [];
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const key = `${init?.method ?? "GET"} ${String(input)}`;
      requests.push(key);
      if (
        key ===
        "POST https://ghe.example.com/api/v3/app/installations/803/access_tokens"
      ) {
        return Response.json(
          {
            token: "ghs_ghe",
            expires_at: new Date(Date.now() + 3_600_000).toISOString(),
          },
          { status: 201 },
        );
      }
      return new Response(null, { status: 404 });
    });

    const result = await gitCredentials(
      "https://ghe.example.com/acme/fleet.git",
      "git",
    );

    expect(result.ok && result.auth).toEqual({
      username: "x-access-token",
      password: "ghs_ghe",
    });
    expect(requests.every((r) => r.includes("//ghe.example.com/api/v3/"))).toBe(
      true,
    );
  });

  it("App keys without enterpriseHost send nothing for a repository that is not on github.com", async () => {
    vi.mocked(readSecretValues).mockResolvedValue({
      appId: "123",
      installationId: "804",
      privateKey,
    });
    const fetch = vi.spyOn(globalThis, "fetch");

    const url = "https://ghe.example.com/acme/fleet.git";
    expect(await gitCredentials(url, "git")).toEqual(
      unavailable(
        `${url} is not on github.com. To use the GitHub App keys of the Secret git with GitHub Enterprise Server, set its key enterpriseHost to the host of the server.`,
      ),
    );
    expect(fetch).not.toHaveBeenCalled();
  });

  it("a failed App session passes its error on", async () => {
    vi.mocked(readSecretValues).mockResolvedValue({
      appId: "12a",
      installationId: "802",
      privateKey,
    });

    expect(await gitCredentials(URL_, "git")).toEqual(
      unavailable("The Secret git has a key appId that is not a number."),
    );
  });
});
