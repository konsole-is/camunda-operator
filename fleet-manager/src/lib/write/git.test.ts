import { execFile } from "node:child_process";
import { generateKeyPairSync } from "node:crypto";
import { chmod, mkdtemp, readdir, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";
import type { HttpClient } from "isomorphic-git";
import gitHttp from "isomorphic-git/http/node";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { parse } from "yaml";
import { LAST_APPLIED_ANNOTATION, REVISION_ANNOTATION } from "@/lib/labels";
import { readSecretValues } from "@/lib/secrets/values";
import {
  type BareRepository,
  serveBareRepository,
} from "../../../test/git/server";
import { type GitTarget, gitWriter } from "./git";
import type {
  Change,
  KubeObject,
  ObjectRef,
  Preview,
  WriteResult,
  Writer,
} from "./types";
import { refKey } from "./writer";

vi.mock("@/lib/secrets/values", () => ({ readSecretValues: vi.fn() }));

const claim = (
  namespace: string,
  name: string,
  preset = "small",
): KubeObject => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  metadata: { name, namespace },
  spec: { presetRef: preset },
});
const environment = (name: string): KubeObject => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name },
  spec: { displayName: name },
});
const namespace = (name: string): KubeObject => ({
  apiVersion: "v1",
  kind: "Namespace",
  metadata: { name },
});
const claimRef = (ns: string, name: string): ObjectRef => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  namespace: ns,
  name,
});

let repo: BareRepository;
let live: Map<string, KubeObject>;
let directPreview: ReturnType<typeof vi.fn<Writer["preview"]>>;
let directApply: ReturnType<typeof vi.fn<Writer["apply"]>>;

const direct = (): Writer => ({
  mode: "direct",
  preview: directPreview,
  apply: directApply,
});

const writerFor = (
  target: Partial<GitTarget> = {},
  http?: HttpClient,
): Writer =>
  gitWriter({
    target: {
      url: repo.url,
      branch: "",
      path: "",
      credentialsSecretRef: "",
      ...target,
    },
    direct: direct(),
    read: async (ref) => live.get(refKey(ref)),
    ...(http ? { http } : {}),
  });

// recordingFetches sends each request and adds the body of each fetch of
// objects to bodies.
const recordingFetches = (bodies: string[]): HttpClient => ({
  request: async (r) => {
    if (!r.url.endsWith("/git-upload-pack") || !r.body) {
      return gitHttp.request(r);
    }
    const chunks: Uint8Array[] = [];
    for await (const chunk of r.body) chunks.push(chunk);
    const body = Buffer.concat(chunks);
    bodies.push(body.toString("utf8"));
    // The test server fails on a streamed body, and the transport sends
    // an array body in one piece.
    return gitHttp.request({
      ...r,
      body: [body] as unknown as AsyncIterableIterator<Uint8Array>,
    });
  },
});

const ok = (result: WriteResult) => {
  if (!result.ok || result.mode !== "git") {
    throw new Error(`not a Git result: ${JSON.stringify(result)}`);
  }
  return result;
};

beforeEach(async () => {
  vi.resetAllMocks();
  process.env.FLEET_NAMESPACE = "fleet";
  live = new Map();
  directPreview = vi.fn<Writer["preview"]>(async (): Promise<Preview> => ({
    objects: [],
    errors: [],
  }));
  directApply = vi.fn<Writer["apply"]>(async () => ({
    ok: true,
    mode: "direct",
  }));
});

afterEach(async () => {
  await repo?.close();
});

// These tests start a local Git server. On a busy machine they need more
// than the default 5 s.
const slow = { timeout: 30_000 };

