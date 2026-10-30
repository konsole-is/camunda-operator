import { describe, expect, it } from "vitest";
import { LAST_APPLIED_ANNOTATION } from "@/lib/labels";
import { clientSideApplyPatch, lastAppliedOf } from "./merge-patch";
import type { KubeObject } from "./types";

const env = (
  spec: object,
  annotations?: Record<string, string>,
): KubeObject => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "prod", ...(annotations ? { annotations } : {}) },
  spec,
});

const applied = (object: KubeObject): KubeObject => ({
  ...object,
  metadata: {
    ...object.metadata,
    annotations: {
      ...((object.metadata.annotations ?? {}) as Record<string, string>),
      [LAST_APPLIED_ANNOTATION]: JSON.stringify(object),
    },
  },
});

describe("lastAppliedOf", () => {
  it("returns undefined for a missing object and for an object without the annotation", () => {
    expect(lastAppliedOf(undefined)).toBeUndefined();
    expect(lastAppliedOf(env({ displayName: "One" }))).toBeUndefined();
  });

  it.each(["{", "[]", "null", '"text"', "7"])(
    "returns undefined for the annotation %j, which is not a JSON object",
    (value) => {
      expect(
        lastAppliedOf(env({}, { [LAST_APPLIED_ANNOTATION]: value })),
      ).toBeUndefined();
    },
  );

  it("returns the parsed object of the annotation", () => {
    const file = env({ displayName: "One" });
    expect(lastAppliedOf(applied(file))).toEqual(file);
  });
});

describe("clientSideApplyPatch", () => {
  it("returns undefined when live has no last applied file", () => {
    expect(
      clientSideApplyPatch(env({ displayName: "One" }), env({})),
    ).toBeUndefined();
    expect(clientSideApplyPatch(undefined, env({}))).toBeUndefined();
  });

  it("sets null at a nested field of the last applied file that the body leaves out", () => {
    const live = applied(
      env({ displayName: "One", releases: { allowed: ["r1"], default: "r1" } }),
    );
    const patch = clientSideApplyPatch(
      live,
      env({ displayName: "One", releases: { default: "r1" } }),
    );
    expect(patch?.spec).toEqual({
      displayName: "One",
      releases: { allowed: null, default: "r1" },
    });
  });

  it("sets null at a whole map that the body leaves out", () => {
    const live = applied(
      env({ displayName: "One", releases: { allowed: ["r1"] } }),
    );
    expect(
      clientSideApplyPatch(live, env({ displayName: "One" }))?.spec,
    ).toEqual({ displayName: "One", releases: null });
  });

  it("sets null at a field that the body sets to undefined", () => {
    const live = applied(env({ releases: { allowed: ["r1"], default: "r1" } }));
    expect(
      clientSideApplyPatch(
        live,
        env({ releases: { allowed: undefined, default: "r1" } }),
      )?.spec,
    ).toEqual({ releases: { allowed: null, default: "r1" } });
  });

  it("sets null at a __proto__ key that the body leaves out", () => {
    const live = applied(env(JSON.parse('{"parameters":{"__proto__":"x"}}')));
    const patch = clientSideApplyPatch(live, env({ parameters: {} }));
    expect(JSON.parse(JSON.stringify(patch?.spec))).toEqual(
      JSON.parse('{"parameters":{"__proto__":null}}'),
    );
  });

  it("sends a shorter list whole, with no null for a removed item", () => {
    const live = applied(env({ releases: { allowed: ["r1", "r2"] } }));
    expect(
      clientSideApplyPatch(live, env({ releases: { allowed: ["r1"] } }))?.spec,
    ).toEqual({ releases: { allowed: ["r1"] } });
  });

  it("sets no null for a field that is on live but not in the last applied file", () => {
    const live = {
      ...applied(env({ displayName: "One" })),
      spec: { displayName: "One", releases: { allowed: ["r1"] } },
    };
    expect(
      clientSideApplyPatch(live, env({ displayName: "Two" }))?.spec,
    ).toEqual({ displayName: "Two" });
  });

  it("never removes the annotations, and keeps the last applied file at its live value", () => {
    const live = applied(env({ displayName: "One" }, { team: "a" }));
    const patch = clientSideApplyPatch(live, env({ displayName: "One" }));
    expect(patch?.metadata).toEqual({
      name: "prod",
      annotations: {
        team: null,
        [LAST_APPLIED_ANNOTATION]: (
          live.metadata.annotations as Record<string, string>
        )[LAST_APPLIED_ANNOTATION],
      },
    });
  });

  it("keeps the resourceVersion of the body, so the dry run still checks the precondition", () => {
    const live = applied(env({ displayName: "One" }));
    const body: KubeObject = {
      ...env({ displayName: "Two" }),
      metadata: { name: "prod", resourceVersion: "7" },
    };
    expect(clientSideApplyPatch(live, body)?.metadata).toMatchObject({
      resourceVersion: "7",
    });
  });
});
