import { describe, expect, it } from "vitest";
import { CLAIM_LABEL } from "@/lib/labels";
import type { FieldLock, Step } from "@/lib/fields";
import type { LockProvider } from "./locks";
import {
  changeOf,
  claimAfter,
  discard,
  openSession,
  rebase,
  resetValue,
  setValue,
  type ClaimObject,
  type Session,
  type SetResult,
} from "./session";

const f = (field: string): Step => ({ field });
const cluster = (...names: string[]): Step[] => [
  f("spec"),
  f("cluster"),
  ...names.map(f),
];
const ACCESS: Step[] = [
  f("spec"),
  f("plugins"),
  { item: { section: "access", registration: "gateway-access" } },
];

function claim(resourceVersion = "41"): ClaimObject {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { name: "orders", namespace: "team-a", resourceVersion },
    spec: {
      presetRef: "medium",
      cluster: {
        zeebe: { replicas: 3 },
        extraEnv: [{ name: "LOG", value: "info" }],
      },
      plugins: [
        {
          section: "access",
          registration: "gateway-access",
          parameters: { hostnamePattern: "{name}.a.example.com" },
        },
      ],
    },
  };
}

const MANAGED = { metadata: { labels: { [CLAIM_LABEL]: "orders" } } };
const releaseLocks: LockProvider = (path) =>
  path === "spec.cluster.version" ? { reason: "release" } : undefined;

function open(c: ClaimObject = claim(), locks = releaseLocks): Session {
  const s = openSession({ claim: c, resource: MANAGED, locks });
  expect(s).toBeDefined();
  return s!;
}

function ok<C extends ClaimObject>(result: SetResult<C>): Session<C> {
  expect(result).toHaveProperty("session");
  return (result as { session: Session<C> }).session;
}

function withSpec(spec: Record<string, unknown>): ClaimObject {
  return { ...claim(), spec: { ...claim().spec, ...spec } };
}

describe("openSession", () => {
  it("opens a session with no pending changes", () => {
    const s = open();
    expect(s.claim).toEqual(claim());
    expect(s.pending.size).toBe(0);
  });

  it("has no session without the claim label on the resource", () => {
    expect(
      openSession({ claim: claim(), resource: {}, locks: releaseLocks }),
    ).toBeUndefined();
    expect(
      openSession({
        claim: claim(),
        resource: { metadata: { labels: { [CLAIM_LABEL]: "" } } },
        locks: releaseLocks,
      }),
    ).toBeUndefined();
  });

  it("has no session without a claim", () => {
    expect(
      openSession({ claim: undefined, resource: MANAGED, locks: releaseLocks }),
    ).toBeUndefined();
  });

  it("has no session for a claim without resourceVersion", () => {
    expect(
      openSession({ claim: claim(""), resource: MANAGED, locks: releaseLocks }),
    ).toBeUndefined();
    const metadata = { ...claim().metadata };
    delete metadata.resourceVersion;
    expect(
      openSession({
        claim: { ...claim(), metadata },
        resource: MANAGED,
        locks: releaseLocks,
      }),
    ).toBeUndefined();
  });
});

