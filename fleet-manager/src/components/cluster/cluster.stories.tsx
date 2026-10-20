import { GitWaitingLine } from "@/components/fleet/git-waiting-line";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { Story } from "@/gallery/registry";
import type { Resource } from "@/lib/k8s/kinds";
import { COMPONENT_LABEL } from "@/lib/labels";
import { ActivityBanner, UnmanagedBanner } from "./banners";
import { ClusterHeader } from "./cluster-header";
import { launchLinks } from "./launch";
import { topologyOf } from "./topology";
import { TopologyView } from "./topology-view";

function pod(
  name: string,
  component: string,
  status: Resource<"Pod">["status"],
  terminating = false,
): Resource<"Pod"> {
  return {
    kind: "Pod",
    metadata: {
      name,
      labels: { [COMPONENT_LABEL]: component },
      deletionTimestamp: terminating ? "2026-10-18T10:00:00Z" : undefined,
    },
    status,
  } as Resource<"Pod">;
}

const ready = {
  phase: "Running",
  conditions: [{ type: "Ready", status: "True" }],
};
const waiting = (reason: string) => ({
  phase: "Pending",
  containerStatuses: [
    {
      name: "zeebe",
      image: "camunda/zeebe",
      imageID: "",
      ready: false,
      restartCount: 4,
      state: { waiting: { reason } },
    },
  ],
});

export const clusterStories: Story[] = [
  {
    component: "ClusterHeader",
    name: "Ready with launch links",
    render: () => (
      <ClusterHeader
        name="orders"
        namespace="team-a"
        ready={{ status: "True", reason: "Healthy" }}
        environment="Production"
        version="8.9.23"
        preset="medium"
        storage="elasticsearch"
        links={launchLinks("https://orders.example.com", undefined)}
      />
    ),
  },
  {
    component: "ClusterHeader",
    name: "Not ready without external URL",
    render: () => (
      <ClusterHeader
        name="payments"
        namespace="team-b"
        ready={{
          status: "False",
          reason: "ZeebeNotReady",
          message: "1 of 3 Zeebe brokers are ready",
        }}
        storage="rdbms"
        links={[]}
      />
    ),
  },
  {
    component: "UnmanagedBanner",
    name: "Default",
    render: () => <UnmanagedBanner />,
  },
  {
    component: "ActivityBanner",
    name: "Backup running",
    render: () => (
      <ActivityBanner
        backup={{
          kind: "LogicalBackupRDBMS",
          namespace: "team-a",
          name: "nightly-1",
        }}
        backupsHref="?tab=backups"
      />
    ),
  },
  {
    component: "ActivityBanner",
    name: "Point-in-time restore running",
    render: () => (
      <ActivityBanner
        restore={{
          kind: "PointInTimeRestore",
          namespace: "team-a",
          name: "before-incident",
        }}
        backupsHref="?tab=backups"
      />
    ),
  },
  {
    component: "TopologyView",
    name: "Standalone gateway",
    render: () => (
      <TopologyView
        boxes={topologyOf({} as CamundaClusterSpec, [
          pod("orders-zeebe-0", "zeebe", ready),
          pod("orders-zeebe-1", "zeebe", ready),
          pod("orders-zeebe-2", "zeebe", ready),
          pod("orders-gateway-7d9f-x2", "gateway", ready),
        ])}
      />
    ),
  },
  {
    component: "TopologyView",
    name: "Embedded gateway",
    render: () => (
      <TopologyView
        boxes={topologyOf(
          {
            gateway: { mode: "Embedded" },
            connectors: { enabled: true },
          } as CamundaClusterSpec,
          [
            pod("orders-zeebe-0", "zeebe", ready),
            pod("orders-connectors-5c-a1", "connectors", ready),
          ],
        )}
      />
    ),
  },
  {
    component: "TopologyView",
    name: "Pods in every state",
    render: () => (
      <TopologyView
        boxes={topologyOf(
          { operate: { mode: "Standalone" } } as CamundaClusterSpec,
          [
            pod("orders-zeebe-0", "zeebe", ready),
            pod("orders-zeebe-1", "zeebe", waiting("CrashLoopBackOff")),
            pod("orders-zeebe-2", "zeebe", waiting("ContainerCreating")),
            pod("orders-gateway-7d9f-x2", "gateway", { phase: "Failed" }),
            pod("orders-gateway-7d9f-y3", "gateway", ready, true),
          ],
        )}
      />
    ),
  },
  {
    component: "GitWaitingLine",
    name: "Waiting",
    render: () => (
      <GitWaitingLine
        pushed={{
          path: "clusters/team-a/camundaclusterclaims/orders.yaml",
          revision: "bbb",
          pushedAt: "2026-10-18T11:58:00Z",
        }}
        liveRevision="aaa"
        now="2026-10-18T12:00:00Z"
      />
    ),
  },
];