describe("the Git writer", slow, () => {
  it("one change is one commit with the intent as subject", async () => {
    repo = await serveBareRepository();

    const result = await writerFor().apply({
      intent: "Create the claims orders and billing",
      apply: [claim("team-a", "orders"), claim("team-a", "billing")],
    });

    expect(ok(result).deleted).toEqual({});
    expect(await repo.log()).toEqual([
      "Create the claims orders and billing",
      "Initial commit",
    ]);
  });

  it("writes each object at its contract path with the revision annotation", async () => {
    repo = await serveBareRepository();

    const result = ok(
      await writerFor({ path: "clusters/prod" }).apply({
        intent: "Create",
        apply: [claim("team-a", "orders"), environment("prod")],
      }),
    );

    const claimPath = "clusters/prod/team-a/camundaclusterclaims/orders.yaml";
    const envPath = "clusters/prod/_cluster/environments/prod.yaml";
    expect(await repo.files()).toEqual(
      expect.arrayContaining([claimPath, envPath]),
    );
    const file = parse((await repo.show(claimPath))!);
    expect(file.metadata.annotations["fleet.camunda.io/revision"]).toBe(
      result.revisions["CamundaClusterClaim/team-a/orders"],
    );
    expect(file.spec).toEqual({ presetRef: "small" });
    expect(result.paths).toEqual({
      "CamundaClusterClaim/team-a/orders": claimPath,
      "Environment//prod": envPath,
    });
    expect(result.commit).toMatch(/^[0-9a-f]{40}$/);
  });

  it("a namespace and its claim land in one commit", async () => {
    repo = await serveBareRepository();

    ok(
      await writerFor().apply({
        intent: "Create team-b with the claim orders",
        apply: [namespace("team-b"), claim("team-b", "orders")],
      }),
    );

    expect(await repo.log()).toHaveLength(2);
    expect(await repo.files()).toEqual(
      expect.arrayContaining([
        "_cluster/namespaces/team-b.yaml",
        "team-b/camundaclusterclaims/orders.yaml",
      ]),
    );
  });

  it("skips the dry run of an object in a namespace that the change creates, and still dry-runs the namespace", async () => {
    repo = await serveBareRepository();
    const preview = await writerFor().preview({
      intent: "Create team-b with the claim orders",
      apply: [namespace("team-b"), claim("team-b", "orders")],
    });

    expect(directPreview).toHaveBeenCalledWith(
      expect.objectContaining({ apply: [namespace("team-b")] }),
    );
    expect(preview.objects).toEqual([
      expect.objectContaining({
        ref: claimRef("team-b", "orders"),
        after: expect.objectContaining({ spec: { presetRef: "small" } }),
      }),
    ]);
  });

  it("dry-runs an object in a namespace that exists already", async () => {
    repo = await serveBareRepository();
    live.set("Namespace//team-b", namespace("team-b"));

    await writerFor().preview({
      intent: "Create",
      apply: [namespace("team-b"), claim("team-b", "orders")],
    });

    expect(directPreview).toHaveBeenCalledWith(
      expect.objectContaining({
        apply: [namespace("team-b"), claim("team-b", "orders")],
      }),
    );
  });

  it("skips the dry run of an ExternalSecret and commits it", async () => {
    repo = await serveBareRepository();
    const externalSecret: KubeObject = {
      apiVersion: "external-secrets.io/v1",
      kind: "ExternalSecret",
      metadata: { name: "db", namespace: "team-a" },
      spec: { refreshInterval: "1h" },
    };

    const result = await writerFor().apply({
      intent: "Set the secret db",
      apply: [externalSecret],
    });

    ok(result);
    expect(directPreview).not.toHaveBeenCalled();
    expect(await repo.files()).toContain("team-a/externalsecrets/db.yaml");
  });

  it("a directOnly change goes to the direct writer and writes nothing to Git", async () => {
    repo = await serveBareRepository();
    const change: Change = {
      intent: "Back up now",
      directOnly: true,
      apply: [environment("prod")],
    };

    expect(await writerFor().apply(change)).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(directApply).toHaveBeenCalledWith(change);
    expect(await repo.log()).toEqual(["Initial commit"]);
  });

  it("a change that the dry run refuses makes no commit and returns the API errors", async () => {
    repo = await serveBareRepository();
    const refusal = {
      code: "Invalid" as const,
      message: "releases.default must be one of releases.allowed",
    };
    directPreview.mockResolvedValue({ objects: [], errors: [refusal] });

    expect(
      await writerFor().apply({
        intent: "Change",
        apply: [environment("prod")],
      }),
    ).toEqual({ ok: false, errors: [refusal] });
    expect(await repo.log()).toEqual(["Initial commit"]);
  });

  it("a stale precondition makes no commit and returns Stale", async () => {
    repo = await serveBareRepository();
    live.set("CamundaClusterClaim/team-a/orders", {
      ...claim("team-a", "orders"),
      metadata: { name: "orders", namespace: "team-a", resourceVersion: "8" },
    });

    const result = await writerFor().apply({
      intent: "Change",
      apply: [claim("team-a", "orders", "large")],
      preconditions: { "CamundaClusterClaim/team-a/orders": "7" },
    });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Stale" })],
    });
    expect(await repo.log()).toEqual(["Initial commit"]);
    expect(directPreview).not.toHaveBeenCalled();
  });

  it("a second save from the same read fails with Conflict while GitOps has not applied the first", async () => {
    repo = await serveBareRepository();
    live.set("CamundaClusterClaim/team-a/orders", {
      ...claim("team-a", "orders"),
      metadata: { name: "orders", namespace: "team-a", resourceVersion: "7" },
    });
    const preconditions = { "CamundaClusterClaim/team-a/orders": "7" };
    ok(
      await writerFor().apply({
        intent: "First",
        apply: [claim("team-a", "orders", "large")],
        preconditions,
      }),
    );

    const second = await writerFor().apply({
      intent: "Second",
      apply: [claim("team-a", "orders", "medium")],
      preconditions,
    });

    expect(second).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Conflict",
          ref: claimRef("team-a", "orders"),
        }),
      ],
    });
    expect(await repo.log()).toEqual(["First", "Initial commit"]);
    const file = parse(
      (await repo.show("team-a/camundaclusterclaims/orders.yaml"))!,
    );
    expect(file.spec.presetRef).toBe("large");
  });

  it("while GitOps does not apply a commit, a save from a fresh read still fails with Conflict that names the way out", async () => {
    repo = await serveBareRepository();
    live.set("CamundaClusterClaim/team-a/orders", {
      ...claim("team-a", "orders"),
      metadata: { name: "orders", namespace: "team-a", resourceVersion: "7" },
    });
    ok(
      await writerFor().apply({
        intent: "Never applied",
        apply: [claim("team-a", "orders", "large")],
        preconditions: { "CamundaClusterClaim/team-a/orders": "7" },
      }),
    );

    const fresh = live.get("CamundaClusterClaim/team-a/orders")!;
    const result = await writerFor().apply({
      intent: "Later",
      apply: [claim("team-a", "orders", "medium")],
      preconditions: {
        "CamundaClusterClaim/team-a/orders": fresh.metadata.resourceVersion!,
      },
    });

    expect(result).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Conflict",
          message: expect.stringContaining(
            "If GitOps cannot apply it, correct or revert team-a/camundaclusterclaims/orders.yaml in the repository.",
          ),
        }),
      ],
    });
    expect(await repo.log()).toEqual(["Never applied", "Initial commit"]);
  });

  it("a save from an older read fails with Stale when GitOps applies the previous save during the clone", async () => {
    repo = await serveBareRepository();
    live.set("CamundaClusterClaim/team-a/orders", {
      ...claim("team-a", "orders"),
      metadata: { name: "orders", namespace: "team-a", resourceVersion: "7" },
    });
    const preconditions = { "CamundaClusterClaim/team-a/orders": "7" };
    const first = ok(
      await writerFor().apply({
        intent: "First",
        apply: [claim("team-a", "orders", "large")],
        preconditions,
      }),
    );
    let applied = false;
    const gitOpsAppliesOnClone: HttpClient = {
      request: (r) => {
        if (!applied) {
          applied = true;
          live.set("CamundaClusterClaim/team-a/orders", {
            ...claim("team-a", "orders", "large"),
            metadata: {
              name: "orders",
              namespace: "team-a",
              resourceVersion: "8",
              annotations: {
                [REVISION_ANNOTATION]:
                  first.revisions["CamundaClusterClaim/team-a/orders"],
              },
            },
          });
        }
        return gitHttp.request(r);
      },
    };

    const second = await writerFor({}, gitOpsAppliesOnClone).apply({
      intent: "Second",
      apply: [claim("team-a", "orders", "medium")],
      preconditions,
    });

    expect(second).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Stale" })],
    });
    expect(await repo.log()).toEqual(["First", "Initial commit"]);
  });

  it("a save after GitOps applied the previous save commits", async () => {
    repo = await serveBareRepository();
    live.set("CamundaClusterClaim/team-a/orders", {
      ...claim("team-a", "orders"),
      metadata: { name: "orders", namespace: "team-a", resourceVersion: "7" },
    });
    const first = ok(
      await writerFor().apply({
        intent: "First",
        apply: [claim("team-a", "orders", "large")],
        preconditions: { "CamundaClusterClaim/team-a/orders": "7" },
      }),
    );
    live.set("CamundaClusterClaim/team-a/orders", {
      ...claim("team-a", "orders", "large"),
      metadata: {
        name: "orders",
        namespace: "team-a",
        resourceVersion: "8",
        annotations: {
          [REVISION_ANNOTATION]:
            first.revisions["CamundaClusterClaim/team-a/orders"],
        },
      },
    });

    ok(
      await writerFor().apply({
        intent: "Second",
        apply: [claim("team-a", "orders", "medium")],
        preconditions: { "CamundaClusterClaim/team-a/orders": "8" },
      }),
    );

    expect(await repo.log()).toEqual(["Second", "First", "Initial commit"]);
  });

  it("a precondition for a deleted object returns Stale", async () => {
    repo = await serveBareRepository();

    const result = await writerFor().apply({
      intent: "Change",
      apply: [claim("team-a", "orders", "large")],
      preconditions: { "CamundaClusterClaim/team-a/orders": "7" },
    });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Stale" })],
    });
    expect(await repo.log()).toEqual(["Initial commit"]);
  });

  it("refuses a Secret and commits nothing", async () => {
    repo = await serveBareRepository();

    const result = await writerFor().apply({
      intent: "Set",
      apply: [
        {
          apiVersion: "v1",
          kind: "Secret",
          metadata: { name: "db", namespace: "team-a" },
          stringData: { password: "pw-123" },
        },
      ],
    });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Invalid" })],
    });
    expect(JSON.stringify(result)).not.toContain("pw-123");
    expect(await repo.log()).toEqual(["Initial commit"]);
    expect(directPreview).not.toHaveBeenCalled();
  });

  it("a delete removes the file in one commit and returns the file of the deleted object", async () => {
    repo = await serveBareRepository();
    await writerFor().apply({
      intent: "Create",
      apply: [claim("team-a", "orders")],
    });

    const result = ok(
      await writerFor().apply({
        intent: "Delete the claim orders",
        apply: [],
        delete: [claimRef("team-a", "orders")],
      }),
    );

    expect(await repo.log()).toEqual([
      "Delete the claim orders",
      "Create",
      "Initial commit",
    ]);
    expect(await repo.files()).toEqual(["README.md"]);
    expect(result.revisions).toEqual({});
    expect(result.paths).toEqual({});
    expect(result.deleted).toEqual({
      "CamundaClusterClaim/team-a/orders":
        "team-a/camundaclusterclaims/orders.yaml",
    });
  });

  it("a change that writes and deletes returns the revision of the written object and the file of the deleted one", async () => {
    repo = await serveBareRepository();
    await writerFor().apply({
      intent: "Create",
      apply: [claim("team-a", "orders")],
    });

    const result = ok(
      await writerFor().apply({
        intent: "Replace orders with prod",
        apply: [environment("prod")],
        delete: [claimRef("team-a", "orders")],
      }),
    );

    expect(Object.keys(result.revisions)).toEqual(["Environment//prod"]);
    expect(result.paths).toEqual({
      "Environment//prod": "_cluster/environments/prod.yaml",
    });
    expect(result.deleted).toEqual({
      "CamundaClusterClaim/team-a/orders":
        "team-a/camundaclusterclaims/orders.yaml",
    });
  });

  it("an object that the change applies and deletes is only in deleted", async () => {
    repo = await serveBareRepository();
    await writerFor().apply({
      intent: "Create",
      apply: [claim("team-a", "orders")],
    });

    const result = ok(
      await writerFor().apply({
        intent: "Apply and delete orders",
        apply: [claim("team-a", "orders")],
        delete: [claimRef("team-a", "orders")],
      }),
    );

    expect(result.revisions).toEqual({});
    expect(result.paths).toEqual({});
    expect(result.deleted).toEqual({
      "CamundaClusterClaim/team-a/orders":
        "team-a/camundaclusterclaims/orders.yaml",
    });
  });

  it("a delete of an object without a file fails and commits nothing", async () => {
    repo = await serveBareRepository();

    const result = await writerFor().apply({
      intent: "Delete",
      apply: [],
      delete: [claimRef("team-a", "orders")],
    });

    expect(result).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Invalid",
          message: expect.stringContaining(
            "team-a/camundaclusterclaims/orders.yaml",
          ),
          ref: claimRef("team-a", "orders"),
        }),
      ],
    });
    expect(await repo.log()).toEqual(["Initial commit"]);
  });

  it("a change whose files are already in the repository makes no commit", async () => {
    repo = await serveBareRepository();
    const change: Change = {
      intent: "Create",
      apply: [claim("team-a", "orders")],
    };

    const first = ok(await writerFor().apply(change));
    const second = ok(await writerFor().apply(change));

    expect(await repo.log()).toEqual(["Create", "Initial commit"]);
    expect(second.revisions).toEqual(first.revisions);
    expect(second.commit).toBe(first.commit);
  });

  it("after a push that another clone beat, the writer rebases once and pushes", async () => {
    repo = await serveBareRepository({
      beforeFirstPush: () =>
        repo.commitFromOtherClone({ "other.txt": "x" }, "Other admin"),
    });

    ok(
      await writerFor().apply({
        intent: "Create",
        apply: [claim("team-a", "orders")],
      }),
    );

    expect(await repo.log()).toEqual([
      "Create",
      "Other admin",
      "Initial commit",
    ]);
    expect(await repo.show("other.txt")).toBe("x");
    expect(await repo.files()).toContain(
      "team-a/camundaclusterclaims/orders.yaml",
    );
  });

  it("a push that another clone beat on the same file fails with Conflict and leaves the other commit", async () => {
    repo = await serveBareRepository({
      beforeFirstPush: () =>
        repo.commitFromOtherClone(
          { "team-a/camundaclusterclaims/orders.yaml": "x" },
          "Other admin",
        ),
    });

    const result = await writerFor().apply({
      intent: "Create",
      apply: [claim("team-a", "orders")],
    });

    expect(result).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Conflict",
          message: expect.stringContaining("Open the page again"),
        }),
      ],
    });
    expect(await repo.log()).toEqual(["Other admin", "Initial commit"]);
  });

  it("fetches only the head commit, and rebases once on a repository with history", async () => {
    repo = await serveBareRepository({
      beforeFirstPush: () =>
        repo.commitFromOtherClone({ "other.txt": "x" }, "Other admin"),
    });
    await repo.commitFromOtherClone({ "a.txt": "a" }, "Older commit");
    await repo.commitFromOtherClone({ "b.txt": "b" }, "Newer commit");
    const fetches: string[] = [];

    ok(
      await writerFor({}, recordingFetches(fetches)).apply({
        intent: "Create",
        apply: [claim("team-a", "orders")],
      }),
    );

    expect(fetches).toHaveLength(2);
    for (const body of fetches) expect(body).toContain("deepen 1");
    expect(await repo.log()).toEqual([
      "Create",
      "Other admin",
      "Newer commit",
      "Older commit",
      "Initial commit",
    ]);
  });

  it("a push from a shallow clone that another clone beat on the same file fails with Conflict", async () => {
    repo = await serveBareRepository({
      beforeFirstPush: () =>
        repo.commitFromOtherClone(
          { "team-a/camundaclusterclaims/orders.yaml": "x" },
          "Other admin",
        ),
    });
    await repo.commitFromOtherClone({ "a.txt": "a" }, "Older commit");
    await repo.commitFromOtherClone({ "b.txt": "b" }, "Newer commit");

    const result = await writerFor().apply({
      intent: "Create",
      apply: [claim("team-a", "orders")],
    });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Conflict" })],
    });
    expect(await repo.show("team-a/camundaclusterclaims/orders.yaml")).toBe(
      "x",
    );
  });

  it("a push that the server refuses on a branch that did not move gives Unavailable", async () => {
    repo = await serveBareRepository();
    const hook = path.join(repo.dir, "hooks", "pre-receive");
    await writeFile(hook, "#!/bin/sh\nexit 1\n");
    await chmod(hook, 0o755);

    const result = await writerFor().apply({
      intent: "Create",
      apply: [claim("team-a", "orders")],
    });

    expect(result).toEqual({
      ok: false,
      errors: [
        {
          code: "Unavailable",
          message: `${repo.url} refused the push to the branch main.`,
        },
      ],
    });
    expect(await repo.log()).toEqual(["Initial commit"]);
  });

  it("refuses a name with a path separator and writes nothing", async () => {
    repo = await serveBareRepository();

    const result = await writerFor().apply({
      intent: "Set the secret",
      apply: [
        {
          apiVersion: "external-secrets.io/v1",
          kind: "ExternalSecret",
          metadata: { name: "../../../escaped", namespace: "team-a" },
        },
      ],
    });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Invalid" })],
    });
    expect(await repo.log()).toEqual(["Initial commit"]);
  });

  it("refuses to write through a symbolic link of the repository", async () => {
    repo = await serveBareRepository();
    const outside = await mkdtemp(path.join(os.tmpdir(), "fleet-outside-"));
    await repo.linkFromOtherClone("team-a", outside, "Link team-a");

    const result = await writerFor().apply({
      intent: "Create",
      apply: [claim("team-a", "orders")],
    });

    expect(result).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Invalid",
          ref: claimRef("team-a", "orders"),
        }),
      ],
    });
    expect(await readdir(outside)).toEqual([]);
    expect(await repo.log()).toEqual(["Link team-a", "Initial commit"]);
    await rm(outside, { recursive: true, force: true });
  });

  it("forces the dry run, so a field that a GitOps tool applied does not block the save", async () => {
    repo = await serveBareRepository();

    ok(
      await writerFor().apply({
        intent: "Change",
        apply: [environment("prod")],
      }),
    );

    expect(directPreview).toHaveBeenCalledWith(
      expect.objectContaining({ force: true }),
    );
  });

  it("dry-runs an object that GitOps applied as the manager of GitOps, with the live revision", async () => {
    repo = await serveBareRepository();
    const envRef = refKey({ kind: "Environment", name: "prod" });
    live.set(envRef, {
      ...environment("prod"),
      metadata: {
        name: "prod",
        annotations: { [REVISION_ANNOTATION]: "applied" },
        managedFields: [
          {
            manager: "kustomize-controller",
            operation: "Apply",
            fieldsV1: {
              "f:metadata": {
                "f:annotations": { [`f:${REVISION_ANNOTATION}`]: {} },
              },
            },
          },
        ],
      },
    });

    ok(
      await writerFor().apply({
        intent: "Change",
        apply: [environment("prod")],
      }),
    );

    expect(directPreview).toHaveBeenCalledWith(
      expect.objectContaining({
        apply: [
          expect.objectContaining({
            metadata: {
              name: "prod",
              annotations: { [REVISION_ANNOTATION]: "applied" },
            },
          }),
        ],
        previewManagers: { [envRef]: "kustomize-controller" },
      }),
    );
  });

  it("dry-runs an object that GitOps applied with client-side apply as a merge patch, with the live revision", async () => {
    repo = await serveBareRepository();
    const envRef = refKey({ kind: "Environment", name: "prod" });
    live.set(envRef, {
      ...environment("prod"),
      metadata: {
        name: "prod",
        annotations: {
          [REVISION_ANNOTATION]: "applied",
          [LAST_APPLIED_ANNOTATION]: JSON.stringify(environment("prod")),
        },
      },
    });

    ok(
      await writerFor().apply({
        intent: "Change",
        apply: [environment("prod")],
      }),
    );

    expect(directPreview).toHaveBeenCalledWith(
      expect.objectContaining({
        apply: [
          expect.objectContaining({
            metadata: {
              name: "prod",
              annotations: { [REVISION_ANNOTATION]: "applied" },
            },
          }),
        ],
        previewManagers: {},
        previewMergePatches: [envRef],
      }),
    );
  });

  it("dry-runs an object with a server-side-apply GitOps manager as that manager, also when it has a last applied file", async () => {
    repo = await serveBareRepository();
    const envRef = refKey({ kind: "Environment", name: "prod" });
    live.set(envRef, {
      ...environment("prod"),
      metadata: {
        name: "prod",
        annotations: {
          [REVISION_ANNOTATION]: "applied",
          [LAST_APPLIED_ANNOTATION]: JSON.stringify(environment("prod")),
        },
        managedFields: [
          {
            manager: "kustomize-controller",
            operation: "Apply",
            fieldsV1: {
              "f:metadata": {
                "f:annotations": { [`f:${REVISION_ANNOTATION}`]: {} },
              },
            },
          },
        ],
      },
    });

    ok(
      await writerFor().apply({
        intent: "Change",
        apply: [environment("prod")],
      }),
    );

    expect(directPreview).toHaveBeenCalledWith(
      expect.objectContaining({
        previewManagers: { [envRef]: "kustomize-controller" },
        previewMergePatches: [],
      }),
    );
  });

  it("dry-runs a Namespace with a last applied file with server-side apply", async () => {
    repo = await serveBareRepository();
    live.set(refKey({ kind: "Namespace", name: "team" }), {
      ...namespace("team"),
      metadata: {
        name: "team",
        annotations: {
          [LAST_APPLIED_ANNOTATION]: JSON.stringify(namespace("team")),
        },
      },
    });

    ok(
      await writerFor().apply({
        intent: "Change",
        apply: [namespace("team")],
      }),
    );

    expect(directPreview).toHaveBeenCalledWith(
      expect.objectContaining({
        apply: [namespace("team")],
        previewManagers: {},
        previewMergePatches: [],
      }),
    );
  });

  it("dry-runs an object that GitOps did not apply with server-side apply as the app", async () => {
    repo = await serveBareRepository();
    live.set(
      refKey({ kind: "Environment", name: "prod" }),
      environment("prod"),
    );

    ok(
      await writerFor().apply({
        intent: "Change",
        apply: [environment("prod")],
      }),
    );

    expect(directPreview).toHaveBeenCalledWith(
      expect.objectContaining({
        apply: [environment("prod")],
        previewManagers: {},
      }),
    );
  });

  it("a failed read of an object makes no commit and returns the error, also when the dry run would pass", async () => {
    repo = await serveBareRepository();
    let reads = 0;
    const writer = gitWriter({
      target: { url: repo.url, branch: "", path: "", credentialsSecretRef: "" },
      direct: direct(),
      read: async () => {
        reads += 1;
        if (reads === 1) throw new Error("connection reset");
        return undefined;
      },
    });

    const result = await writer.apply({
      intent: "Change",
      apply: [environment("prod")],
    });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Unavailable" })],
    });
    expect(await repo.log()).toEqual(["Initial commit"]);
  });

  it("a delete of an ExternalSecret skips the dry run and removes the file", async () => {
    repo = await serveBareRepository();
    const ref: ObjectRef = {
      apiVersion: "external-secrets.io/v1",
      kind: "ExternalSecret",
      namespace: "team-a",
      name: "db",
    };
    await writerFor().apply({
      intent: "Set",
      apply: [
        {
          apiVersion: ref.apiVersion,
          kind: ref.kind,
          metadata: { name: "db", namespace: "team-a" },
        },
      ],
    });

    ok(
      await writerFor().apply({
        intent: "Remove the secret db",
        apply: [],
        delete: [ref],
      }),
    );

    expect(directPreview).not.toHaveBeenCalled();
    expect(await repo.files()).toEqual(["README.md"]);
  });

  it("a missing branch gives Unavailable and commits nothing", async () => {
    repo = await serveBareRepository();

    const result = await writerFor({ branch: "release" }).apply({
      intent: "Create",
      apply: [claim("team-a", "orders")],
    });

    expect(result).toEqual({
      ok: false,
      errors: [
        {
          code: "Unavailable",
          message: `${repo.url} has no branch release.`,
        },
      ],
    });
  });

  it("a tag in git.branch gives Unavailable and commits nothing", async () => {
    repo = await serveBareRepository();
    await promisify(execFile)("git", ["-C", repo.dir, "tag", "v1", "main"]);

    const result = await writerFor({ branch: "v1" }).apply({
      intent: "Create",
      apply: [claim("team-a", "orders")],
    });

    expect(result).toEqual({
      ok: false,
      errors: [
        { code: "Unavailable", message: `${repo.url} has no branch v1.` },
      ],
    });
    expect(await repo.log()).toEqual(["Initial commit"]);
  });

  it("refuses a git.path that leaves the repository", async () => {
    repo = await serveBareRepository();

    const result = await writerFor({ path: "../x" }).apply({
      intent: "Create",
      apply: [claim("team-a", "orders")],
    });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Invalid", path: "git.path" })],
    });
  });
});

