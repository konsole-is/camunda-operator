import { beforeEach, describe, expect, it, vi } from "vitest";
import { getJSON, requestJSON } from "@/lib/k8s/client";
import { getSettings } from "@/lib/settings";
import { SETTING_KEYS, type Settings } from "@/lib/settings/keys";
import type { Change, WriteResult, Writer } from "@/lib/write/types";
import { getWriter } from "@/lib/write/writer";
import { externalSecretFor } from "./external-secret";
import { writeSecretValue } from "./index";

vi.mock("@/lib/write/writer", async (real) => ({
  ...(await real<typeof import("@/lib/write/writer")>()),
  getWriter: vi.fn(),
}));
vi.mock("@/lib/settings", () => ({ getSettings: vi.fn() }));
vi.mock("@/lib/k8s/client", async (real) => ({
  ...(await real<typeof import("@/lib/k8s/client")>()),
  getJSON: vi.fn(),
  requestJSON: vi.fn(),
}));

const settings = (values: Partial<Settings>): Settings => ({
  ...(Object.fromEntries(SETTING_KEYS.map((k) => [k, ""])) as Settings),
  ...values,
});

let changes: Change[];
const gitWriter: Writer = {
  mode: "git",
  preview: vi.fn(),
  apply: vi.fn(async (change: Change): Promise<WriteResult> => {
    changes.push(change);
    return { ok: true, mode: "git", commit: "abc123", revisions: {} };
  }),
};

beforeEach(() => {
  vi.clearAllMocks();
  changes = [];
  vi.mocked(getWriter).mockResolvedValue(gitWriter);
});

describe("writeSecretValue in Git mode", () => {
  it("commits an ExternalSecret and sends no API request", async () => {
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "secretStore.kind": "SecretStore",
        "secretStore.name": "vault",
      }),
    );

    const result = await writeSecretValue(
      { namespace: "orders", name: "db", key: "password" },
      "pw-123",
    );

    expect(result).toEqual({
      ok: true,
      mode: "git",
      commit: "abc123",
      revisions: {},
    });
    expect(changes).toHaveLength(1);
    expect(changes[0].apply).toEqual([
      externalSecretFor(
        { namespace: "orders", name: "db" },
        { kind: "SecretStore", name: "vault" },
      ),
    ]);
    expect(getJSON).not.toHaveBeenCalled();
    expect(requestJSON).not.toHaveBeenCalled();
  });

  it("the change holds no value", async () => {
    vi.mocked(getSettings).mockResolvedValue(
      settings({
        "secretStore.kind": "SecretStore",
        "secretStore.name": "vault",
      }),
    );

    await writeSecretValue(
      { namespace: "orders", name: "db", key: "password" },
      "pw-123",
    );

    const text = JSON.stringify(changes);
    expect(text).not.toContain("pw-123");
    expect(text).not.toContain(Buffer.from("pw-123").toString("base64"));
  });

  it.each([
    [{ "secretStore.name": "vault" }],
    [{ "secretStore.kind": "SecretStore" }],
  ])(
    "without a store gives Invalid and commits nothing (%j)",
    async (store) => {
      vi.mocked(getSettings).mockResolvedValue(settings(store));

      expect(
        await writeSecretValue(
          { namespace: "orders", name: "db", key: "password" },
          "pw",
        ),
      ).toEqual({
        ok: false,
        errors: [
          expect.objectContaining({ code: "Invalid", path: "secretStore" }),
        ],
      });
      expect(gitWriter.apply).not.toHaveBeenCalled();
    },
  );
});
