// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { ClaimRef } from "./impact";
import { ImpactLine, impactLines } from "./impact-line";

afterEach(cleanup);

const orders: ClaimRef = {
  kind: "CamundaClusterClaim",
  namespace: "team-a",
  name: "orders",
};

describe("impactLines", () => {
  it("counts the claims that keep their recorded policy", () => {
    expect(impactLines({ clusters: 2, planes: 1, warnings: [] })).toEqual([
      "Changes apply to new claims only. 2 clusters and 1 management plane keep the policy they recorded at creation.",
    ]);
    expect(impactLines({ clusters: 1, planes: 0, warnings: [] })).toEqual([
      "Changes apply to new claims only. 1 cluster keeps the policy it recorded at creation.",
    ]);
    expect(impactLines({ clusters: 0, planes: 2, warnings: [] })).toEqual([
      "Changes apply to new claims only. 2 management planes keep the policy they recorded at creation.",
    ]);
  });

  it("says only that changes apply to new claims when no claim records another policy", () => {
    expect(impactLines({ clusters: 0, planes: 0, warnings: [] })).toEqual([
      "Changes apply to new claims only.",
    ]);
  });

  it("names the claims whose release the edit removes and says that they keep running", () => {
    const [, line] = impactLines({
      clusters: 0,
      planes: 0,
      warnings: [
        { claim: orders, kind: "CamundaRelease", name: "8.9.1", removed: true },
      ],
    });
    expect(line).toBe(
      "orders in team-a uses release 8.9.1, which this change no longer allows. The cluster keeps running. A later change of its release must pick an allowed one.",
    );
    expect(line).not.toContain("refused");
  });

  it("names the Elasticsearch preset in the warning", () => {
    const [, line] = impactLines({
      clusters: 0,
      planes: 0,
      warnings: [
        {
          claim: orders,
          kind: "ElasticsearchClusterPreset",
          name: "es-small",
          removed: true,
        },
      ],
    });
    expect(line).toBe(
      "orders in team-a uses Elasticsearch preset es-small, which this change no longer allows. The cluster keeps running. A later change of its preset must pick an allowed one.",
    );
  });

  it("says that a management plane keeps running when its PostgreSQL preset goes", () => {
    const [, line] = impactLines({
      clusters: 0,
      planes: 0,
      warnings: [
        {
          claim: {
            kind: "CamundaManagementClusterClaim",
            namespace: "platform",
            name: "plane-a",
          },
          kind: "DatabaseServerPreset",
          name: "pg-small",
          removed: true,
        },
      ],
    });
    expect(line).toBe(
      "plane-a in platform uses PostgreSQL preset pg-small, which this change no longer allows. The management plane keeps running. A later change of its preset must pick an allowed one.",
    );
  });

  it("names the claims whose cluster preset the edit removes", () => {
    expect(
      impactLines({
        clusters: 0,
        planes: 0,
        warnings: [
          {
            claim: orders,
            kind: "CamundaClusterPreset",
            name: "small",
            removed: true,
          },
        ],
      })[1],
    ).toBe(
      "orders in team-a uses cluster preset small, which this change no longer allows. The cluster keeps running. A later change of its preset must pick an allowed one.",
    );
  });

  it("names the claims that follow a new default", () => {
    expect(
      impactLines({
        clusters: 0,
        planes: 0,
        warnings: [
          {
            claim: orders,
            kind: "CamundaRelease",
            name: "8.9.2",
            followsDefault: true,
          },
          {
            claim: orders,
            kind: "CamundaPlatformConfig",
            name: "shared",
            followsDefault: true,
          },
        ],
      }).slice(1),
    ).toEqual([
      "orders in team-a names no release and moves to 8.9.2. Its brokers restart one at a time.",
      "orders in team-a names no platform config and moves to shared.",
    ]);
  });
});

describe("impactLines of a cleared default", () => {
  it("names the claims that lose their default", () => {
    expect(
      impactLines({
        clusters: 0,
        planes: 0,
        warnings: [
          { claim: orders, kind: "CamundaRelease", cleared: true },
          { claim: orders, kind: "CamundaPlatformConfig", cleared: true },
          {
            claim: {
              kind: "CamundaManagementClusterClaim",
              namespace: "platform",
              name: "plane-a",
            },
            kind: "CamundaPlatformConfig",
            cleared: true,
          },
        ],
      }).slice(1),
    ).toEqual([
      "orders in team-a names no release, and this change removes the default release. The cluster keeps running, but the claim is not ready and its cluster takes no further change.",
      "orders in team-a names no platform config, and this change removes the default platform config. The cluster keeps running, but the claim is not ready and its cluster takes no further change.",
      "plane-a in platform names no platform config, and this change removes the default platform config. Its management plane keeps running, but the claim reports MissingInput and takes no further change.",
    ]);
  });
});

describe("impactLines of a default release that the cluster refuses", () => {
  it.each([
    ["VersionDowngradeRefused" as const],
    ["VersionSkipRefused" as const],
  ])("says that the cluster refuses the move (%s)", (refused) => {
    expect(
      impactLines({
        clusters: 0,
        planes: 0,
        warnings: [
          {
            claim: orders,
            kind: "CamundaRelease",
            name: "8.7.0",
            followsDefault: true,
            refused,
          },
        ],
      })[1],
    ).toBe(
      `orders in team-a names no release, and its cluster refuses the move to 8.7.0 (${refused}). The cluster keeps running and takes no further change.`,
    );
  });
});

describe("impactLines of a first environment", () => {
  it("names the namespaces whose claims take no change after it", () => {
    expect(
      impactLines({
        clusters: 0,
        planes: 0,
        warnings: [],
        unassigned: ["legacy", "team-x"],
      }),
    ).toEqual([
      "Changes apply to new claims only.",
      "This is the first environment. The claims in legacy, team-x have no environment label, so they take no change until their namespaces join an environment (NoEnvironment). Their clusters keep running.",
    ]);
  });
});

describe("ImpactLine", () => {
  it("shows the count and each warning", () => {
    render(
      <ImpactLine
        impact={{
          clusters: 2,
          planes: 0,
          warnings: [
            {
              claim: orders,
              kind: "CamundaRelease",
              name: "8.9.1",
              removed: true,
            },
          ],
        }}
      />,
    );
    const note = screen.getByRole("note", { name: "Impact" });
    expect(note.textContent).toContain(
      "2 clusters keep the policy they recorded at creation.",
    );
    expect(note.textContent).toContain("The cluster keeps running.");
  });
});
