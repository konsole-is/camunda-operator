// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SectionComponent } from "@/lib/claims/sections";
import type { Resource } from "@/lib/k8s/kinds";
import { SectionComponentDetail } from "./section-detail";

const detail = vi.hoisted(() => ({
  seen: {} as Record<string, unknown>,
}));
vi.mock("./component-detail", () => ({
  ComponentDetail: ({
    children,
    ...props
  }: Record<string, unknown> & { children?: ReactNode }) => {
    detail.seen = props;
    return <div>{children}</div>;
  },
}));

beforeEach(() => {
  detail.seen = {};
});
afterEach(cleanup);

const now = "2026-11-15T12:00:00Z";

const ready = {
  type: "Ready",
  status: "False" as const,
  reason: "GrantRejected",
  message: "Grant orders-backup-storage-read has Ready=False",
  lastTransitionTime: "2026-11-15T11:58:00Z",
};
const outputsReady = {
  type: "OutputsReady",
  status: "True" as const,
  reason: "Healthy",
  message: "The plugin wrote the outputs",
  lastTransitionTime: "2026-11-15T11:50:00Z",
};

function request(
  status: NonNullable<Resource<"PluginRequest">["status"]>,
): Resource<"PluginRequest"> {
  return {
    kind: "PluginRequest",
    metadata: {
      name: "orders-backup-storage",
      namespace: "team-a",
      uid: "u-1",
    },
    spec: { registration: "aws-s3", section: "backup-storage" },
    status,
  } as Resource<"PluginRequest">;
}

const backupStorage = (
  over: Partial<SectionComponent> = {},
): SectionComponent => ({
  id: "section:backup-storage",
  section: "backup-storage",
  label: "Backup storage",
  order: 2,
  selection: { section: "backup-storage", registration: "aws-s3" },
  ...over,
});

const withRequest = backupStorage({
  request: request({
    conditions: [outputsReady, ready],
    podSelector: {
      matchLabels: { app: "aws-s3", request: "orders-backup-storage" },
    },
  }),
  condition: ready,
});

const s3 = {
  kind: "PluginRegistration",
  metadata: { name: "aws-s3" },
  spec: { label: "Amazon S3" },
} as unknown as Resource<"PluginRegistration">;