describe("setValue", () => {
  it("sets a scalar field", () => {
    const s = ok(setValue(open(), cluster("zeebe", "replicas"), 5));
    expect(claimAfter(s)).toEqual(
      withSpec({
        cluster: {
          zeebe: { replicas: 5 },
          extraEnv: [{ name: "LOG", value: "info" }],
        },
      }),
    );
    expect([...s.pending.keys()]).toEqual(["spec.cluster.zeebe.replicas"]);
  });

  it("sets a field that the claim does not hold yet", () => {
    const s = ok(setValue(open(), cluster("gateway", "replicas"), 2));
    expect(claimAfter(s)).toEqual(
      withSpec({
        cluster: {
          zeebe: { replicas: 3 },
          extraEnv: [{ name: "LOG", value: "info" }],
          gateway: { replicas: 2 },
        },
      }),
    );
    expect([...s.pending.keys()]).toEqual(["spec.cluster.gateway.replicas"]);
  });

  it("sets a keyed list item", () => {
    const s = ok(
      setValue(open(), [...cluster("extraEnv"), { item: { name: "TZ" } }], {
        name: "TZ",
        value: "UTC",
      }),
    );
    expect(claimAfter(s)).toEqual(
      withSpec({
        cluster: {
          zeebe: { replicas: 3 },
          extraEnv: [
            { name: "LOG", value: "info" },
            { name: "TZ", value: "UTC" },
          ],
        },
      }),
    );
    expect([...s.pending.keys()]).toEqual(["spec.cluster.extraEnv[name=TZ]"]);
  });

  it("sets a section parameter", () => {
    const s = ok(
      setValue(
        open(),
        [...ACCESS, f("parameters"), f("hostnamePattern")],
        "{name}.b.example.com",
      ),
    );
    expect(claimAfter(s)).toEqual(
      withSpec({
        plugins: [
          {
            section: "access",
            registration: "gateway-access",
            parameters: { hostnamePattern: "{name}.b.example.com" },
          },
        ],
      }),
    );
    expect([...s.pending.keys()]).toEqual([
      "spec.plugins[section=access,registration=gateway-access].parameters.hostnamePattern",
    ]);
  });

  it("sets a top-level claim field", () => {
    const s = ok(setValue(open(), [f("spec"), f("presetRef")], "large"));
    expect(claimAfter(s)).toEqual(withSpec({ presetRef: "large" }));
    expect([...s.pending.keys()]).toEqual(["spec.presetRef"]);
  });

  it("a change back to the current value removes the entry", () => {
    const changed = ok(setValue(open(), cluster("zeebe", "replicas"), 5));
    const back = ok(setValue(changed, cluster("zeebe", "replicas"), 3));
    expect(back.pending.size).toBe(0);
    expect(claimAfter(back)).toEqual(claim());
  });

  it("does not change the session it gets", () => {
    const s = open();
    ok(setValue(s, cluster("zeebe", "replicas"), 5));
    expect(s.pending.size).toBe(0);
  });

  it("keeps its entry when the caller edits the steps or the value later", () => {
    const steps = cluster("gateway");
    const value = { replicas: 2 };
    const s = ok(setValue(open(), steps, value));
    steps.push(f("mode"));
    value.replicas = 9;
    expect(s.pending.get("spec.cluster.gateway")).toEqual({
      path: "spec.cluster.gateway",
      steps: cluster("gateway"),
      value: { replicas: 2 },
    });
  });

  it("a value of undefined is a reset", () => {
    const s = ok(setValue(open(), cluster("zeebe", "replicas"), undefined));
    expect(s.pending.get("spec.cluster.zeebe.replicas")).toEqual({
      path: "spec.cluster.zeebe.replicas",
      steps: cluster("zeebe", "replicas"),
      reset: true,
    });
  });
});

describe("resetValue", () => {
  it("reset removes the claim value", () => {
    const s = ok(resetValue(open(), cluster("zeebe", "replicas")));
    expect(claimAfter(s)).toEqual(
      withSpec({ cluster: { extraEnv: [{ name: "LOG", value: "info" }] } }),
    );
    expect([...s.pending.keys()]).toEqual(["spec.cluster.zeebe.replicas"]);
  });

  it("reset of a value the claim does not hold leaves no entry", () => {
    const changed = ok(setValue(open(), cluster("gateway", "replicas"), 2));
    const s = ok(resetValue(changed, cluster("gateway", "replicas")));
    expect(s.pending.size).toBe(0);
    expect(claimAfter(s)).toEqual(claim());
  });
});

describe("locks", () => {
  it("a locked field never enters the session", () => {
    const s = open();
    const refused: FieldLock = { reason: "release" };
    expect(setValue(s, cluster("version"), "8.9.9")).toEqual({ refused });
    expect(resetValue(s, cluster("version"))).toEqual({ refused });
    expect(s.pending.size).toBe(0);
  });

  it("a session over a CamundaManagementClusterClaim keeps locked fields out", () => {
    const plane: ClaimObject = {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaManagementClusterClaim",
      metadata: { name: "plane", namespace: "ops", resourceVersion: "7" },
      spec: { platformConfigRef: "aws" },
    };
    const s = open(plane, (p) =>
      p === "spec.platformConfigRef"
        ? { reason: "claimController" }
        : undefined,
    );
    expect(setValue(s, [f("spec"), f("platformConfigRef")], "gcp")).toEqual({
      refused: { reason: "claimController" },
    });
    const suspended = ok(setValue(s, [f("spec"), f("suspend")], true));
    expect([...suspended.pending.keys()]).toEqual(["spec.suspend"]);
    expect(changeOf(suspended, "Suspend plane").apply).toEqual([
      { ...plane, spec: { platformConfigRef: "aws", suspend: true } },
    ]);
  });
});

