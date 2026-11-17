import type { ReadError } from "@/lib/k8s/unread";
import type { Resource } from "@/lib/k8s/kinds";

/**
 * PlaneClaimTabData holds the lists that the Claim tab of a plane reads
 * beside the plane page data: every CamundaCluster, CamundaPlatformConfig,
 * PluginSection, and PluginRegistration, and the PluginRequests of the
 * claim. A list whose first read failed is empty and has an entry in
 * readErrors.
 */
export type PlaneClaimTabData = {
  clusters: Resource<"CamundaCluster">[];
  platformConfigs: Resource<"CamundaPlatformConfig">[];
  sections: Resource<"PluginSection">[];
  registrations: Resource<"PluginRegistration">[];
  requests: Resource<"PluginRequest">[];
  readErrors: ReadError[];
};
