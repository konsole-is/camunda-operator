import { describe, expect, it } from "vitest";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { planeProgressRows } from "./progress";

const sections = [
  {
    metadata: { name: "management-database" },
    spec: { label: "Databases", slot: "managementDatabases" },
  },
  {
    metadata: { name: "access" },
    spec: { label: "Access", slot: "externalAccess" },
  },
] as PluginSection[];

const database = {
  section: "management-database",
  registration: "existing-database-configs",
};
const access = { section: "access", registration: "gateway" };

const condition = (
  type: string,
  status: "True" | "False" | "Unknown",
  reason = "Test",
  message = "",
) => ({
  type,
  status,
  reason,
  message,
  lastTransitionTime: "2026-11-10T00:00:00Z",
});

const claim = (
  plugins: object[],
  status?: CamundaManagementClusterClaim["status"],
) =>
  ({
    metadata: { namespace: "platform", name: "plane-a", generation: 1 },
    spec: { plugins },
    ...(status && { status }),
  }) as unknown as CamundaManagementClusterClaim;

describe("planeProgressRows", () => {
  it("rows follow the expansion order: environment, databases, access, management cluster", () => {
    expect(
      planeProgressRows(claim([database, access]), sections).map((r) => [
        r.id,
        r.label,
      ]),
    ).toEqual([
      ["environment", "Environment and policy"],
      ["database", "Databases"],
      ["access", "Access"],
      ["managementCluster", "Management cluster"],
    ]);
  });

  it("adds a row for each other selected section after access, in claim order, with its request", () => {
    const withOthers = [
      ...sections,
      { metadata: { name: "dns-zone" }, spec: { label: "DNS zone" } },
      { metadata: { name: "mail" }, spec: { label: "Mail relay" } },
    ] as PluginSection[];
    const rows = planeProgressRows(
      claim(
        [
          { section: "mail", registration: "relay" },
          access,
          { section: "dns-zone", registration: "route53" },
          database,
        ],
        {
          requests: [
            {
              section: "dns-zone",
              name: "plane-a-dns-zone",
              outputsReady: "True",
              ready: "False",
            },
          ],
        } as unknown as CamundaManagementClusterClaim["status"],
      ),
      withOthers,
    );
    expect(rows.map((r) => [r.id, r.label])).toEqual([
      ["environment", "Environment and policy"],
      ["database", "Databases"],
      ["access", "Access"],
      ["section:mail", "Mail relay"],
      ["section:dns-zone", "DNS zone"],
      ["managementCluster", "Management cluster"],
    ]);
    expect(rows[4]).toMatchObject({
      detail: "plane-a-dns-zone",
      checks: [
        { label: "Outputs", condition: { status: "True" } },
        { label: "Ready", condition: { status: "False" } },
      ],
    });
  });

  it("leaves out the access row without an access selection", () => {
    expect(
      planeProgressRows(claim([database]), sections).map((r) => r.id),
    ).toEqual(["environment", "database", "managementCluster"]);
  });

  it("every row is pending without a status", () => {
    const rows = planeProgressRows(claim([database, access]), sections);
    expect(rows.flatMap((r) => r.checks.map((c) => c.condition))).toEqual(
      rows.flatMap((r) => r.checks.map(() => undefined)),
    );
    expect(planeProgressRows(undefined, sections).map((r) => r.id)).toEqual([
      "environment",
      "database",
      "managementCluster",
    ]);
  });

  it("the environment row passes once the claim controller recorded the environment", () => {
    const [row] = planeProgressRows(
      claim([database], { environment: "production" }),
      sections,
    );
    expect(row.detail).toBe("production");
    expect(row.checks).toEqual([
      { label: "Policy", condition: { status: "True" } },
    ]);
  });

  it("the environment row shows the refusal of the claim controller", () => {
    const refused = condition(
      "Ready",
      "False",
      "MissingInput",
      "without an access selection, the claim must set spec.management.identity.externalUrl",
    );
    const [row] = planeProgressRows(
      claim([database], {
        environment: "production",
        conditions: [{ ...refused, observedGeneration: 1 }],
      }),
      sections,
    );
    expect(row.checks[0].condition).toMatchObject({
      status: "False",
      reason: "MissingInput",
    });
  });

  it("the request rows read outputsReady and ready of status.requests", () => {
    const rows = planeProgressRows(
      claim([database, access], {
        requests: [
          {
            section: "management-database",
            registration: "existing-database-configs",
            name: "plane-a-management-database",
            outputsReady: "True",
            ready: "False",
          },
        ],
      }),
      sections,
    );
    expect(rows[1]).toEqual({
      id: "database",
      label: "Databases",
      detail: "plane-a-management-database",
      checks: [
        { label: "Outputs", condition: { status: "True" } },
        { label: "Ready", condition: { status: "False" } },
      ],
    });
    expect(rows[2].checks.map((c) => c.condition)).toEqual([
      undefined,
      undefined,
    ]);
  });

  it("the management cluster row reads ManagementClusterReady", () => {
    const rows = planeProgressRows(
      claim([database], {
        conditions: [condition("ManagementClusterReady", "False", "Creating")],
      }),
      sections,
    );
    expect(rows.at(-1)?.checks).toEqual([
      {
        label: "Ready",
        condition: { status: "False", reason: "Creating", message: "" },
      },
    ]);
  });
});