describe("overlapping changes", () => {
  it("a change below a pending change goes into that change", () => {
    let s = ok(setValue(open(), cluster("zeebe"), { replicas: 5 }));
    s = ok(setValue(s, cluster("zeebe", "replicas"), 3));
    expect(s.pending.size).toBe(0);
    expect(claimAfter(s)).toEqual(claim());

    s = ok(setValue(open(), cluster("zeebe"), { replicas: 5 }));
    s = ok(setValue(s, cluster("zeebe", "replicas"), 7));
    expect([...s.pending.keys()]).toEqual(["spec.cluster.zeebe"]);
    expect(
      (claimAfter(s).spec as { cluster: { zeebe: unknown } }).cluster.zeebe,
    ).toEqual({ replicas: 7 });
  });

  it("a reset of the last item of a pending list removes the list", () => {
    let s = ok(
      setValue(open(), cluster("extraEnv"), [{ name: "TZ", value: "UTC" }]),
    );
    s = ok(resetValue(s, [...cluster("extraEnv"), { item: { name: "TZ" } }]));
    expect([...s.pending.values()]).toEqual([
      {
        path: "spec.cluster.extraEnv",
        steps: cluster("extraEnv"),
        reset: true,
      },
    ]);
    expect(
      (claimAfter(s).spec as { cluster: Record<string, unknown> }).cluster,
    ).not.toHaveProperty("extraEnv");
  });

  it("a reset of a field that a pending value lacks keeps that value", () => {
    let s = ok(setValue(open(), cluster("podLabels"), {}));
    s = ok(resetValue(s, cluster("podLabels", "team")));
    expect([...s.pending.values()]).toEqual([
      {
        path: "spec.cluster.podLabels",
        steps: cluster("podLabels"),
        value: {},
      },
    ]);
  });

  it("a change of a parent drops the pending changes below it", () => {
    let s = ok(setValue(open(), cluster("zeebe", "replicas"), 5));
    s = ok(setValue(s, cluster("zeebe"), { replicas: 4 }));
    expect([...s.pending.keys()]).toEqual(["spec.cluster.zeebe"]);

    s = ok(setValue(open(), cluster("zeebe", "replicas"), 5));
    s = ok(resetValue(s, cluster("zeebe")));
    expect([...s.pending.keys()]).toEqual(["spec.cluster.zeebe"]);
    expect(
      (claimAfter(s).spec as { cluster: Record<string, unknown> }).cluster,
    ).not.toHaveProperty("zeebe");
  });
});

