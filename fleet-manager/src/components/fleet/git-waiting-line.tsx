import { age } from "./age";

/** PushedObject is a file that the fleet manager pushed to Git, with the revision it carries. */
export type PushedObject = { path: string; revision: string; pushedAt: string };

/** PushedDelete is a file that the fleet manager removed from Git. */
export type PushedDelete = { path: string; pushedAt: string; deleted: true };

/**
 * GitWaitingLine says that a pushed change has not reached the cluster yet.
 * It renders nothing without pushed, or when liveRevision, the revision
 * annotation of the live object, equals the revision of a PushedObject. A
 * PushedDelete always renders its line and ignores liveRevision.
 */
export function GitWaitingLine({
  pushed,
  liveRevision,
  now,
}: {
  pushed?: PushedObject | PushedDelete;
  liveRevision?: string;
  now: string;
}) {
  if (!pushed) return null;
  const deleted = "deleted" in pushed;
  if (!deleted && liveRevision === pushed.revision) return null;
  return (
    <p role="status" className="text-sm text-warn">
      1 {deleted ? "delete" : "change"} waiting for GitOps:{" "}
      <code className="font-mono">{pushed.path}</code>, pushed{" "}
      {age(pushed.pushedAt, now)} ago.
    </p>
  );
}
