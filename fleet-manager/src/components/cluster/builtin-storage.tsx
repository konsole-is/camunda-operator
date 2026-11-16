import { ConditionList } from "@/components/fleet/condition-list";
import { ConditionBadge } from "@/components/setup/plugins/condition-badge";
import {
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Term, Terms } from "./claim-tab";
import { DetailSection } from "./detail-section";
import { NotRead } from "./not-read";
import { type BuiltinStorage as Storage, storageFacts } from "./requests";

const NOT_REPORTED = "Not reported yet";

/**
 * BuiltinStorage shows the ElasticsearchCluster or DatabaseServer of a
 * request of built-in storage, with what its status holds: the version,
 * the snapshot repository or the archive and the recovery, the volumes,
 * and the conditions. A field that the status does not hold says "Not
 * reported yet". unread holds the kinds that the page could not read or
 * that are out of date: while it holds the kind of storage, the box names
 * that kind, shows its conditions and each missing field as Not known,
 * and says nothing of missing volumes. An object that is not
 * in the namespace shows nothing, unless its kind is unread. now is the
 * time that the ages count to.
 */
export function BuiltinStorage({
  storage,
  unread,
  now,
}: {
  storage: Storage;
  unread: readonly string[];
  now: string;
}) {
  const kindUnread = unread.includes(storage.kind);
  if (!storage.object && !kindUnread) return null;
  const facts = storage.object && storageFacts(storage);
  // A list that is not current cannot say that a field is missing.
  const missing = kindUnread ? "Not known" : NOT_REPORTED;
  return (
    <DetailSection title={storage.kind} note={storage.name} region>
      {kindUnread && <NotRead kinds={[storage.kind]} />}
      {facts && (
        <>
          {facts.errors.map((error) => (
            <p key={error} className="text-sm text-bad">
              {error}
            </p>
          ))}
          <Terms>
            <Term label="Version">{facts.version ?? missing}</Term>
            {storage.kind === "ElasticsearchCluster" ? (
              <Term label="Snapshot repository">
                {facts.snapshotRepository ?? missing}
              </Term>
            ) : (
              <>
                <Term label="Archive from">
                  {facts.archive?.reachableFrom ?? missing}
                </Term>
                {facts.archive?.boundary && (
                  <Term label="Archive boundary">{facts.archive.boundary}</Term>
                )}
                <Term label="Recovery request">
                  {facts.recovery?.requestID ?? missing}
                </Term>
                {facts.recovery?.targetTime && (
                  <Term label="Recovery target">
                    {facts.recovery.targetTime}
                  </Term>
                )}
                {facts.recovery?.startedAt && (
                  <Term label="Recovery started">
                    {facts.recovery.startedAt}
                  </Term>
                )}
              </>
            )}
          </Terms>
          {facts.volumes.length > 0 ? (
            <table aria-label="Volumes" className="w-full text-sm">
              <TableHeader>
                <TableRow>
                  <TableHead>Volume</TableHead>
                  <TableHead>Capacity</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {facts.volumes.map((volume, i) =>
                  "error" in volume ? (
                    <TableRow key={i}>
                      <TableCell colSpan={2} className="whitespace-normal">
                        <span className="text-bad">{volume.error}</span>
                      </TableCell>
                    </TableRow>
                  ) : (
                    <TableRow key={volume.name}>
                      <TableCell className="font-mono text-xs whitespace-normal wrap-anywhere">
                        {volume.name}
                      </TableCell>
                      <TableCell>{volume.capacity}</TableCell>
                    </TableRow>
                  ),
                )}
              </TableBody>
            </table>
          ) : (
            !kindUnread && (
              <p className="text-sm text-muted">No volumes reported yet</p>
            )
          )}
          {kindUnread ? (
            <span className="flex flex-wrap gap-1.5">
              <ConditionBadge type="Ready" unknown />
            </span>
          ) : (
            <ConditionList
              label={`${storage.name} conditions`}
              conditions={facts.conditions}
              now={now}
            />
          )}
        </>
      )}
    </DetailSection>
  );
}
