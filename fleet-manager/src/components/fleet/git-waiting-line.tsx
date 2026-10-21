import { age } from "./age";

/** PushedObject is a file that the fleet manager pushed to Git, with the revision it carries. */
export type PushedObject = { path: string; revision: string; pushedAt: string };

/**
 * GitWaitingLine says that a pushed change has not reached the cluster yet.
 * It renders nothing without pushed, or when liveRevision, the revision
 * annotation of the live object, equals the pushed revision.
 */
export function GitWaitingLine({
  pushed,
  liveRevision,
  now,
}: {
  pushed?: PushedObject;
  liveRevision?: string;
  now: string;
}) {
  if (!pushed || liveRevision === pushed.revision) return null;
  return (
    <p role="status" className="text-sm text-warn">
      1 change waiting for GitOps:{" "}
      <code className="font-mono">{pushed.path}</code>, pushed{" "}
      {age(pushed.pushedAt, now)} ago.
    </p>
  );
}
