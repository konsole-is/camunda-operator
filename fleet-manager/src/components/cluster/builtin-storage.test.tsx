// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { BuiltinStorage } from "./builtin-storage";
import { fixtureCondition, requestNow } from "./request-fixtures";

afterEach(cleanup);

function elasticsearch(status?: object): Resource<"ElasticsearchCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "ElasticsearchCluster",
    metadata: { namespace: "team-a", name: "orders-es" },
    spec: {},
    ...(status && { status }),
  } as unknown as Resource<"ElasticsearchCluster">;
}

function databaseServer(status?: object): Resource<"DatabaseServer"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "DatabaseServer",
    metadata: { namespace: "team-a", name: "orders-db" },
    spec: {},
    ...(status && { status }),
  } as unknown as Resource<"DatabaseServer">;
}

const term = (box: HTMLElement, label: string) =>
  within(box).getByText(label, { selector: "dt" }).nextElementSibling
    ?.textContent;

describe("BuiltinStorage", () => {
  it("shows the version, volumes, snapshot repository, and conditions of an ElasticsearchCluster", () => {
    render(
      <BuiltinStorage
        storage={{
          kind: "ElasticsearchCluster",
          name: "orders-es",
          object: elasticsearch({
            version: "8.19.2",
            snapshotRepository: "camunda",
            volumes: [{ name: "data-orders-es-0", capacity: "32Gi" }],
            conditions: [fixtureCondition("Ready", "True", "Healthy")],
          }),
        }}
        unread={[]}
        now={requestNow}
      />,
    );
    const box = screen.getByRole("region", { name: "ElasticsearchCluster" });
    expect(within(box).getByText("orders-es")).toBeTruthy();
    expect(term(box, "Version")).toBe("8.19.2");
    expect(term(box, "Snapshot repository")).toBe("camunda");
    expect(
      within(box).getByRole("row", { name: "data-orders-es-0 32Gi" }),
    ).toBeTruthy();
    expect(
      within(box).getByRole("list", { name: "orders-es conditions" })
        .textContent,
    ).toContain("Healthy");
  });

  it("shows the version, archive, recovery, volumes, and conditions of a DatabaseServer", () => {
    render(
      <BuiltinStorage
        storage={{
          kind: "DatabaseServer",
          name: "orders-db",
          object: databaseServer({
            version: "17.5",
            archive: {
              reachableFrom: "2026-11-10T09:58:00Z",
              boundary: {
                at: "2026-11-15T08:00:00Z",
                location: "s3://orders/db",
              },
            },
            recovery: {
              requestID: "3f2b1c4d",
              targetTime: "2026-11-15T07:30:00Z",
              startedAt: "2026-11-15T07:31:00Z",
              contract: "orders-db",
              requestedBy: "orders",
            },
            volumes: [{ name: "orders-db-1", capacity: "10Gi" }],
            conditions: [fixtureCondition("Ready", "False", "ArchiveFailing")],
          }),
        }}
        unread={[]}
        now={requestNow}
      />,
    );
    const box = screen.getByRole("region", { name: "DatabaseServer" });
    expect(term(box, "Version")).toBe("17.5");
    expect(term(box, "Archive from")).toBe("2026-11-10T09:58:00Z");
    expect(term(box, "Archive boundary")).toBe(
      "2026-11-15T08:00:00Z, s3://orders/db",
    );
    expect(term(box, "Recovery request")).toBe("3f2b1c4d");
    expect(term(box, "Recovery target")).toBe("2026-11-15T07:30:00Z");
    expect(term(box, "Recovery started")).toBe("2026-11-15T07:31:00Z");
    expect(
      within(box).getByRole("row", { name: "orders-db-1 10Gi" }),
    ).toBeTruthy();
    expect(within(box).getByText("ArchiveFailing")).toBeTruthy();
    expect(within(box).queryByText("Snapshot repository")).toBeNull();
  });

  it("says Not reported yet for a field that the status does not hold", () => {
    render(
      <BuiltinStorage
        storage={{
          kind: "DatabaseServer",
          name: "orders-db",
          object: databaseServer(),
        }}
        unread={[]}
        now={requestNow}
      />,
    );
    const box = screen.getByRole("region", { name: "DatabaseServer" });
    expect(term(box, "Version")).toBe("Not reported yet");
    expect(term(box, "Archive from")).toBe("Not reported yet");
    expect(term(box, "Recovery request")).toBe("Not reported yet");
    expect(within(box).getByText("No volumes reported yet")).toBeTruthy();
  });

  it("shows no health, node, disk, or connection number", () => {
    render(
      <BuiltinStorage
        storage={{
          kind: "ElasticsearchCluster",
          name: "orders-es",
          object: elasticsearch({ version: "8.19.2" }),
        }}
        unread={[]}
        now={requestNow}
      />,
    );
    const text = screen.getByRole("region", {
      name: "ElasticsearchCluster",
    }).textContent;
    expect(text).not.toMatch(/health|nodes|disk|connections|instances/i);
  });

  it("names a value of the wrong type and keeps the box", () => {
    render(
      <BuiltinStorage
        storage={{
          kind: "ElasticsearchCluster",
          name: "orders-es",
          object: elasticsearch({ version: 8, volumes: "data-0" }),
        }}
        unread={[]}
        now={requestNow}
      />,
    );
    const box = screen.getByRole("region", { name: "ElasticsearchCluster" });
    expect(within(box).getByText("The version is not a text.")).toBeTruthy();
    expect(within(box).getByText("The volumes are not a list.")).toBeTruthy();
  });

  it("marks the conditions Not known and names the kind while it is not read", () => {
    render(
      <BuiltinStorage
        storage={{
          kind: "ElasticsearchCluster",
          name: "orders-es",
          object: elasticsearch({
            conditions: [fixtureCondition("Ready", "True", "Healthy")],
          }),
        }}
        unread={["ElasticsearchCluster"]}
        now={requestNow}
      />,
    );
    const box = screen.getByRole("region", { name: "ElasticsearchCluster" });
    expect(
      within(box).getByText(
        "Not known: the page has no current list of ElasticsearchCluster.",
      ),
    ).toBeTruthy();
    expect(within(box).getByText("Ready: Not known")).toBeTruthy();
    expect(within(box).queryByText("Healthy")).toBeNull();
  });

  it("names the kind of an object that is not read yet, and shows nothing for one that is not in the namespace", () => {
    const storage = {
      kind: "ElasticsearchCluster" as const,
      name: "orders-es",
    };
    const { rerender } = render(
      <BuiltinStorage
        storage={storage}
        unread={["ElasticsearchCluster"]}
        now={requestNow}
      />,
    );
    expect(
      screen.getByText(
        "Not known: the page has no current list of ElasticsearchCluster.",
      ),
    ).toBeTruthy();
    rerender(<BuiltinStorage storage={storage} unread={[]} now={requestNow} />);
    expect(
      screen.queryByRole("region", { name: "ElasticsearchCluster" }),
    ).toBeNull();
  });
});
