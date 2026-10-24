"use client";

import { ChevronRight } from "lucide-react";
import { Fragment, type ReactNode, useState } from "react";
import {
  DOT_TONE,
  type StatusTone,
  TEXT_TONE,
} from "@/components/fleet/status";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  type BackupRow,
  type BackupStorage,
  type BucketView,
  type DatabaseView,
  type RestoreInterval,
  type RestoreRow,
  type ScheduleView,
  dateTime,
} from "./backups";

const PANEL =
  "flex flex-col gap-3 rounded-lg border border-border bg-panel p-4";
const HEADING = "font-heading text-base font-semibold";

/**
 * BackupsTab is the content of the Backups tab: the backup storage and the
 * schedules, the backups, and the restores of one cluster. actions is the
 * node of the cluster.backups.actions slot, shown in the header.
 */
export function BackupsTab({
  storage,
  backups,
  restores,
  actions,
}: {
  storage: BackupStorage;
  backups: BackupRow[];
  restores: RestoreRow[];
  actions?: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-4">
      {actions && (
        <div className="flex flex-wrap items-center justify-end gap-2">
          {actions}
        </div>
      )}
      <div className="grid gap-4 lg:grid-cols-[3fr_2fr]">
        <StorageSection storage={storage} />
        <ScheduleSection schedules={storage.schedules} />
      </div>
      <BackupsSection rows={backups} />
      <RestoresSection rows={restores} />
    </div>
  );
}

function StorageSection({ storage }: { storage: BackupStorage }) {
  return (
    <section aria-label="Backup storage" className={PANEL}>
      <h2 className={HEADING}>Backup storage</h2>
      {storage.kind === "none" ? (
        <p className="text-sm text-text-2">
          This cluster has no backup storage. Backups need a bucket from the
          Backup storage section.
        </p>
      ) : (
        <dl className="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-2 text-sm">
          {storage.missing && (
            <Term label="Secondary storage">
              <span className="text-bad">{storage.missing} not found</span>
            </Term>
          )}
          <Term label="Bucket">{bucketText(storage.bucket)}</Term>
          {storage.kind === "elasticsearch" && (
            <Term label="Snapshot repository">
              {storage.repository ?? (
                <span className="text-warn">Not registered yet</span>
              )}
            </Term>
          )}
          {storage.database && <DatabaseTerms database={storage.database} />}
        </dl>
      )}
    </section>
  );
}

function DatabaseTerms({ database }: { database: DatabaseView }) {
  if (database.missing) {
    return (
      <Term label="Database server">
        <span className="text-bad">{database.missing} not found</span>
      </Term>
    );
  }
  const { pitr, archive } = database;
  return (
    <>
      <Term label="Database server">
        {database.serverConfig}
        <span className="block break-all text-xs text-muted">
          {database.endpoint}
        </span>
      </Term>
      <Term label="Point-in-time restore">
        {pitr?.enabled
          ? [
              "On",
              pitr.retentionDays !== undefined && `${pitr.retentionDays} days`,
              pitr.recovery === "operator"
                ? "recovery by the operator"
                : "recovery outside the operator",
            ]
              .filter(Boolean)
              .join(", ")
          : "Off"}
      </Term>
      {archive && (
        <>
          <Term label="Archive">
            <Tone tone={archive.state.tone} title={archive.state.message}>
              {archive.state.label}
            </Tone>
          </Term>
          {archive.bucket && (
            <Term label="Archive bucket">{bucketText(archive.bucket)}</Term>
          )}
          {archive.window && (
            <Term label="Restore window">
              <RestoreWindow window={archive.window} />
            </Term>
          )}
        </>
      )}
    </>
  );
}

function RestoreWindow({ window }: { window: RestoreInterval[] }) {
  if (window.length === 0) return <>No restore point yet</>;
  return (
    <ul className="flex flex-col gap-0.5">
      {window.map((interval, i) => (
        <Fragment key={interval.from}>
          {i > 0 && !adjoins(window[i - 1], interval) && (
            <li className="text-xs text-muted">No restore point in this gap</li>
          )}
          <li>
            {dateTime(interval.from)} to{" "}
            {interval.to ? dateTime(interval.to) : "now"}
            {interval.failing && (
              <span className="block text-xs text-warn">
                Uploads are failing after this point
              </span>
            )}
          </li>
        </Fragment>
      ))}
    </ul>
  );
}

function adjoins(before: RestoreInterval, after: RestoreInterval): boolean {
  return (
    before.to !== undefined && Date.parse(before.to) === Date.parse(after.from)
  );
}

