import { randomUUID } from "node:crypto";
import {
  KubeConfig,
  KubernetesObjectApi,
  PatchStrategy,
} from "@kubernetes/client-node";
import { afterEach, beforeEach, describe, expect, inject, it } from "vitest";
import { REVISION_ANNOTATION } from "@/lib/labels";
import { gitWriter } from "@/lib/write/git";
import type { KubeObject, Writer } from "@/lib/write/types";
import { directWriter, readObject, refKey } from "@/lib/write/writer";
import { type BareRepository, serveBareRepository } from "../git/server";

const kc = new KubeConfig();
kc.loadFromFile(inject("kubeconfig"));
const api = KubernetesObjectApi.makeApiClient(kc);
const direct = directWriter(kc);

const environment = (name: string, spec: object): KubeObject => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name },
  spec,
});

let repo: BareRepository;
let writer: Writer;
let name: string;

beforeEach(async () => {
  repo = await serveBareRepository();
  writer = gitWriter({
    target: {
      url: repo.url,
      branch: "",
      path: "fleet",
      credentialsSecretRef: "",
    },
    direct,
    read: (ref) => readObject(kc, ref),
  });
  name = `env-${randomUUID().slice(0, 8)}`;
});

afterEach(async () => {
  await repo.close();
});

describe("the Git writer against envtest", () => {
  it("a change that a CEL rule refuses makes no commit and returns the API message", async () => {
    const result = await writer.apply({
      intent: "Create the environment",
      apply: [
        environment(name, {
          displayName: "One",
          releases: { allowed: ["r1"], default: "r2" },
        }),
      ],
    });

    expect(result.ok).toBe(false);
    expect(!result.ok && result.errors[0]).toEqual(
      expect.objectContaining({
        code: "Invalid",
        message: expect.stringContaining(
          "releases.default must be one of releases.allowed",
        ),
      }),
    );
    expect(await repo.log()).toEqual(["Initial commit"]);
  });

  it("a change with a stale precondition makes no commit and returns Stale", async () => {
    await direct.apply({
      intent: "Create",
      apply: [environment(name, { displayName: "One" })],
    });
    const first = await readObject(kc, {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      name,
    });
    await direct.apply({
      intent: "Change",
      apply: [environment(name, { displayName: "Two" })],
    });

    const result = await writer.apply({
      intent: "Change the display name",
      apply: [environment(name, { displayName: "Three" })],
      preconditions: {
        [refKey({ kind: "Environment", name })]:
          first!.metadata.resourceVersion!,
      },
    });

    expect(result).toEqual({
      ok: false,
      errors: [expect.objectContaining({ code: "Stale" })],
    });
    expect(await repo.log()).toEqual(["Initial commit"]);
  });

  it("a second save of a field that a GitOps tool applied commits", async () => {
    // A GitOps tool applies the first commit under its own field manager.
    await api.patch(
      environment(name, { displayName: "One" }),
      undefined,
      undefined,
      "gitops",
      true,
      PatchStrategy.ServerSideApply,
    );

    const result = await writer.apply({
      intent: "Change the display name",
      apply: [environment(name, { displayName: "Two" })],
    });

    expect(result).toEqual(expect.objectContaining({ ok: true, mode: "git" }));
    expect(await repo.log()).toEqual([
      "Change the display name",
      "Initial commit",
    ]);
  });

  it("a save that leaves out a field that only GitOps applied is checked without it", async () => {
    // GitOps applied a file with releases.allowed. The save leaves it out,
    // so the GitOps apply of the commit removes it and the CEL rule fails.
    await api.patch(
      {
        ...environment(name, {
          displayName: "One",
          releases: { allowed: ["r1"], default: "r1" },
        }),
        metadata: { name, annotations: { [REVISION_ANNOTATION]: "one" } },
      },
      undefined,
      undefined,
      "gitops",
      true,
      PatchStrategy.ServerSideApply,
    );

    const result = await writer.apply({
      intent: "Drop the allowed releases",
      apply: [
        environment(name, { displayName: "One", releases: { default: "r1" } }),
      ],
    });

    expect(result.ok).toBe(false);
    expect(!result.ok && result.errors[0]).toEqual(
      expect.objectContaining({
        code: "Invalid",
        message: expect.stringContaining(
          "releases.default must be one of releases.allowed",
        ),
      }),
    );
    expect(await repo.log()).toEqual(["Initial commit"]);
  });

  it("a save keeps a field that another manager applied, as the GitOps apply does", async () => {
    await api.patch(
      {
        ...environment(name, { displayName: "One" }),
        metadata: { name, annotations: { [REVISION_ANNOTATION]: "one" } },
      },
      undefined,
      undefined,
      "gitops",
      true,
      PatchStrategy.ServerSideApply,
    );
    await api.patch(
      environment(name, { releases: { allowed: ["r1"] } }),
      undefined,
      undefined,
      "someone-else",
      true,
      PatchStrategy.ServerSideApply,
    );

    const result = await writer.apply({
      intent: "Set the default release",
      apply: [
        environment(name, { displayName: "One", releases: { default: "r1" } }),
      ],
    });

    expect(result).toEqual(expect.objectContaining({ ok: true, mode: "git" }));
  });

  it("a directOnly change writes nothing to Git and creates the object in envtest", async () => {
    expect(
      await writer.apply({
        intent: "Create the environment",
        directOnly: true,
        apply: [environment(name, { displayName: "One" })],
      }),
    ).toEqual({ ok: true, mode: "direct" });

    const object = await api.read({
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      metadata: { name },
    });
    expect(object.metadata?.managedFields).toContainEqual(
      expect.objectContaining({ manager: "fleet-manager" }),
    );
    expect(await repo.log()).toEqual(["Initial commit"]);
  });

  it("a new namespace and its claim land in one commit", async () => {
    const namespace = `team-${randomUUID().slice(0, 8)}`;

    const result = await writer.apply({
      intent: `Create ${namespace} with the claim orders`,
      apply: [
        { apiVersion: "v1", kind: "Namespace", metadata: { name: namespace } },
        {
          apiVersion: "fleet.camunda.io/v1alpha1",
          kind: "CamundaClusterClaim",
          metadata: { name: "orders", namespace },
          spec: { presetRef: "small", releaseRef: "camunda-8-9" },
        },
      ],
    });

    expect(result).toEqual(expect.objectContaining({ ok: true, mode: "git" }));
    expect(await repo.log()).toEqual([
      `Create ${namespace} with the claim orders`,
      "Initial commit",
    ]);
    expect(await repo.files()).toEqual(
      expect.arrayContaining([
        `fleet/_cluster/namespaces/${namespace}.yaml`,
        `fleet/${namespace}/camundaclusterclaims/orders.yaml`,
      ]),
    );
    expect(
      await readObject(kc, {
        apiVersion: "v1",
        kind: "Namespace",
        name: namespace,
      }),
    ).toBeUndefined();
  });
});
