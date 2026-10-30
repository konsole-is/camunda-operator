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
    expect(patch?.spec).toEqual({ releases: { allowed: null } });
  });

  it("sets null at a whole map that the body leaves out", () => {
    const live = applied(
      env({ displayName: "One", releases: { allowed: ["r1"] } }),
    );
    expect(
      clientSideApplyPatch(live, env({ displayName: "One" }))?.spec,
    ).toEqual({ releases: null });
  });

  it("sets null at a field that the body sets to undefined", () => {
    const live = applied(env({ releases: { allowed: ["r1"], default: "r1" } }));
    expect(
      clientSideApplyPatch(
        live,
        env({ releases: { allowed: undefined, default: "r1" } }),
      )?.spec,
    ).toEqual({ releases: { allowed: null } });
  });

  it("sets null at a __proto__ key that the body leaves out", () => {
    const live = applied(env(JSON.parse('{"parameters":{"__proto__":"x"}}')));
    const patch = clientSideApplyPatch(live, env({ parameters: {} }));
    expect(JSON.parse(JSON.stringify(patch?.spec))).toEqual(
      JSON.parse('{"parameters":{"__proto__":null}}'),
    );
  });

  it("sends no null for a field that is null in the last applied file and in the body", () => {
    const live = applied(env({ parameters: { keep: null, size: "s" } }));
    expect(
      clientSideApplyPatch(live, env({ parameters: { keep: null, size: "m" } }))
        ?.spec,
    ).toEqual({ parameters: { size: "m" } });
  });

  it("sends the null of the body for a field that the last applied file holds with a value", () => {
    const live = applied(env({ parameters: { drop: "x" } }));
    expect(
      clientSideApplyPatch(live, env({ parameters: { drop: null } }))?.spec,
    ).toEqual({ parameters: { drop: null } });
  });

  it("sends the null of the body when the last applied file holds a value, also when live is null", () => {
    const live = {
      ...applied(env({ parameters: { keep: "x" } })),
      spec: { parameters: { keep: null } },
    };
    expect(
      clientSideApplyPatch(live, env({ parameters: { keep: null } }))?.spec,
    ).toEqual({ parameters: { keep: null } });
  });

  it("sends no null of the body when the last applied file holds null, also when live holds a value", () => {
    const live = {
      ...applied(env({ parameters: { keep: null } })),
      spec: { parameters: { keep: "x" } },
    };
    expect(
      clientSideApplyPatch(live, env({ parameters: { keep: null } }))?.spec,
    ).toBeUndefined();
  });

  it("leaves out a map that holds only unchanged nulls, also when live holds no map there", () => {
    const live = {
      ...applied(env({ parameters: { option: { keep: null } } })),
      spec: { parameters: { option: "s" } },
    };
    expect(
      clientSideApplyPatch(
        live,
        env({ parameters: { option: { keep: null } } }),
      )?.spec,
    ).toBeUndefined();
  });

  it("sends an empty map of the body that live does not hold", () => {
    const live = {
      ...applied(env({ parameters: { option: "s" } })),
      spec: { parameters: { option: "s" } },
    };
    expect(
      clientSideApplyPatch(live, env({ parameters: { option: {} } }))?.spec,
    ).toEqual({ parameters: { option: {} } });
  });

  it("removes the null fields of an object in a new list, and keeps a null item", () => {
    const live = applied(env({ displayName: "One" }));
    expect(
      clientSideApplyPatch(
        live,
        env({ displayName: "One", items: [{ name: "a", value: null }, null] }),
      )?.spec,
    ).toEqual({ items: [{ name: "a" }, null] });
  });

  it("sends a change and a removal of the same map together", () => {
    const live = applied(
      env({ displayName: "One", releases: { allowed: ["r1"] } }),
    );
    expect(
      clientSideApplyPatch(live, env({ displayName: "Two" }))?.spec,
    ).toEqual({ displayName: "Two", releases: null });
    const other = applied(env({ é: 1 }));
    expect(clientSideApplyPatch(other, env({ ü: 2 }))?.spec).toEqual({
      é: null,
      ü: 2,
    });
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

  it("never removes the annotations, and leaves the last applied file at its live value", () => {
    const live = applied(env({ displayName: "One" }, { team: "a" }));
    const patch = clientSideApplyPatch(live, env({ displayName: "One" }));
    expect(patch?.metadata).toEqual({ annotations: { team: null } });
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