describe("the credentials of the Git writer", slow, () => {
  // The bare repository speaks plain HTTP. This transport sends an
  // https:// address to it over HTTP, so the writer sees an HTTPS
  // repository.
  const overHttp: HttpClient = {
    request: (r) =>
      gitHttp.request({ ...r, url: r.url.replace(/^https:/, "http:") }),
  };

  it("sends the username and token of the Secret, and the message of a refusal holds no token", async () => {
    repo = await serveBareRepository({
      auth: { username: "fleet", token: "right-token" },
    });
    const url = repo.url.replace(/^http:/, "https:");
    const change: Change = {
      intent: "Create",
      apply: [claim("team-a", "orders")],
    };

    vi.mocked(readSecretValues).mockResolvedValue({
      username: "fleet",
      token: "right-token",
    });
    ok(
      await writerFor(
        { url, credentialsSecretRef: "git-token" },
        overHttp,
      ).apply(change),
    );
    expect(readSecretValues).toHaveBeenCalledWith("fleet", "git-token");

    vi.mocked(readSecretValues).mockResolvedValue({
      username: "fleet",
      token: "wrong-token",
    });
    const refused = await writerFor(
      { url, credentialsSecretRef: "git-token" },
      overHttp,
    ).apply({ ...change, apply: [claim("team-a", "billing")] });

    expect(refused).toEqual({
      ok: false,
      errors: [
        { code: "Forbidden", message: `${url} refused the credentials.` },
      ],
    });
    expect(JSON.stringify(refused)).not.toContain("wrong-token");
  });

  it("refuses to send credentials to an http:// address", async () => {
    repo = await serveBareRepository();
    const request = vi.fn<HttpClient["request"]>();
    vi.mocked(readSecretValues).mockResolvedValue({
      username: "fleet",
      token: "t",
    });

    const result = await writerFor(
      { credentialsSecretRef: "git-token" },
      { request },
    ).apply({ intent: "Create", apply: [claim("team-a", "orders")] });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Invalid" })],
    });
    expect(request).not.toHaveBeenCalled();
    expect(readSecretValues).not.toHaveBeenCalled();
  });

  it("a Secret without the key token gives Unavailable that names the Secret and the key", async () => {
    repo = await serveBareRepository();
    vi.mocked(readSecretValues).mockResolvedValue({ username: "fleet" });

    const result = await writerFor(
      {
        url: repo.url.replace(/^http:/, "https:"),
        credentialsSecretRef: "git-token",
      },
      overHttp,
    ).apply({ intent: "Create", apply: [claim("team-a", "orders")] });

    expect(result).toEqual({
      ok: false,
      errors: [
        {
          code: "Unavailable",
          message: "The Secret git-token has no key token.",
        },
      ],
    });
  });

  describe("with GitHub App keys", () => {
    const { privateKey } = generateKeyPairSync("rsa", {
      modulusLength: 2048,
      privateKeyEncoding: { type: "pkcs1", format: "pem" },
      publicKeyEncoding: { type: "spki", format: "pem" },
    });
    const appKeys = (installationId: string) => ({
      appId: "123",
      installationId,
      privateKey,
    });
    let tokenRequests: { authorization: string }[];

    // githubFor answers as GitHub Enterprise Server on the host of the
    // bare repository. Each token request gets the next of tokens. app
    // answers GET /app.
    const githubFor = (tokens: string[], app: () => Response) => {
      const api = `${new URL(repo.url).origin.replace(/^http:/, "https:")}/api/v3`;
      tokenRequests = [];
      vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
        const url = String(input);
        const method = init?.method ?? "GET";
        if (method === "GET" && url === `${api}/meta`) {
          return Response.json({ installed_version: "3.17.0" });
        }
        if (method === "POST" && url.endsWith("/access_tokens")) {
          tokenRequests.push({
            authorization: new Headers(init?.headers).get("authorization")!,
          });
          return Response.json(
            {
              token: tokens.shift(),
              expires_at: new Date(Date.now() + 3_600_000).toISOString(),
            },
            { status: 201 },
          );
        }
        if (method === "GET" && url === `${api}/app`) return app();
        if (method === "GET" && url === `${api}/users/fleet-bot%5Bbot%5D`) {
          return Response.json({ id: 4242 });
        }
        throw new Error(`unexpected request ${method} ${url}`);
      });
    };
    const bot = () => Response.json({ slug: "fleet-bot" });
    const authorOf = async () =>
      (
        await promisify(execFile)(
          "git",
          ["log", "-1", "--format=%an <%ae>", "main"],
          { cwd: repo.dir },
        )
      ).stdout.trim();
    const save = (name: string) =>
      writerFor(
        {
          url: repo.url.replace(/^http:/, "https:"),
          credentialsSecretRef: "git",
        },
        overHttp,
      ).apply({ intent: `Create ${name}`, apply: [claim("team-a", name)] });

    afterEach(() => {
      vi.restoreAllMocks();
    });

    it("pushes with GitHub App keys and the commit shows the App bot as the author", async () => {
      repo = await serveBareRepository({
        auth: { username: "x-access-token", token: "ghs_test" },
      });
      githubFor(["ghs_test"], bot);
      vi.mocked(readSecretValues).mockResolvedValue(appKeys("901"));

      ok(await save("orders"));

      expect(await authorOf()).toBe(
        "fleet-bot[bot] <4242+fleet-bot[bot]@users.noreply.127.0.0.1>",
      );
    });

    it("a failed author lookup commits as fleet-manager and the save does not fail", async () => {
      repo = await serveBareRepository({
        auth: { username: "x-access-token", token: "ghs_test" },
      });
      githubFor(["ghs_test"], () => new Response(null, { status: 500 }));
      vi.mocked(readSecretValues).mockResolvedValue(appKeys("902"));

      ok(await save("orders"));

      expect(await authorOf()).toBe(
        "fleet-manager <fleet-manager@noreply.invalid>",
      );
    });

    it("a Secret with token keys and App keys gives Unavailable and commits nothing", async () => {
      repo = await serveBareRepository();
      const before = await repo.log();
      vi.mocked(readSecretValues).mockResolvedValue({
        username: "fleet",
        token: "t",
        ...appKeys("903"),
      });

      expect(await save("orders")).toEqual({
        ok: false,
        errors: [
          {
            code: "Unavailable",
            message:
              "The Secret git has keys for a token and keys for a GitHub App. Keep username and token, or appId, installationId, and privateKey.",
          },
        ],
      });
      expect(await repo.log()).toEqual(before);
    });

    it("a refused push with an App token drops the cached token", async () => {
      repo = await serveBareRepository({
        auth: { username: "x-access-token", token: "ghs_new" },
      });
      githubFor(["ghs_old", "ghs_new"], bot);
      vi.mocked(readSecretValues).mockResolvedValue(appKeys("904"));
      const url = repo.url.replace(/^http:/, "https:");

      const refused = await save("orders");

      expect(refused).toEqual({
        ok: false,
        errors: [
          { code: "Forbidden", message: `${url} refused the credentials.` },
        ],
      });
      const jwt = tokenRequests[0].authorization.slice("Bearer ".length);
      expect(JSON.stringify(refused)).not.toContain("ghs_old");
      expect(JSON.stringify(refused)).not.toContain(jwt);

      ok(await save("billing"));
      expect(tokenRequests).toHaveLength(2);
    });
  });
});
