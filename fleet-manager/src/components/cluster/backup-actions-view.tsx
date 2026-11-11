"use client";

import Link from "next/link";
import { useId, useRef, useState, useTransition } from "react";
import { InlineConfirm } from "@/components/fleet/inline-confirm";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { Change, WriteResult } from "@/lib/write/types";
import {
  type BackupContext,
  backUpNow,
  backupChange,
  backupNameErrors,
  DIRECT_NOTE,
  deleteBackupChange,
  deleteLines,
  manualBackupName,
} from "./backup-actions";
import type { BackupRow } from "./backups";

/**
 * Write is how a backup action writes. apply is the server action of the
 * change. show reports its result in the one toast of the page.
 */
type Write = {
  apply: (change: Change) => Promise<WriteResult>;
  show: (result: WriteResult, message: string) => void;
};

/**
 * BackupHeaderActions holds Back up now, Change schedule, and the note that
 * the writes skip Git. Back up now opens a form with a name. The form
 * closes when the write succeeds and stays open when it is refused.
 */
export function BackupHeaderActions({
  context,
  apply,
  show,
}: { context: BackupContext } & Write) {
  const id = useId();
  const state = backUpNow(context);
  const [name, setName] = useState<string>();
  const [writing, startWrite] = useTransition();
  const open = name !== undefined;
  const errors = open ? backupNameErrors(name, context.backupNames) : [];

  const start = () => {
    if (name === undefined || state.kind === undefined) return;
    const change = backupChange(context, state.kind, name);
    startWrite(async () => {
      const result = await apply(change);
      show(result, `Backup ${name} created.`);
      if (result.ok) setName(undefined);
    });
  };

  return (
    <div className="flex flex-col items-end gap-3">
      <div className="flex flex-wrap items-center justify-end gap-x-3 gap-y-2">
        <p className="text-xs text-muted">{DIRECT_NOTE}</p>
        <Link
          href="?tab=claim&section=attachments"
          className={buttonVariants({ variant: "outline", size: "sm" })}
        >
          Change schedule
        </Link>
        {!open && (
          <Button
            size="sm"
            disabled={state.disabled !== undefined}
            aria-describedby={state.disabled ? `${id}-reason` : undefined}
            onClick={() =>
              setName(manualBackupName(context.cluster, context.now))
            }
          >
            Back up now
          </Button>
        )}
      </div>
      {state.disabled && (
        <p id={`${id}-reason`} className="max-w-md text-right text-xs text-muted">
          {state.disabled}
        </p>
      )}
      {open && (
        <form
          aria-label="Back up now"
          className="flex w-full max-w-md flex-col gap-3 rounded-md border border-border bg-panel-2/50 p-3 text-sm"
          onSubmit={(event) => {
            event.preventDefault();
            start();
          }}
        >
          <div className="flex flex-col gap-1">
            <label htmlFor={`${id}-name`} className="font-medium">
              Name
            </label>
            <Input
              id={`${id}-name`}
              value={name}
              autoFocus
              autoComplete="off"
              spellCheck={false}
              aria-invalid={errors.length > 0 || undefined}
              aria-describedby={errors.length > 0 ? `${id}-errors` : undefined}
              onChange={(event) => setName(event.target.value)}
            />
            {errors.length > 0 && (
              <ul
                id={`${id}-errors`}
                className="flex flex-col gap-0.5 text-xs text-bad"
              >
                {errors.map((error) => (
                  <li key={error}>{error}</li>
                ))}
              </ul>
            )}
          </div>
          {state.waits && <p className="text-xs text-info">{state.waits}</p>}
          <div className="flex flex-wrap gap-2">
            <Button
              type="submit"
              size="sm"
              disabled={errors.length > 0 || writing}
            >
              Start backup
            </Button>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              onClick={() => setName(undefined)}
            >
              Cancel
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}

/** BackupRowActions holds the actions of one backup row: Delete, with a confirm that names what goes and what stays. */
export function BackupRowActions({
  context,
  row,
  apply,
  show,
}: { context: BackupContext; row: BackupRow } & Write) {
  const [, startWrite] = useTransition();
  // A second confirm while the first delete runs would send a second
  // delete, which fails because the backup is gone.
  const writing = useRef(false);
  const { goes, stays } = deleteLines(row);
  return (
    <InlineConfirm
      triggerLabel="Delete"
      confirmLabel="Delete backup"
      goes={goes}
      stays={stays}
      onConfirm={() => {
        if (writing.current) return;
        writing.current = true;
        const change = deleteBackupChange(context.namespace, row);
        startWrite(async () => {
          const result = await apply(change);
          writing.current = false;
          show(result, `Backup ${row.name} deleted.`);
        });
      }}
    />
  );
}
