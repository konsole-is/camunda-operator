import { describe, expect, it } from "vitest";
import {
  condition,
  fleetEnvironment,
  labelledNamespace,
  pluginRegistration,
  pluginRequest,
  registrationSources,
} from "./fixtures";
import {
  registrationGroups,
  registrationRow,
  registrationSummary,
  sectionFields,
  unreadKinds,
} from "./groups";

describe("registrationGroups", () => {
  it("gives every section a group in the order of the sections", () => {
    const groups = registrationGroups(registrationSources());
    expect(groups.map((g) => g.section?.name)).toEqual([
      "cloud-identity",
      "encryption-key",
      "volume-encryption",
      "backup-storage",
      "secondary-storage",
      "access",
      "connectors-permissions",
      "management-database",
    ]);
    expect(groups[0].rows).toEqual([]);
  });

  it("orders sections of the same order by name", () => {
    const sources = registrationSources();
    const [first] = sources.sections;
    const groups = registrationGroups({
      ...sources,
      sections: [
        ...sources.sections,
        { ...first, metadata: { name: "a-identity" } },
      ],
    });
    expect(groups.slice(0, 2).map((g) => g.section?.name)).toEqual([
      "a-identity",
      "cloud-identity",
    ]);
  });

  it("shows the selection, the empty option, and the field each claim kind fills", () => {
    const groups = registrationGroups(registrationSources());
    const backup = groups.find((g) => g.section?.name === "backup-storage");
    expect(backup?.section).toMatchObject({
      label: "Backup storage",
      selection: "One",
      emptyOptionLabel: "No backups",
    });
    expect(backup?.section?.fields.map((f) => `${f.kind} ${f.path}`)).toEqual([
      "CamundaCluster spec.backupStorageRef",
    ]);
    const access = groups.find((g) => g.section?.name === "access");
    expect(access?.section?.fields.map((f) => f.claimKind)).toEqual(
      expect.arrayContaining([
        "CamundaClusterClaim",
        "CamundaManagementClusterClaim",
      ]),
    );
  });

  it("lists a registration in each of its sections", () => {
    const both = pluginRegistration("two-sections", {
      label: "Two sections",
      sections: ["cloud-identity", "connectors-permissions"],
    });
    const sources = registrationSources();
    const groups = registrationGroups({
      ...sources,
      registrations: [...sources.registrations, both],
    });
    for (const name of ["cloud-identity", "connectors-permissions"]) {
      expect(
        groups.find((g) => g.section?.name === name)?.rows.map((r) => r.name),
      ).toContain("two-sections");
    }
  });

  it("sorts the rows of a group by label, then by name", () => {
    const sources = registrationSources();
    const groups = registrationGroups({
      ...sources,
      registrations: [
        pluginRegistration("b", { label: "Same", sections: ["access"] }),
        pluginRegistration("z", { label: "Alpha", sections: ["access"] }),
        pluginRegistration("a", { label: "Same", sections: ["access"] }),
      ],
    });
    expect(
      groups.find((g) => g.section?.name === "access")?.rows.map((r) => r.name),
    ).toEqual(["z", "a", "b"]);
  });

  it("keeps a registration whose section does not exist in a last group", () => {
    const lost = pluginRegistration(
      "lost",
      { label: "Lost", sections: ["gone"] },
      [condition("Accepted", "False", "SectionMissing")],
    );
    const sources = registrationSources();
    const groups = registrationGroups({
      ...sources,
      registrations: [...sources.registrations, lost],
    });
    const last = groups.at(-1);
    expect(last?.section).toBeUndefined();
    expect(last?.rows.map((r) => r.name)).toEqual(["lost"]);
    expect(last?.rows[0].accepted?.reason).toBe("SectionMissing");
  });

  it("has no group without a section when every registration has one", () => {
    expect(
      registrationGroups(registrationSources()).every((g) => g.section),
    ).toBe(true);
  });

  it("marks every section group unread while PluginRegistration is not read", () => {
    const groups = registrationGroups({
      ...registrationSources(),
      registrations: [],
      stale: ["PluginRegistration"],
    });
    expect(groups).toHaveLength(8);
    expect(groups.every((g) => g.section && g.unread)).toBe(true);
  });

  it("does not call a registration lost while PluginSection is not read", () => {
    const sources = registrationSources();
    for (const reads of [
      { stale: ["PluginSection"] },
      { errors: [{ kind: "PluginSection", message: "forbidden" }] },
    ]) {
      const groups = registrationGroups({ ...sources, sections: [], ...reads });
      expect(groups).toHaveLength(1);
      expect(groups[0].section).toBeUndefined();
      expect(groups[0].unread).toBe(true);
      expect(groups[0].rows).toHaveLength(sources.registrations.length);
    }
  });

  it("marks no group unread while every list is read", () => {
    expect(
      registrationGroups(registrationSources()).some((g) => g.unread),
    ).toBe(false);
  });
});

