import { describe, expect, expectTypeOf, it } from "vitest";
import type { Process } from "@/components/cluster/topology";
import { parseTemplate } from "@/lib/gotemplate";
import type { Resource } from "@/lib/k8s/kinds";
import {
  coreMetricDefs,
  coreQueries,
  podsPattern,
  registrationQueries,
} from "./queries";
import type { CoreComponent, QueryError } from "./types";

const registration = {
  metadata: { name: "rustfs-bucket" },
  spec: {
    metricQueries: [
      {
        label: "Requests",
        query:
          'sum(rate(rustfs_requests_total{namespace="{{ .namespace }}",pod=~"{{ .pods }}",bucket="{{ .outputs.bucket }}"}[5m]))',
        unit: "req/s",
      },
      {
        label: "Claim",
        query:
          'x{claim="{{ .claim }}",request="{{ .request }}",sa="{{ .serviceAccount }}",r="{{ .parameters.region }}"}',
      },
      { label: "Bad", query: "{{ if .namespace }}x{{ end }}" },
    ],
  },
} as unknown as Resource<"PluginRegistration">;

const request = {
  metadata: { name: "orders-backup-storage", namespace: "team-a" },
  spec: {
    claimRef: { name: "orders" },
    target: { serviceAccountName: "orders-camunda" },
    parameters: { region: "eu-1" },
  },
  status: { outputs: { bucket: "b-1" } },
} as unknown as Resource<"PluginRequest">;

describe("registrationQueries", () => {
  it("renders the placeholders into PromQL", () =>
    expect(
      registrationQueries(registration, request, ["rustfs-0", "rustfs-1"]).slice(
        0,
        2,
      ),
    ).toEqual([
      {
        label: "Requests",
        unit: "req/s",
        source: "rustfs-bucket",
        query:
          'sum(rate(rustfs_requests_total{namespace="team-a",pod=~"rustfs-0|rustfs-1",bucket="b-1"}[5m]))',
      },
      {
        label: "Claim",
        source: "rustfs-bucket",
        query:
          'x{claim="orders",request="orders-backup-storage",sa="orders-camunda",r="eu-1"}',
      },
    ]));

  it("returns an error that names the if construct", () => {
    const bad = registrationQueries(registration, request, ["p"])[2] as QueryError;
    expect(bad.label).toBe("Bad");
    expect(bad.source).toBe("rustfs-bucket");
    expect(bad.error).toContain("if");
  });

  it("leaves out a .pods query when the request has no pods", () =>
    expect(
      registrationQueries(registration, request, []).map((q) => q.label),
    ).toEqual(["Claim", "Bad"]));

  it("returns an error for an output that the request does not have yet", () => {
    const pending = { ...request, status: {} } as Resource<"PluginRequest">;
    expect(registrationQueries(registration, pending, ["p"])[0]).toEqual({
      label: "Requests",
      source: "rustfs-bucket",
      error: 'output "bucket" has no value',
    });
  });

  it("a registration without metricQueries has no queries", () =>
    expect(
      registrationQueries(
        { ...registration, spec: {} } as never,
        request,
        ["p"],
      ),
    ).toEqual([]));
});

describe("podsPattern", () => {
  it("sorts, joins, and writes a dot as [.]", () =>
    expect(podsPattern(["b-1", "a.x"])).toBe("a[.]x|b-1"));
});

describe("coreQueries", () => {
  const all = Object.values(coreQueries).flat();

  it("every core query names a metric that its query uses, and a source URL", () => {
    for (const q of all) {
      expect(q.query).toContain(q.metric);
      expect(q.source).toMatch(/^https:\/\//);
      expect(parseTemplate(q.query, "metric")).not.toHaveProperty("error");
    }
  });

  it("every core query selects the namespace and the pods", () => {
    for (const q of all) {
      expect(q.query).toContain('namespace="{{ .namespace }}"');
      expect(q.query).toContain('pod=~"{{ .pods }}"');
    }
  });

  it("each cAdvisor query drops the pod-level series", () => {
    const cadvisor = all.filter((q) => q.metric.startsWith("container_"));
    expect(cadvisor.length).toBeGreaterThan(0);
    for (const q of cadvisor) expect(q.query).toContain('container!=""');
  });

  it("CoreComponent is the Process of the topology", () =>
    expectTypeOf<CoreComponent>().toEqualTypeOf<Process>());
});

describe("coreMetricDefs", () => {
  it("renders the namespace and the pods of a component", () =>
    expect(
      coreMetricDefs("zeebe", "team-a", ["orders-zeebe-0"]).find(
        (d) => d.label === "Records processed",
      )?.query,
    ).toBe(
      'sum(rate(zeebe_stream_processor_records_total{namespace="team-a", pod=~"orders-zeebe-0", action="processed"}[5m]))',
    ));

  it("keeps the label, the unit, and the source of the query", () =>
    expect(coreMetricDefs("operate", "team-a", ["o-0"])[0]).toMatchObject({
      label: "CPU",
      unit: "cores",
      source: coreQueries.operate[0].source,
    }));

  it("gives nothing for a component without pods", () =>
    expect(coreMetricDefs("operate", "team-a", [])).toEqual([]));
});