function ScheduleSection({ schedules }: { schedules: ScheduleView[] }) {
  return (
    <section aria-label="Backup schedule" className={PANEL}>
      <h2 className={HEADING}>Schedule</h2>
      {schedules.length === 0 ? (
        <p className="text-sm text-muted">
          No backup schedule. Backups run only when someone starts one.
        </p>
      ) : (
        <ul className="flex flex-col gap-2 text-sm">
          {schedules.map((s) => (
            <li key={s.name}>
              <span className="font-medium">{s.name}</span>:{" "}
              <code className="rounded bg-panel-2 px-1 text-xs">{s.cron}</code>,
              keeps {s.completed} completed and {s.failed} failed
              {s.lastBackup && (
                <span className="block text-xs text-muted">
                  Last backup {s.lastBackup}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function BackupsSection({ rows }: { rows: BackupRow[] }) {
  const [open, setOpen] = useState<string | undefined>();
  return (
    <section aria-label="Backups" className={PANEL}>
      <h2 className={HEADING}>Backups</h2>
      {rows.length === 0 ? (
        <p className="text-sm text-muted">No backups yet.</p>
      ) : (
        <Table label="Backup list">
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Origin</TableHead>
              <TableHead>Started</TableHead>
              <TableHead>Finished</TableHead>
              <TableHead>Duration</TableHead>
              <TableHead>Phase</TableHead>
              <TableHead>Step</TableHead>
              <TableHead>Version</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => {
              const id = `backup-${row.kind}-${row.name}`;
              const expanded = open === id;
              return (
                <Fragment key={id}>
                  <TableRow>
                    <TableCell>
                      <button
                        type="button"
                        aria-expanded={expanded}
                        aria-controls={id}
                        aria-label={`Details of ${row.name}`}
                        onClick={() => setOpen(expanded ? undefined : id)}
                        className="flex items-center gap-1.5 rounded font-medium outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50"
                      >
                        <ChevronRight
                          aria-hidden
                          className={`size-3.5 text-muted transition-transform motion-reduce:transition-none ${expanded ? "rotate-90" : ""}`}
                        />
                        {row.name}
                      </button>
                    </TableCell>
                    <TableCell className="text-text-2">{row.origin}</TableCell>
                    <TableCell className="text-text-2">{row.started}</TableCell>
                    <TableCell className="text-text-2">
                      {row.finished}
                    </TableCell>
                    <TableCell className="tabular-nums">
                      {row.duration}
                    </TableCell>
                    <TableCell>
                      <Tone tone={row.tone}>{row.phase}</Tone>
                    </TableCell>
                    <TableCell className="text-text-2">{row.step}</TableCell>
                    <TableCell className="text-text-2">{row.version}</TableCell>
                  </TableRow>
                  {expanded && (
                    <TableRow id={id} className="hover:bg-transparent">
                      <TableCell colSpan={8} className="bg-panel-2/50">
                        {row.details.length === 0 ? (
                          <p className="text-sm text-muted">No details yet.</p>
                        ) : (
                          <dl className="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-sm whitespace-normal">
                            {row.details.map((d) => (
                              <Term key={d.label} label={d.label}>
                                <span className="break-all">{d.value}</span>
                              </Term>
                            ))}
                          </dl>
                        )}
                      </TableCell>
                    </TableRow>
                  )}
                </Fragment>
              );
            })}
          </TableBody>
        </Table>
      )}
    </section>
  );
}

function RestoresSection({ rows }: { rows: RestoreRow[] }) {
  return (
    <section aria-label="Restores" className={PANEL}>
      <h2 className={HEADING}>Restores</h2>
      {rows.length === 0 ? (
        <p className="text-sm text-muted">No restores.</p>
      ) : (
        <Table label="Restore list">
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>Source</TableHead>
              <TableHead>Started</TableHead>
              <TableHead>Phase</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow key={`${row.kind}-${row.name}`}>
                <TableCell className="font-medium">{row.name}</TableCell>
                <TableCell className="text-text-2">{row.label}</TableCell>
                <TableCell className="text-text-2">{row.source}</TableCell>
                <TableCell className="text-text-2">{row.started}</TableCell>
                <TableCell className="whitespace-normal">
                  <Tone tone={row.tone}>{row.phase}</Tone>
                  {row.failureMessage && (
                    <span className="block text-xs text-text-2">
                      {row.failureMessage}
                    </span>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </section>
  );
}

function Term({ label, children }: { label: string; children: ReactNode }) {
  return (
    <>
      <dt className="text-muted">{label}</dt>
      <dd className="min-w-0">{children}</dd>
    </>
  );
}

function Tone({
  tone,
  title,
  children,
}: {
  tone: StatusTone;
  title?: string;
  children: ReactNode;
}) {
  return (
    <span
      title={title}
      className={`inline-flex items-center gap-1.5 ${TEXT_TONE[tone]}`}
    >
      <span
        aria-hidden
        className={`inline-block size-2 shrink-0 rounded-full ${DOT_TONE[tone]}`}
      />
      {children}
    </span>
  );
}

function bucketText(bucket: BucketView): string {
  return bucket.location ?? `${bucket.name} (not found)`;
}