describe("registrationRow", () => {
  it("shows the built-in controller, its version, and Orphan support", () => {
    const sources = registrationSources();
    const row = registrationRow(
      sources.registrations.find((r) => r.metadata.name === "elasticsearch")!,
      sources,
    );
    expect(row.controller).toEqual({
      name: "camunda-operator",
      version: "0.0.0-dev",
    });
    expect(row.orphan).toBe(true);
  });

  it("has no Orphan support when the deletion policies leave it out", () => {
    const row = registrationRow(
      pluginRegistration("x", { deletionPolicies: ["Delete"] }),
      registrationSources(),
    );
    expect(row.orphan).toBe(false);
  });

  it("gives Healthy since from the transition of a true Ready only", () => {
    const sources = registrationSources();
    const healthy = pluginRegistration("ok", { sections: ["access"] }, [
      condition("Accepted", "True", "Healthy"),
      condition("Ready", "True", "Healthy"),
    ]);
    const failing = pluginRegistration("bad", { sections: ["access"] }, [
      condition("Accepted", "True", "Healthy"),
      condition("Ready", "False", "CredentialsInvalid"),
    ]);
    expect(registrationRow(healthy, sources).healthySince).toBe(
      "2026-11-10T09:00:00Z",
    );
    expect(registrationRow(failing, sources).healthySince).toBeUndefined();
  });

  it("has no condition and no Healthy since without a status", () => {
    const row = registrationRow(
      pluginRegistration("new", { sections: ["access"] }),
      registrationSources(),
    );
    expect(row.accepted).toBeUndefined();
    expect(row.ready).toBeUndefined();
    expect(row.healthySince).toBeUndefined();
  });

  it("counts the requests of the registration by environment", () => {
    const sources = registrationSources({
      namespaces: [
        labelledNamespace("team-orders", "production"),
        labelledNamespace("lab"),
      ],
      requests: [
        pluginRequest("team-orders", "orders-secondary-storage", {
          registration: "elasticsearch",
        }),
        pluginRequest("team-orders", "billing-secondary-storage", {
          registration: "elasticsearch",
        }),
        pluginRequest("lab", "try-secondary-storage", {
          registration: "elasticsearch",
        }),
        pluginRequest("lab", "try-backup-storage", {
          registration: "existing-object-storage",
        }),
      ],
    });
    const row = registrationRow(
      sources.registrations.find((r) => r.metadata.name === "elasticsearch")!,
      sources,
    );
    expect(row.requests).toEqual({
      total: 3,
      byEnvironment: [
        { environment: "No environment", count: 1 },
        { environment: "Production", count: 2 },
      ],
    });
  });

  it("counts a request in a namespace that is not listed under No environment", () => {
    const sources = registrationSources({
      namespaces: [],
      requests: [pluginRequest("gone", "x", { registration: "elasticsearch" })],
    });
    const row = registrationRow(
      sources.registrations.find((r) => r.metadata.name === "elasticsearch")!,
      sources,
    );
    expect(row.requests?.byEnvironment).toEqual([
      { environment: "No environment", count: 1 },
    ]);
  });

  it("counts two Environments with one display name apart, by their names", () => {
    const sources = registrationSources({
      environments: [
        fleetEnvironment("prod-eu", { displayName: "Production" }),
        fleetEnvironment("prod-us", { displayName: "Production" }),
        fleetEnvironment("unset", { displayName: "No environment" }),
      ],
      namespaces: [
        labelledNamespace("orders-eu", "prod-eu"),
        labelledNamespace("orders-us", "prod-us"),
        labelledNamespace("billing", "unset"),
      ],
      requests: [
        pluginRequest("orders-eu", "x", { registration: "elasticsearch" }),
        pluginRequest("orders-us", "x", { registration: "elasticsearch" }),
        pluginRequest("orders-us", "y", { registration: "elasticsearch" }),
        pluginRequest("billing", "x", { registration: "elasticsearch" }),
        pluginRequest("gone", "x", { registration: "elasticsearch" }),
      ],
    });
    const row = registrationRow(
      sources.registrations.find((r) => r.metadata.name === "elasticsearch")!,
      sources,
    );
    expect(row.requests?.byEnvironment).toEqual([
      { environment: "No environment", count: 1 },
      { environment: "No environment (unset)", count: 1 },
      { environment: "Production (prod-eu)", count: 1 },
      { environment: "Production (prod-us)", count: 2 },
    ]);
  });

  it("names an Environment without a display name by its name", () => {
    const sources = registrationSources({
      environments: [],
      namespaces: [labelledNamespace("team-orders", "production")],
      requests: [
        pluginRequest("team-orders", "x", { registration: "elasticsearch" }),
      ],
    });
    sources.environments = [
      { ...registrationSources().environments[0], spec: {} },
    ];
    const row = registrationRow(
      sources.registrations.find((r) => r.metadata.name === "elasticsearch")!,
      sources,
    );
    expect(row.requests?.byEnvironment).toEqual([
      { environment: "production", count: 1 },
    ]);
  });
});

