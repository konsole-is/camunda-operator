// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PlaneField } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { planeCards } from "./plane-cards";
import { PlaneComponentDetail } from "./plane-detail";

const seen = vi.hoisted(() => ({ props: [] as Record<string, unknown>[] }));
vi.mock("@/components/cluster/component-detail", () => ({
  ComponentDetail: (props: Record<string, unknown>) => {
    seen.props.push(props);
    return null;
  },
}));

beforeEach(() => {
  seen.props = [];
});
afterEach(cleanup);

const condition = (type: string, status = "True", reason = "Healthy") => ({
  type,
  status,
  reason,
  message: "",
  lastTransitionTime: "2026-11-15T12:00:00Z",
});

function planeOf(conditions = [condition("IdentityReady")]) {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: { namespace: "platform", name: "plane-a", uid: "u-plane" },
    spec: {
      identityProvider: { oidc: {} },
      identity: {},
      console: {},
    },
    status: { conditions },
  } as unknown as Resource<"CamundaManagementCluster">;
}

const fields: PlaneField[] = [
  {
    key: "admin.claimName",
    group: "identity",
    label: "Administrator claim name",
    value: "email",
    claimPath: "spec.management.identity.admin.claimName",
    claimSteps: [],
    lock: { reason: "immutableAfterStart" },
  },
  {
    key: "versions.console",
    group: "versions",
    label: "Console version",
    value: "8.9.133",
    claimPath: "spec.versions.console",
    claimSteps: [],
  },
];

function detailOf(
  plane = planeOf(),
  id: "identity" | "console" = "identity",
  unread: string[] = [],
) {
  render(
    <PlaneComponentDetail
      id={id}
      cards={planeCards({ plane, pods: [] })}
      plane={plane}
      podSelector="camunda.io/management-cluster=plane-a"
      fields={fields}
      unread={unread}
      now="2026-11-15T12:00:00Z"
    />,
  );
  return seen.props.at(-1)!;
}

describe("PlaneComponentDetail", () => {
  it("the Identity detail shows IdentityReady and watches the Identity pods of the plane", () => {
    const props = detailOf();
    expect(props).toMatchObject({
      title: "Management Identity",
      namespace: "platform",
      podSelector:
        "camunda.io/management-cluster=plane-a,camunda.io/component=management-identity",
      conditions: [{ type: "IdentityReady", status: "True" }],
      ready: { type: "IdentityReady", status: "True" },
    });
    expect(props.metrics).toBeUndefined();
  });

  it("an IdentityReady of False shows in the Identity detail", () => {
    const props = detailOf(
      planeOf([condition("IdentityReady", "False", "Down")]),
    );
    expect(props.ready).toMatchObject({ status: "False", reason: "Down" });
    expect(props.conditions).toEqual([
      expect.objectContaining({ type: "IdentityReady", reason: "Down" }),
    ]);
  });

  it("the Settings tab of Identity shows the admin fields and not those of another card", () => {
    render(detailOf().settings as ReactElement);
    expect(screen.getByText("Administrator claim name")).toBeTruthy();
    expect(screen.getByText("Fixed after Identity started")).toBeTruthy();
    expect(screen.queryByText("Console version")).toBeNull();
  });

  it("the events are those of the CamundaManagementCluster", () => {
    expect(detailOf().eventRefs).toEqual([
      {
        kind: "CamundaManagementCluster",
        name: "plane-a",
        namespace: "platform",
        uid: "u-plane",
      },
    ]);
  });

  it("states no condition while the page cannot read the plane, and names the kind", () => {
    const props = detailOf(planeOf(), "identity", ["CamundaManagementCluster"]);
    expect(props.readyUnknown).toBe(true);
    expect(props.conditions).toEqual([]);
    render(props.children as ReactElement);
    expect(
      screen.getByText(/could not read CamundaManagementCluster/),
    ).toBeTruthy();
  });
});