describe("locks above a field", () => {
  const admin: FieldLock = { reason: "admin" };
  const hostLocked: LockProvider = (p) =>
    p ===
    "spec.plugins[section=access,registration=gateway-access].parameters.hostnamePattern"
      ? admin
      : undefined;

  it("refuses a parent that changes a locked field below it", () => {
    const s = open(claim(), hostLocked);
    expect(
      setValue(s, [...ACCESS, f("parameters")], { hostnamePattern: "x" }),
    ).toEqual({ refused: admin });
    expect(resetValue(s, [...ACCESS, f("parameters")])).toEqual({
      refused: admin,
    });
  });

  it("accepts a parent that keeps a locked field below it", () => {
    const s = ok(
      setValue(open(claim(), hostLocked), [...ACCESS, f("parameters")], {
        hostnamePattern: "{name}.a.example.com",
        tier: "small",
      }),
    );
    expect(s.pending.size).toBe(1);
  });

  it("refuses an index step to an item that the lock names by its keys", () => {
    const s = open(claim(), hostLocked);
    expect(
      setValue(
        s,
        [
          f("spec"),
          f("plugins"),
          { index: 0 },
          f("parameters"),
          f("hostnamePattern"),
        ],
        "x",
      ),
    ).toEqual({ refused: admin });
  });

  it.each<[string, (s: Session) => SetResult<ClaimObject>]>([
    [
      "an index step to another option with the parameter",
      (s) =>
        setValue(s, [f("spec"), f("plugins"), { index: 0 }], {
          section: "access",
          registration: "other-access",
          parameters: { hostnamePattern: "h" },
        }),
    ],
    [
      "an index step to another option without the parameter",
      (s) =>
        setValue(s, [f("spec"), f("plugins"), { index: 0 }], {
          section: "access",
          registration: "other-access",
        }),
    ],
    [
      "a set of spec.plugins that swaps the selection",
      (s) =>
        setValue(
          s,
          [f("spec"), f("plugins")],
          [{ section: "access", registration: "other-access" }],
        ),
    ],
    ["a reset that deselects the plugin", (s) => resetValue(s, ACCESS)],
    [
      "a set of spec that deselects the plugin",
      (s) => setValue(s, [f("spec")], { ...claim().spec, plugins: [] }),
    ],
    [
      "a change of the keys of the item",
      (s) =>
        setValue(
          s,
          [f("spec"), f("plugins"), { index: 0 }, f("section")],
          "other",
        ),
    ],
  ])("accepts %s: the lock goes with its selection", (_, change) => {
    expect(change(open(claim(), hostLocked))).toHaveProperty("session");
  });

  it("accepts an append of a selection that the claim does not hold, with a locked parameter", () => {
    const locks: LockProvider = (p) =>
      p ===
      "spec.plugins[section=backup,registration=s3-backup].parameters.bucket"
        ? admin
        : undefined;
    expect(
      setValue(open(claim(), locks), [f("spec"), f("plugins"), { index: 1 }], {
        section: "backup",
        registration: "s3-backup",
        parameters: { bucket: "b" },
      }),
    ).toHaveProperty("session");
  });

  it("refuses a duplicate append of an item with a locked field that changes it", () => {
    const s = open(claim(), hostLocked);
    expect(
      setValue(s, [f("spec"), f("plugins"), { index: 1 }], {
        section: "access",
        registration: "gateway-access",
        parameters: { hostnamePattern: "h" },
      }),
    ).toEqual({ refused: admin });
  });

  it.each([
    [
      "the keys in another order",
      { registration: "gateway-access", section: "access" },
    ],
    ["a part of the keys", { section: "access" }],
  ])(
    "refuses a keyed step with %s to an item with a locked field",
    (_, keys) => {
      const s = open(claim(), hostLocked);
      expect(
        setValue(
          s,
          [
            f("spec"),
            f("plugins"),
            { item: keys },
            f("parameters"),
            f("hostnamePattern"),
          ],
          "z",
        ),
      ).toEqual({ refused: admin });
    },
  );

  it("refuses a change below a pending parent that changes a locked field", () => {
    const locks: LockProvider = (p) =>
      p === "spec.cluster.zeebe.replicas" ? admin : undefined;
    const s = ok(setValue(open(claim(), locks), cluster("zeebe", "cpu"), 2));
    expect(setValue(s, cluster("zeebe"), { replicas: 9 })).toEqual({
      refused: admin,
    });
  });
});

describe("locks below a field", () => {
  it("refuses a change below a locked field", () => {
    const s = open(claim(), (p) =>
      p === "spec.cluster.serviceAccount.annotations"
        ? {
            reason: "filled",
            section: "cloud-identity",
            sectionLabel: "Cloud identity",
          }
        : undefined,
    );
    const steps = cluster("serviceAccount", "annotations", "team");
    expect(setValue(s, steps, "a")).toEqual({
      refused: {
        reason: "filled",
        section: "cloud-identity",
        sectionLabel: "Cloud identity",
      },
    });
    expect("refused" in resetValue(s, steps)).toBe(true);
  });
});

