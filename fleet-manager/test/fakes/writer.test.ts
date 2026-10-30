import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Change } from "@/lib/write/types";
import { FakeWriter, fakeWriter, getWriter, writeMode } from "./writer";

vi.mock("@/lib/write/writer", () => import("./writer"));

const { runChange } = await import("@/lib/write/run-change");

const claim = (name: string) => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  metadata: { name, namespace: "team-a" },
  spec: { cluster: { zeebe: { replicas: 3 } } },
});

const change = (name = "orders"): Change => ({
  intent: `Change ${name}`,
  apply: [claim(name)],
  preconditions: { [`CamundaClusterClaim/team-a/${name}`]: "41" },
});

beforeEach(() => fakeWriter.reset());

describe("FakeWriter", () => {
  it("records each apply and returns the scripted results in order", async () => {
    const writer = new FakeWriter();
    writer.results = [
      { ok: false, errors: [{ code: "Stale", message: "x" }] },
      { ok: true, mode: "direct" },
    ];

    expect(await writer.apply(change("a"))).toEqual({
      ok: false,
      errors: [{ code: "Stale", message: "x" }],
    });
    expect(await writer.apply(change("b"))).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(writer.applied).toEqual([change("a"), change("b")]);
  });

  it("records each preview and returns the scripted previews in order", async () => {
    const writer = new FakeWriter();
    const refused = {
      objects: [],
      errors: [{ code: "Invalid" as const, message: "no" }],
    };
    writer.previews = [refused];

    expect(await writer.preview(change())).toEqual(refused);
    expect(await writer.preview(change())).toEqual({ objects: [], errors: [] });
    expect(writer.previewed).toEqual([change(), change()]);
    expect(writer.applied).toEqual([]);
  });

  it("returns ok in its mode without a script", async () => {
    expect(await new FakeWriter().apply(change())).toEqual({
      ok: true,
      mode: "direct",
    });

    const git = new FakeWriter("git");
    expect(git.mode).toBe("git");
    expect(
      await git.apply({ intent: "x", apply: [claim("a"), claim("b")] }),
    ).toEqual({
      ok: true,
      mode: "git",
      commit: "0000000000000000000000000000000000000001",
      revisions: {
        "CamundaClusterClaim/team-a/a": expect.any(String),
        "CamundaClusterClaim/team-a/b": expect.any(String),
      },
      paths: {
        "CamundaClusterClaim/team-a/a": "team-a/camundaclusterclaim/a.yaml",
        "CamundaClusterClaim/team-a/b": "team-a/camundaclusterclaim/b.yaml",
      },
      deleted: {},
    });
  });

  it("records a deep copy, so a later edit of the change does not change the record", async () => {
    const writer = new FakeWriter();
    const sent = change();
    await writer.apply(sent);
    await writer.preview(sent);
    (sent.apply[0].spec as { cluster: unknown }).cluster = {};

    expect(writer.applied).toEqual([change()]);
    expect(writer.previewed).toEqual([change()]);
  });

  it("reset empties the records and the scripts and sets the mode", async () => {
    const writer = new FakeWriter("git");
    writer.results = [{ ok: true, mode: "direct" }];
    writer.previews = [{ objects: [], errors: [] }];
    await writer.apply(change());
    await writer.preview(change());

    writer.reset();
    expect(writer.mode).toBe("direct");
    expect(writer.applied).toEqual([]);
    expect(writer.previewed).toEqual([]);
    expect(writer.results).toEqual([]);
    expect(writer.previews).toEqual([]);

    writer.reset("git");
    expect(writer.mode).toBe("git");
  });

  it("getWriter and writeMode give the shared fake and its mode", async () => {
    fakeWriter.reset("git");
    expect(await getWriter()).toBe(fakeWriter);
    expect(await writeMode()).toBe("git");
  });

  it("runChange writes through it when it stands in for the writer module", async () => {
    expect(await runChange(change())).toEqual({ ok: true, mode: "direct" });
    expect(fakeWriter.applied).toEqual([change()]);
  });
});
