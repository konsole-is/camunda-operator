import { describe, expect, it } from "vitest";
import {
  condition,
  labelledNamespace,
  pluginGrant,
  pluginRequest,
  registrationSources,
} from "./fixtures";
import { usedByRows } from "./used-by";

const LABELS = {
  "plugins.camunda.io/registration": "aws-s3",
  "plugins.camunda.io/request": "orders-backup-storage",
};

const esRequest = pluginRequest(
  "team-orders",
  "orders-secondary-storage",
  {
    registration: "elasticsearch",
    claimRef: { kind: "CamundaClusterClaim", name: "orders" },
    target: { kind: "CamundaCluster", name: "orders" },
    deletionPolicy: "Orphan",
  },
  {
    conditions: [
      condition("OutputsReady", "True", "Healthy"),
      condition("Ready", "True", "Healthy"),
    ] as never,
    summary: [
      { label: "Health", value: "green", severity: "Info" },
      { label: "Nodes", value: "3" },
    ],
    resources: [
      { kind: "ElasticsearchCluster", name: "orders-es", onOrphan: "Kept" },
      { kind: "SecondaryStorageConfig", name: "orders-es", onOrphan: "Kept" },
    ],
  },
);
const s3Request = pluginRequest(
  "team-orders",
  "orders-backup-storage",
  { registration: "aws-s3" },
  {
    conditions: [
      condition("OutputsReady", "True", "Healthy"),
      condition("Ready", "False", "GrantRejected"),
    ] as never,
  },
);
const refusedGrant = (namespace: string, name: string) =>
  pluginGrant(namespace, name, { template: "bucket-access" }, LABELS, [
    condition("Ready", "False", "OverCeiling"),
  ]);