describe("SectionComponentDetail", () => {
  it("opens the request of the section with the props of its observe tabs", () => {
    render(
      <SectionComponentDetail
        component={withRequest}
        registrations={[s3]}
        namespace="team-a"
        now={now}
      />,
    );
    expect(detail.seen).toMatchObject({
      title: "Backup storage",
      subtitle: "Amazon S3",
      namespace: "team-a",
      podSelector: "app=aws-s3,request=orders-backup-storage",
      metrics: { request: "orders-backup-storage" },
      controllerLogs: {
        registration: "aws-s3",
        request: { namespace: "team-a", name: "orders-backup-storage" },
      },
      eventRefs: [
        {
          kind: "PluginRequest",
          name: "orders-backup-storage",
          namespace: "team-a",
          uid: "u-1",
        },
      ],
      conditions: [outputsReady, ready],
      ready,
    });
  });

  it("names the registration of the selection when no registration gives a label", () => {
    render(
      <SectionComponentDetail
        component={withRequest}
        registrations={[]}
        namespace="team-a"
        now={now}
      />,
    );
    expect(detail.seen.subtitle).toBe("aws-s3");
  });

  it("gives no pod selector for a request without one", () => {
    render(
      <SectionComponentDetail
        component={backupStorage({ request: request({}) })}
        registrations={[]}
        namespace="team-a"
        now={now}
      />,
    );
    expect(detail.seen.podSelector).toBe("");
  });

  it("says that the request is not made yet, and observes nothing", () => {
    render(
      <SectionComponentDetail
        component={backupStorage()}
        registrations={[]}
        namespace="team-a"
        now={now}
      />,
    );
    expect(
      screen.getByText(
        "The claim controller has not made the request of this section yet.",
      ),
    ).toBeTruthy();
    expect(detail.seen).toMatchObject({ eventRefs: [], conditions: [] });
    expect(detail.seen.metrics).toBeUndefined();
    expect(detail.seen.controllerLogs).toBeUndefined();
    expect(detail.seen.podSelector).toBeFalsy();
  });

  it("says that the request could not be read when the read of the requests failed", () => {
    render(
      <SectionComponentDetail
        component={backupStorage()}
        registrations={[]}
        namespace="team-a"
        requestsError="watch broke"
        now={now}
      />,
    );
    expect(
      screen.getByText("Could not read PluginRequest: watch broke"),
    ).toBeTruthy();
    expect(
      screen.queryByText(
        "The claim controller has not made the request of this section yet.",
      ),
    ).toBeNull();
    expect(detail.seen.podSelector).toBeFalsy();
  });

  it("opens a request that the page holds although the read of the requests failed", () => {
    render(
      <SectionComponentDetail
        component={withRequest}
        registrations={[]}
        namespace="team-a"
        requestsError="watch broke"
        now={now}
      />,
    );
    expect(detail.seen.podSelector).toBe(
      "app=aws-s3,request=orders-backup-storage",
    );
  });

  it("shows no pods and says so when the pod selector of the request is not valid", () => {
    render(
      <SectionComponentDetail
        component={backupStorage({
          request: request({
            podSelector: {
              matchExpressions: [
                { key: "app", operator: "in", values: ["echo"] },
              ],
            },
          }),
        })}
        registrations={[]}
        namespace="team-a"
        now={now}
      />,
    );
    expect(detail.seen.podSelector).toBeFalsy();
    expect(
      screen.getByText(/^The pod selector of the request is not valid/),
    ).toBeTruthy();
    expect(detail.seen.metrics).toEqual({ request: "orders-backup-storage" });
  });

  it("shows the state of a request as not known, and names PluginRequest in place of children, while the requests are out of date", () => {
    render(
      <SectionComponentDetail
        component={withRequest}
        registrations={[s3]}
        namespace="team-a"
        requestsStale
        now={now}
      >
        <p>Request outputs</p>
      </SectionComponentDetail>,
    );
    expect(detail.seen.unread).toBe("PluginRequest");
    expect(
      screen.getByText("Not known: the page could not read PluginRequest."),
    ).toBeTruthy();
    expect(screen.queryByText("Request outputs")).toBeNull();
  });

  it("says the error of the read in place of children when the read of the requests failed", () => {
    render(
      <SectionComponentDetail
        component={withRequest}
        registrations={[s3]}
        namespace="team-a"
        requestsError="watch broke"
        now={now}
      >
        <p>Request outputs</p>
      </SectionComponentDetail>,
    );
    expect(detail.seen.unread).toBe("PluginRequest");
    expect(
      screen.getByText("Could not read PluginRequest: watch broke"),
    ).toBeTruthy();
    expect(screen.queryByText("Request outputs")).toBeNull();
  });

  it("names PluginRequest, never that the request is not made yet, while the requests are out of date", () => {
    render(
      <SectionComponentDetail
        component={backupStorage()}
        registrations={[]}
        namespace="team-a"
        requestsStale
        now={now}
      />,
    );
    expect(
      screen.getByText("Not known: the page could not read PluginRequest."),
    ).toBeTruthy();
    expect(screen.queryByText(/has not made the request/)).toBeNull();
  });

  it("names conditions of the wrong type and shows none of them", () => {
    render(
      <SectionComponentDetail
        component={backupStorage({
          request: request({ conditions: "Ready" } as never),
        })}
        registrations={[]}
        namespace="team-a"
        now={now}
      />,
    );
    expect(detail.seen.conditions).toEqual([]);
    expect(screen.getByText("The conditions are not a list.")).toBeTruthy();
  });

  it("passes the settings and the children of the section", () => {
    render(
      <SectionComponentDetail
        component={withRequest}
        registrations={[s3]}
        namespace="team-a"
        settings={<p>Section settings</p>}
        now={now}
      >
        <p>Request outputs</p>
      </SectionComponentDetail>,
    );
    expect(screen.getByText("Request outputs")).toBeTruthy();
    expect(detail.seen.settings).toBeTruthy();
  });
});
