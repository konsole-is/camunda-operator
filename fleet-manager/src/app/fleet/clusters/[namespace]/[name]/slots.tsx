import type { ClusterSlots } from "@/components/cluster/slots";

/**
 * clusterSlots fills the action slots of the cluster page. Each write flow
 * adds one entry: the slot name and a render function over the slot
 * context. Only cluster-page.tsx imports this module, because a server
 * component cannot pass a function.
 */
export const clusterSlots: ClusterSlots = {};