describe("usedByRows", () => {
  it("gives the claim, the environment, the headline, the resources, and the deletion policy", () => {
    const sources = registrationSources({ requests: [esRequest] });
    expect(usedByRows("elasticsearch", sources, [])).toEqual([
      expect.objectContaining({
        namespace: "team-orders",
        name: "orders-secondary-storage",
        claim: { kind: "CamundaClusterClaim", name: "orders" },
        target: { kind: "CamundaCluster", name: "orders" },
        environment: "Production",
        outputsReady: expect.objectContaining({ status: "True" }),
        ready: expect.objectContaining({ status: "True" }),
        headline: { label: "Health", value: "green", severity: "Info" },
        resources: [
          { kind: "ElasticsearchCluster", name: "orders-es", onOrphan: "Kept" },
          {
            kind: "SecondaryStorageConfig",
            name: "orders-es",
            onOrphan: "Kept",
          },
        ],
        deletionPolicy: "Orphan",
        grantsNotReady: [],
        grants: [],
      }),
    ]);
  });

  it("names the Grants with Ready False of a request whose Ready is GrantRejected", () => {
    const sources = registrationSources({ requests: [s3Request] });
    const [row] = usedByRows("aws-s3", sources, [
      refusedGrant("team-orders", "orders-bucket-access"),
      refusedGrant("team-other", "other-bucket-access"),
    ]);
    expect(row.grantsNotReady).toEqual(["orders-bucket-access"]);
    expect(row.grants?.map((g) => g.name)).toEqual(["orders-bucket-access"]);
  });

  it("names a Grant of an old template of a request whose Ready is GrantRejected", () => {
    const sources = registrationSources({ requests: [s3Request] });
    const [row] = usedByRows("aws-s3", sources, [
      pluginGrant(
        "team-orders",
        "orders-bucket-access",
        { template: "bucket-access" },
        {
          "plugins.camunda.io/registration": "aws-s3",
          "plugins.camunda.io/request": "orders-backup-storage",
        },
        [
          condition("Applied", "True", "Healthy"),
          condition("MatchesTemplate", "False", "TemplateChanged"),
          condition("Ready", "False", "TemplateChanged"),
        ],
      ),
    ]);
    expect(row.grantsNotReady).toEqual(["orders-bucket-access"]);
  });

  it("leaves out the requests of other registrations", () => {
    const sources = registrationSources({
      requests: [
        pluginRequest("team-orders", "x", {
          registration: "existing-object-storage",
        }),
      ],
    });
    expect(usedByRows("elasticsearch", sources, [])).toEqual([]);
  });

  it("sorts the rows by namespace and name", () => {
    const sources = registrationSources({
      requests: [
        pluginRequest("team-b", "a", { registration: "aws-s3" }),
        pluginRequest("team-a", "b", { registration: "aws-s3" }),
        pluginRequest("team-a", "a", { registration: "aws-s3" }),
      ],
    });
    expect(
      usedByRows("aws-s3", sources, []).map((r) => `${r.namespace}/${r.name}`),
    ).toEqual(["team-a/a", "team-a/b", "team-b/a"]);
  });

  it("names a namespace without an environment", () => {
    const sources = registrationSources({
      requests: [pluginRequest("team-free", "a", { registration: "aws-s3" })],
      namespaces: [labelledNamespace("team-free")],
    });
    expect(usedByRows("aws-s3", sources, [])[0].environment).toBe(
      "No environment",
    );
  });

  it("makes no health claim while PluginRequest is not read", () => {
    const sources = registrationSources({ requests: [esRequest] });
    const [row] = usedByRows(
      "elasticsearch",
      { ...sources, stale: ["PluginRequest"] },
      [],
    );
    expect(row.healthUnknown).toBe(true);
    expect(row.ready).toBeUndefined();
    expect(row.outputsReady).toBeUndefined();
  });

  it("knows no environment while Namespace or Environment is not read", () => {
    const sources = registrationSources({ requests: [esRequest] });
    for (const kind of ["Namespace", "Environment"]) {
      const [row] = usedByRows(
        "elasticsearch",
        { ...sources, stale: [kind] },
        [],
      );
      expect(row.environment).toBeUndefined();
    }
  });

  it("gives an error for a request whose claim or target is not valid", () => {
    for (const spec of [{ target: undefined }, { claimRef: "orders" }]) {
      const sources = registrationSources({
        requests: [
          pluginRequest("team-orders", "bad", {
            registration: "aws-s3",
            ...spec,
          } as never),
        ],
      });
      const [row] = usedByRows("aws-s3", sources, []);
      expect(row).toMatchObject({
        name: "bad",
        refError: "The claim or the target of the request is not valid.",
      });
      expect(row.claim).toBeUndefined();
      expect(row.target).toBeUndefined();
    }
  });

  it("knows no Grant while Grant is not read", () => {
    const sources = registrationSources({ requests: [s3Request] });
    const [row] = usedByRows(
      "aws-s3",
      { ...sources, errors: [{ kind: "Grant", message: "forbidden" }] },
      [refusedGrant("team-orders", "orders-bucket-access")],
    );
    expect(row.grants).toBeUndefined();
    expect(row.grantsNotReady).toBeUndefined();
  });

  it("gives an error for a summary, a resource, or conditions that are not valid", () => {
    const sources = registrationSources({
      requests: [
        pluginRequest("team-orders", "bad", { registration: "aws-s3" }, {
          conditions: { Ready: "True" },
          summary: [{ label: "Health" }],
          resources: [
            { kind: "Bucket", name: "b", onOrphan: "Kept" },
            { kind: "Role" },
          ],
        } as never),
      ],
    });
    const [row] = usedByRows("aws-s3", sources, []);
    expect(row.error).toBe("The conditions are not a list.");
    expect(row.headline).toEqual({
      error: "Summary item 1 is not a valid summary item.",
    });
    expect(row.resources).toEqual([
      { kind: "Bucket", name: "b", onOrphan: "Kept" },
      { error: "Resource 2 is not a valid resource." },
    ]);
  });
});
