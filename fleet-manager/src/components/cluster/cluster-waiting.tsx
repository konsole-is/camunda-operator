import { NotRead } from "@/components/fleet/not-read";
import type { WaitedRequest } from "@/lib/clusters/progress";

/**
 * ClusterWaiting is the box of a cluster page whose claim has no cluster.
 * It says that the cluster does not exist yet and names each of waits, or,
 * while unread is not empty, the kinds that it could not read. With
 * deleting, it says only that the claim is being deleted. While unread
 * names CamundaCluster, it says neither.
 */
export function ClusterWaiting({
  waits,
  deleting = false,
  unread,
}: {
  waits?: readonly WaitedRequest[];
  deleting?: boolean;
  unread: readonly string[];
}) {
  const clusterUnread = unread.includes("CamundaCluster");
  const heading = clusterUnread
    ? "Cluster"
    : deleting
      ? "The cluster no longer exists"
      : "The cluster does not exist yet";
  return (
    <section
      aria-label={heading}
      className="flex flex-col gap-3 rounded-lg border border-border bg-panel p-4"
    >
      <h2 className="font-heading text-base font-semibold">{heading}</h2>
      {deleting && !clusterUnread ? (
        <p className="text-sm text-text-2">The claim is being deleted.</p>
      ) : unread.length > 0 ? (
        <NotRead kinds={unread} />
      ) : (
        waits &&
        waits.length > 0 && (
          <>
            <p className="text-sm text-text-2">
              The claim waits for the outputs of these requests:
            </p>
            <ul
              aria-label="Requests the claim waits for"
              className="flex flex-col gap-1 text-sm"
            >
              {waits.map((w, i) => (
                <li key={`${w.section}/${i}`} className="text-text">
                  {w.label}:{" "}
                  {w.request ? (
                    <span className="font-mono text-text-2">{w.request}</span>
                  ) : (
                    <span className="text-muted">Not made yet</span>
                  )}
                </li>
              ))}
            </ul>
          </>
        )
      )}
    </section>
  );
}