describe("registrationRow on lists that are not read", () => {
  const existing = (sources: ReturnType<typeof registrationSources>) =>
    sources.registrations.find(
      (r) => r.metadata.name === "existing-secondary-storage",
    )!;

  it("gives no request count while PluginRequest is not read", () => {
    const sources = registrationSources();
    for (const reads of [
      { stale: ["PluginRequest"] },
      { errors: [{ kind: "PluginRequest", message: "forbidden" }] },
    ]) {
      const read = { ...sources, ...reads };
      expect(registrationRow(existing(read), read).requests).toBeUndefined();
    }
  });

  it("makes no health claim while PluginRegistration is not read", () => {
    const sources = registrationSources();
    for (const reads of [
      { stale: ["PluginRegistration"] },
      { errors: [{ kind: "PluginRegistration", message: "watch closed" }] },
    ]) {
      const read = { ...sources, ...reads };
      const row = registrationRow(existing(read), read);
      expect(row.healthUnknown).toBe(true);
      expect(row.accepted).toBeUndefined();
      expect(row.ready).toBeUndefined();
      expect(row.healthySince).toBeUndefined();
    }
  });

  it.each(["Namespace", "Environment"])(
    "gives the total but no count by environment while %s is not read",
    (kind) => {
      const sources = { ...registrationSources(), stale: [kind] };
      expect(registrationRow(existing(sources), sources).requests).toEqual({
        total: 1,
      });
    },
  );
});

describe("unreadKinds", () => {
  it("names each failed or stale kind once", () => {
    expect(
      unreadKinds({
        errors: [{ kind: "PluginRequest", message: "forbidden" }],
        stale: ["PluginSection", "PluginRequest"],
      }),
    ).toEqual(["PluginRequest", "PluginSection"]);
    expect(unreadKinds({})).toEqual([]);
  });
});

describe("registrationSummary", () => {
  it("counts a registration once, healthy only when Accepted and Ready are True", () => {
    const sources = registrationSources();
    expect(
      registrationSummary([
        ...sources.registrations,
        pluginRegistration("two", { sections: ["access", "cloud-identity"] }),
        pluginRegistration("refused", { sections: ["access"] }, [
          condition("Accepted", "False", "DefaultConflict"),
          condition("Ready", "True", "Healthy"),
        ]),
      ]),
    ).toEqual({ healthy: 6, notHealthy: 2 });
  });
});

describe("sectionFields", () => {
  it("is empty for a section without a slot", () => {
    const section = registrationSources().sections.find(
      (s) => s.metadata.name === "encryption-key",
    )!;
    expect(sectionFields(section)).toEqual([]);
  });

  it("leaves out the fields of a claim kind the section does not apply to", () => {
    const access = registrationSources().sections.find(
      (s) => s.metadata.name === "access",
    )!;
    const clusterOnly = {
      ...access,
      spec: { ...access.spec, appliesTo: ["CamundaClusterClaim" as const] },
    };
    expect(
      sectionFields(clusterOnly).every(
        (f) => f.claimKind === "CamundaClusterClaim",
      ),
    ).toBe(true);
    expect(sectionFields(clusterOnly).length).toBeGreaterThan(0);
  });
});