describe("indexed items", () => {
  const listClaim = () =>
    withSpec({ cluster: { extraEnvFrom: [{ a: 1 }, { b: 2 }, { c: 3 }] } });
  const item = (i: number): Step[] => [
    ...cluster("extraEnvFrom"),
    { index: i },
  ];

  it("resets of two indexed items remove both", () => {
    let s = ok(resetValue(open(listClaim()), item(0)));
    s = ok(resetValue(s, item(1)));
    expect(
      (claimAfter(s).spec as { cluster: { extraEnvFrom: unknown } }).cluster
        .extraEnvFrom,
    ).toEqual([{ c: 3 }]);
  });

  it("a reset before a change of a later item keeps the change on that item", () => {
    let s = ok(resetValue(open(listClaim()), item(0)));
    s = ok(setValue(s, item(2), { c: 4 }));
    expect(
      (claimAfter(s).spec as { cluster: { extraEnvFrom: unknown } }).cluster
        .extraEnvFrom,
    ).toEqual([{ b: 2 }, { c: 4 }]);
  });

  it("a reset of the only item and an append at the next index keep one item", () => {
    const one = withSpec({ cluster: { extraEnvFrom: [{ a: 1 }] } });
    let s = ok(resetValue(open(one), item(0)));
    s = ok(setValue(s, item(1), { d: 4 }));
    expect(
      (claimAfter(s).spec as { cluster: { extraEnvFrom: unknown } }).cluster
        .extraEnvFrom,
    ).toEqual([{ d: 4 }]);
  });

  it("a reset of an appended item leaves no hole in the list", () => {
    const one = withSpec({ cluster: { extraEnvFrom: [{ a: 1 }] } });
    let s = ok(setValue(open(one), item(1), { d: 4 }));
    s = ok(setValue(s, item(2), { e: 5 }));
    s = ok(resetValue(s, item(1)));
    expect(
      (claimAfter(s).spec as { cluster: { extraEnvFrom: unknown } }).cluster
        .extraEnvFrom,
    ).toEqual([{ a: 1 }, { e: 5 }]);
    expect([...s.pending.keys()]).toEqual(["spec.cluster.extraEnvFrom[1]"]);
  });

  it("a discard of an appended item leaves no hole in the list", () => {
    const one = withSpec({ cluster: { extraEnvFrom: [{ a: 1 }] } });
    let s = ok(setValue(open(one), item(1), { d: 4 }));
    s = ok(setValue(s, item(2), { e: 5 }));
    s = discard(s, "spec.cluster.extraEnvFrom[1]");
    expect(
      (claimAfter(s).spec as { cluster: { extraEnvFrom: unknown } }).cluster
        .extraEnvFrom,
    ).toEqual([{ a: 1 }, { e: 5 }]);
  });

  it("a reset or a discard of the last field of an appended item leaves no hole", () => {
    const one = withSpec({ cluster: { extraEnvFrom: [{ a: 1 }] } });
    let s = ok(setValue(open(one), [...item(1), f("configMapRef")], "m"));
    s = ok(setValue(s, [...item(2), f("secretRef")], "x"));
    const list = (session: Session) =>
      (claimAfter(session).spec as { cluster: { extraEnvFrom: unknown } })
        .cluster.extraEnvFrom;
    expect(list(ok(resetValue(s, [...item(1), f("configMapRef")])))).toEqual([
      { a: 1 },
      { secretRef: "x" },
    ]);
    expect(
      list(discard(s, "spec.cluster.extraEnvFrom[1].configMapRef")),
    ).toEqual([{ a: 1 }, { secretRef: "x" }]);
  });

  it("a reset in a nested appended list leaves no hole", () => {
    const term = (i: number): Step[] => [
      ...cluster("affinity", "terms"),
      { index: 0 },
      f("expressions"),
      { index: i },
    ];
    let s = ok(setValue(open(), term(0), { key: "a" }));
    s = ok(setValue(s, term(1), { key: "b" }));
    s = ok(resetValue(s, term(0)));
    expect(
      (claimAfter(s).spec as { cluster: { affinity: unknown } }).cluster
        .affinity,
    ).toEqual({ terms: [{ expressions: [{ key: "b" }] }] });
  });

  it("rebase drops a locked appended item without a hole, and checks the moved items", () => {
    const one = withSpec({ cluster: { extraEnvFrom: [{ a: 1 }] } });
    let s = ok(setValue(open(one), item(1), { d: 4 }));
    s = ok(setValue(s, item(2), { e: 5 }));
    const next = {
      ...one,
      metadata: { ...one.metadata, resourceVersion: "42" },
    };
    const rebased = rebase(s, next, (p) =>
      p === "spec.cluster.extraEnvFrom[1]" ? { reason: "admin" } : undefined,
    );
    expect(
      (claimAfter(rebased).spec as { cluster: { extraEnvFrom: unknown } })
        .cluster.extraEnvFrom,
    ).toEqual([{ a: 1 }]);
    const unlocked = rebase(s, next, (p) =>
      p === "spec.cluster.extraEnvFrom[2]" ? { reason: "admin" } : undefined,
    );
    expect(
      (claimAfter(unlocked).spec as { cluster: { extraEnvFrom: unknown } })
        .cluster.extraEnvFrom,
    ).toEqual([{ a: 1 }, { d: 4 }]);
  });

  it("rebase drops an indexed change when its list changed", () => {
    const s = ok(setValue(open(listClaim()), item(1), { b: 5 }));
    const next = withSpec({ cluster: { extraEnvFrom: [{ b: 2 }, { c: 3 }] } });
    next.metadata = { ...next.metadata, resourceVersion: "42" };
    expect(rebase(s, next).pending.size).toBe(0);
  });
});

