import type { V1Namespace } from "@kubernetes/client-node";
import type { CamundaCluster } from "@/generated/crds/camundacluster";
import type { CamundaClusterClaim } from "@/generated/crds/camundaclusterclaim";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { DatabaseServer } from "@/generated/crds/databaseserver";
import type { ElasticsearchCluster } from "@/generated/crds/elasticsearchcluster";
import type { Environment } from "@/generated/crds/environment";

/** ReleasesData holds every list that the release pages read, in all namespaces. */
export type ReleasesData = {
  releases: CamundaRelease[];
  environments: Environment[];
  namespaces: V1Namespace[];
  clusters: CamundaCluster[];
  elasticsearchClusters: ElasticsearchCluster[];
  databaseServers: DatabaseServer[];
  claims: CamundaClusterClaim[];
};
