import { backupSlots } from "@/components/cluster/backup-slots";
import { ComponentSettingsEditor } from "@/components/cluster/edit/settings-slot";
import type { ClusterSlots } from "@/components/cluster/slots";
import { TopologyEditor } from "@/components/cluster/topology-editor";
import { applyBackupChange } from "./actions";

/**
 * clusterSlots fills the action slots of the cluster page. Each write flow
 * adds one entry: the slot name and a render function over the slot
 * context. Only cluster-page.tsx imports this module, because a server
 * component cannot pass a function.
 */
export const clusterSlots: ClusterSlots = {
  ...backupSlots(applyBackupChange),
  "cluster.topology.actions": (context) => <TopologyEditor context={context} />,
  "component.settings.editor": (ctx) => <ComponentSettingsEditor ctx={ctx} />,
};
