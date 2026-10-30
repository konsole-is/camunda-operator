import { Lock } from "lucide-react";
import Link from "next/link";
import {
  type ClusterField,
  lockLabel,
  stateLabel,
  stateNote,
} from "@/lib/fields";

/** FieldValue shows the value of a ClusterField, or Not set when it has none. */
export function FieldValue({ value }: { value: unknown }) {
  if (value === undefined) return <span className="text-muted">Not set</span>;
  if (typeof value === "string") return <>{value}</>;
  if (typeof value === "number" || typeof value === "boolean")
    return <>{String(value)}</>;
  return (
    <code className="text-xs text-text-2 [overflow-wrap:anywhere]">
      {JSON.stringify(value)}
    </code>
  );
}

/**
 * FieldState shows where the value of field comes from, with its note and
 * its lock. With claimHref it also links the field in the Claim tab.
 */
export function FieldState({
  field,
  claimHref,
}: {
  field: ClusterField;
  claimHref?: string;
}) {
  const note = stateNote(field.state);
  return (
    <div className="flex flex-col gap-1">
      <span className="text-text-2">{stateLabel(field.state)}</span>
      {note && <span className="text-xs text-muted">{note}</span>}
      {field.lock && (
        <span className="flex items-center gap-1.5 text-xs text-muted">
          <Lock aria-hidden className="size-3.5 shrink-0" />
          {lockLabel(field.lock)}
        </span>
      )}
      {claimHref && (
        <Link
          href={claimHref}
          className="text-xs text-info underline-offset-4 hover:underline"
        >
          Open in the Claim tab
        </Link>
      )}
    </div>
  );
}
