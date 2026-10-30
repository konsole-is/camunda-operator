import type { Story } from "@/gallery/registry";
import { KindMap } from "@/lib/k8s/kinds";
import { AccessSection } from "./access-section";
import { GitLayoutPreview } from "./git-layout-preview";
import { MetricsServerLine } from "./metrics-server-line";
import { PermissionsTable } from "./permissions-table";

const kinds = {
  namespaced: KindMap.CamundaClusterClaim,
  clusterScoped: KindMap.Environment,
};

const rules = [
  {
    apiGroups: [""],
    resources: ["configmaps"],
    resourceNames: ["fleet-manager-settings"],
    verbs: ["get", "update", "patch"],
  },
  {
    apiGroups: [""],
    resources: ["secrets"],
    resourceNames: [],
    verbs: ["get", "list", "watch"],
  },
  {
    apiGroups: ["fleet.camunda.io"],
    resources: ["camundaclusterclaims", "environments"],
    resourceNames: [],
    verbs: ["create", "delete", "get", "list", "patch", "update", "watch"],
  },
];

export const settingsPageStories: Story[] = [
  {
    component: "GitLayoutPreview",
    name: "Under a folder",
    render: () => <GitLayoutPreview path="clusters/prod" {...kinds} />,
  },
  {
    component: "GitLayoutPreview",
    name: "At the root",
    render: () => <GitLayoutPreview path="" {...kinds} />,
  },
  {
    component: "GitLayoutPreview",
    name: "Invalid path",
    render: () => <GitLayoutPreview path="a/../b" {...kinds} />,
  },
  {
    component: "PermissionsTable",
    name: "Rules",
    render: () => (
      <PermissionsTable permissions={{ rules, incomplete: false }} />
    ),
  },
  {
    component: "PermissionsTable",
    name: "Incomplete",
    render: () => (
      <PermissionsTable permissions={{ rules, incomplete: true }} />
    ),
  },
  {
    component: "PermissionsTable",
    name: "Read failed",
    render: () => (
      <PermissionsTable
        permissions={{
          error:
            'selfsubjectrulesreviews.authorization.k8s.io is forbidden: User "system:serviceaccount:fleet:fleet-manager" cannot create resource',
        }}
      />
    ),
  },
  {
    component: "AccessSection",
    name: "With the Service name",
    render: () => (
      <AccessSection
        namespace="camunda-system"
        serviceName="camunda-operator-fleet-manager"
      />
    ),
  },
  {
    component: "AccessSection",
    name: "Without the Service name",
    render: () => <AccessSection namespace="camunda-system" />,
  },
  {
    component: "MetricsServerLine",
    name: "Found",
    render: () => <MetricsServerLine state={{ found: true }} />,
  },
  {
    component: "MetricsServerLine",
    name: "Not found",
    render: () => <MetricsServerLine state={{ found: false }} />,
  },
  {
    component: "MetricsServerLine",
    name: "Check failed",
    render: () => (
      <MetricsServerLine
        state={{ error: "connect ECONNREFUSED 10.96.0.1:443" }}
      />
    ),
  },
];
