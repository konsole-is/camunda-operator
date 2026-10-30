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

  it("rebase drops an indexed change when its list changed", () => {
    const s = ok(setValue(open(listClaim()), item(1), { b: 5 }));
    const next = withSpec({ cluster: { extraEnvFrom: [{ b: 2 }, { c: 3 }] } });
    next.metadata = { ...next.metadata, resourceVersion: "42" };
    expect(rebase(s, next).pending.size).toBe(0);
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