describe("order", () => {
  it("a second change of an entry keeps the place of its first change", () => {
    const env = (name: string): Step[] => [
      ...cluster("extraEnv"),
      { item: { name } },
    ];
    let s = ok(setValue(open(), env("A"), { name: "A", value: "1" }));
    s = ok(setValue(s, env("B"), { name: "B", value: "$(A)" }));
    s = ok(setValue(s, env("A"), { name: "A", value: "2" }));
    expect(
      (
        claimAfter(s).spec as { cluster: { extraEnv: { name: string }[] } }
      ).cluster.extraEnv.map((e) => e.name),
    ).toEqual(["LOG", "A", "B"]);
  });
});

describe("discard", () => {
  it("drops one pending change and keeps the others", () => {
    let s = ok(setValue(open(), cluster("zeebe", "replicas"), 5));
    s = ok(setValue(s, [f("spec"), f("presetRef")], "large"));
    const left = discard(s, "spec.cluster.zeebe.replicas");
    expect([...left.pending.keys()]).toEqual(["spec.presetRef"]);
    expect(s.pending.size).toBe(2);
  });
});

describe("changeOf", () => {
  it("changeOf carries the claim after the change and the precondition, and nothing else", () => {
    const s = ok(setValue(open(), cluster("zeebe", "replicas"), 5));
    expect(changeOf(s, "Scale the brokers of orders")).toEqual({
      intent: "Scale the brokers of orders",
      apply: [claimAfter(s)],
      preconditions: { "CamundaClusterClaim/team-a/orders": "41" },
    });
  });
});

describe("rebase", () => {
  it("rebase keeps changes that still apply and drops the ones the new claim holds", () => {
    let s = ok(setValue(open(), cluster("zeebe", "replicas"), 5));
    s = ok(setValue(s, [f("spec"), f("presetRef")], "large"));
    const next = { ...withSpec({ presetRef: "large" }) };
    next.metadata = { ...next.metadata, resourceVersion: "42" };

    const rebased = rebase(s, next);
    expect(rebased.claim).toEqual(next);
    expect([...rebased.pending.keys()]).toEqual([
      "spec.cluster.zeebe.replicas",
    ]);
    expect(changeOf(rebased, "x").preconditions).toEqual({
      "CamundaClusterClaim/team-a/orders": "42",
    });
  });

  it("drops a change that the new lock provider locks", () => {
    const s = ok(setValue(open(), cluster("zeebe", "replicas"), 5));
    const rebased = rebase(s, claim("42"), (p) =>
      p === "spec.cluster.zeebe.replicas" ? { reason: "admin" } : undefined,
    );
    expect(rebased.pending.size).toBe(0);
  });

  it("keeps a reset while the new claim still holds the value", () => {
    const s = ok(resetValue(open(), cluster("zeebe", "replicas")));
    expect(rebase(s, claim("42")).pending.size).toBe(1);
    const gone = withSpec({ cluster: { extraEnv: [] } });
    expect(rebase(s, gone).pending.size).toBe(0);
  });
});
