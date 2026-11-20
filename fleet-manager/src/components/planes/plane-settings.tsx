import { Lock } from "lucide-react";
import type { ReactNode } from "react";
import { Term, Terms } from "@/components/cluster/claim-tab";
import { lockLabel, type PlaneField } from "@/lib/fields";
import {
  IDENTITY_MODE_LABEL,
  type IdentityModeId,
} from "@/lib/planes/identity";

/**
 * PlaneSettings lists fields read-only: the label, the value, and under
 * the value the lock label, or the note of a field without a lock. A
 * component switch reads On or Off, and the identity provider its mode.
 */
export function PlaneSettings({
  fields,
}: {
  fields: readonly PlaneField[];
}): ReactNode {
  if (fields.length === 0) {
    return <p className="text-sm text-muted">No settings.</p>;
  }
  return (
    <Terms>
      {fields.map((f) => (
        <Term key={f.key} label={f.label} note={f.lock ? undefined : f.note}>
          {valueText(f)}
          {f.lock && (
            <span className="flex items-start gap-1.5 text-xs text-muted">
              <Lock aria-hidden className="mt-px size-3.5 shrink-0" />
              {lockLabel(f.lock)}
            </span>
          )}
        </Term>
      ))}
    </Terms>
  );
}

const SWITCHES = ["console", "webModeler"];

function valueText(f: PlaneField): ReactNode {
  if (SWITCHES.includes(f.key)) return f.value === true ? "On" : "Off";
  if (f.key === "identityProvider" && typeof f.value === "string") {
    return IDENTITY_MODE_LABEL[f.value as IdentityModeId] ?? f.value;
  }
  const { value } = f;
  if (value === undefined || value === null || value === "") return "-";
  if (Array.isArray(value)) return value.join(", ");
  if (typeof value === "object") {
    return (
      <span className="block">
        {Object.entries(value).map(([key, v]) => (
          <span key={key} className="block break-all">
            {key}: {typeof v === "object" ? JSON.stringify(v) : String(v)}
          </span>
        ))}
      </span>
    );
  }
  return String(value);
}
