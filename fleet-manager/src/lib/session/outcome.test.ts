import { describe, expect, it } from "vitest";
import type { Step } from "@/lib/fields";
import { CLAIM_LABEL } from "@/lib/labels";
import { outcomeOf, STALE_CLAIM_MESSAGE } from "./outcome";
import {
  openSession,
  resetValue,
  setValue,
  type ClaimObject,
  type Session,
} from "./session";

const ref = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  namespace: "team-a",
  name: "orders",
};

function session(): Session {
  const claim: ClaimObject = {
    apiVersion: ref.apiVersion,
    kind: ref.kind,
    metadata: { name: "orders", namespace: "team-a", resourceVersion: "41" },
    spec: { cluster: { zeebe: { replicas: 3 } } },
  };
  const s = openSession({
    claim,
    resource: { metadata: { labels: { [CLAIM_LABEL]: "orders" } } },
    locks: () => undefined,
  })!;
  const result = setValue(
    s,
    [
      { field: "spec" },
      { field: "cluster" },
      { field: "zeebe" },
      { field: "replicas" },
    ],
    -1,
  );
  if (!("session" in result)) throw new Error("refused");
  return result.session;
}

describe("outcomeOf", () => {
  it("an ok write and a clean preview are ok", () => {
    const s = session();
    expect(outcomeOf({ ok: true, mode: "direct" }, s)).toEqual({ ok: true });
    expect(outcomeOf({ objects: [], errors: [] }, s)).toEqual({ ok: true });
  });

  it("Stale gives the claim text and keeps the pending changes", () => {
    const s = session();
    expect(
      outcomeOf(
        {
          ok: false,
          errors: [
            { code: "Stale", message: "The object changed.", ref },
            { code: "Invalid", message: "other" },
          ],
        },
        s,
      ),
    ).toEqual({
      ok: false,
      stale: true,
      fields: [],
      messages: [STALE_CLAIM_MESSAGE],
    });
    expect(s.pending.size).toBe(1);
  });

  it("Stale in Git mode gives the same text", () => {
    const s = session();
    expect(
      outcomeOf(
        {
          ok: false,
          errors: [{ code: "Stale", message: "The file changed in Git.", ref }],
        },
        s,
      ),
    ).toEqual({
      ok: false,
      stale: true,
      fields: [],
      messages: [STALE_CLAIM_MESSAGE],
    });
  });

  it("an Invalid error with the path of a pending field goes to that field", () => {
    const s = session();
    const outcome = {
      ok: false,
      stale: false,
      fields: [
        { path: "spec.cluster.zeebe.replicas", message: "must be >= 1" },
      ],
      messages: [],
    };
    expect(
      outcomeOf(
        {
          ok: false,
          errors: [
            {
              code: "Invalid",
              message: "must be >= 1",
              path: "spec.cluster.zeebe.replicas",
            },
          ],
        },
        s,
      ),
    ).toEqual(outcome);
    // The API server can name the path without the leading spec.
    expect(
      outcomeOf(
        {
          objects: [],
          errors: [
            {
              code: "Invalid",
              message: "must be >= 1",
              path: "cluster.zeebe.replicas",
            },
          ],
        },
        s,
      ),
    ).toEqual(outcome);
  });

  it("an error at an index path or below a pending object goes to its pending field", () => {
    const claim: ClaimObject = {
      apiVersion: ref.apiVersion,
      kind: ref.kind,
      metadata: { name: "orders", namespace: "team-a", resourceVersion: "41" },
      spec: {
        cluster: {
          zeebe: { replicas: 3 },
          extraEnv: [{ name: "LOG", value: "info" }],
        },
      },
    };
    let s = openSession({
      claim,
      resource: { metadata: { labels: { [CLAIM_LABEL]: "orders" } } },
      locks: () => undefined,
    })!;
    const env: Step[] = [
      { field: "spec" },
      { field: "cluster" },
      { field: "extraEnv" },
      { item: { name: "A" } },
    ];
    const zeebe: Step[] = [
      { field: "spec" },
      { field: "cluster" },
      { field: "zeebe" },
    ];
    for (const [steps, value] of [
      [env, { name: "A", value: "" }],
      [zeebe, { replicas: -1 }],
    ] as const) {
      const result = setValue(s, [...steps], value);
      if (!("session" in result)) throw new Error("refused");
      s = result.session;
    }
    expect(
      outcomeOf(
        {
          objects: [],
          errors: [
            {
              code: "Invalid",
              message: "must not be empty",
              path: "spec.cluster.extraEnv[1].value",
            },
            {
              code: "Invalid",
              message: "must be >= 1",
              path: "cluster.zeebe.replicas",
            },
            {
              code: "Invalid",
              message: "other item",
              path: "spec.cluster.extraEnv[0].value",
            },
          ],
        },
        s,
      ),
    ).toEqual({
      ok: false,
      stale: false,
      fields: [
        { path: "spec.cluster.extraEnv[name=A]", message: "must not be empty" },
        { path: "spec.cluster.zeebe", message: "must be >= 1" },
      ],
      messages: ["other item"],
    });
  });

  it("an error at an index after a reset of an earlier item goes to the edited item", () => {
    const claim: ClaimObject = {
      apiVersion: ref.apiVersion,
      kind: ref.kind,
      metadata: { name: "orders", namespace: "team-a", resourceVersion: "41" },
      spec: { cluster: { extraEnvFrom: [{ a: "1" }, { b: "2" }] } },
    };
    const item = (index: number): Step[] => [
      { field: "spec" },
      { field: "cluster" },
      { field: "extraEnvFrom" },
      { index },
    ];
    const s0 = openSession({
      claim,
      resource: { metadata: { labels: { [CLAIM_LABEL]: "orders" } } },
      locks: () => undefined,
    })!;
    const reset = resetValue(s0, item(0));
    if (!("session" in reset)) throw new Error("refused");
    const set = setValue(reset.session, item(1), { b: "" });
    if (!("session" in set)) throw new Error("refused");
    expect(
      outcomeOf(
        {
          objects: [],
          errors: [
            {
              code: "Invalid",
              message: "must not be empty",
              path: "spec.cluster.extraEnvFrom[0].b",
            },
          ],
        },
        set.session,
      ),
    ).toEqual({
      ok: false,
      stale: false,
      fields: [
        { path: "spec.cluster.extraEnvFrom[1]", message: "must not be empty" },
      ],
      messages: [],
    });
  });

  it("an item that a reset empties and a change fills keeps its index", () => {
    const claim: ClaimObject = {
      apiVersion: ref.apiVersion,
      kind: ref.kind,
      metadata: { name: "orders", namespace: "team-a", resourceVersion: "41" },
      spec: { cluster: { extraEnvFrom: [{ a: "1" }, { b: "2" }] } },
    };
    const at = (...steps: Step[]): Step[] => [
      { field: "spec" },
      { field: "cluster" },
      { field: "extraEnvFrom" },
      ...steps,
    ];
    let s = openSession({
      claim,
      resource: { metadata: { labels: { [CLAIM_LABEL]: "orders" } } },
      locks: () => undefined,
    })!;
    for (const [steps, value] of [
      [at({ index: 1 }), { b: "" }],
      [at({ index: 0 }, { field: "a" }), undefined],
      [at({ index: 0 }, { field: "c" }), "3"],
    ] as const) {
      const result =
        value === undefined
          ? resetValue(s, [...steps])
          : setValue(s, [...steps], value);
      if (!("session" in result)) throw new Error("refused");
      s = result.session;
    }
    expect(
      outcomeOf(
        {
          objects: [],
          errors: [
            {
              code: "Invalid",
              message: "must not be empty",
              path: "spec.cluster.extraEnvFrom[1].b",
            },
            {
              code: "Invalid",
              message: "too short",
              path: "spec.cluster.extraEnvFrom[0].c",
            },
          ],
        },
        s,
      ),
    ).toEqual({
      ok: false,
      stale: false,
      fields: [
        { path: "spec.cluster.extraEnvFrom[1]", message: "must not be empty" },
        { path: "spec.cluster.extraEnvFrom[0].c", message: "too short" },
      ],
      messages: [],
    });
  });

  it("any other error is a message", () => {
    expect(
      outcomeOf(
        {
          ok: false,
          errors: [
            { code: "Invalid", message: "bad", path: "spec.presetRef" },
            { code: "Forbidden", message: "no" },
          ],
        },
        session(),
      ),
    ).toEqual({ ok: false, stale: false, fields: [], messages: ["bad", "no"] });
  });
});
