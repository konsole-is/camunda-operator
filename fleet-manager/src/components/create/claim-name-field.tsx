"use client";

import { useId, useState } from "react";
import { Input } from "@/components/ui/input";
import type { RenderedHost } from "@/lib/claims/hosts";
import { MAX_CLAIM_NAME } from "@/lib/claims/names";

/**
 * ClaimNameField is the name of a new claim, with the hosts that the name
 * gives under it. errors show only after the first edit, so an empty form
 * does not open with an error.
 */
export function ClaimNameField({
  value,
  onChange,
  errors,
  hosts,
}: {
  value: string;
  onChange?: (name: string) => void;
  errors: readonly string[];
  hosts: readonly RenderedHost[];
}) {
  const id = useId();
  const [edited, setEdited] = useState(false);
  const shown = edited ? errors : [];
  return (
    <div className="flex max-w-md flex-col gap-1">
      <label
        htmlFor={`${id}-name`}
        className="font-heading text-base font-semibold"
      >
        Name
      </label>
      <p id={`${id}-hint`} className="text-sm text-muted">
        {`Lowercase letters, digits, and '-', at most ${MAX_CLAIM_NAME} characters.`}
      </p>
      <Input
        id={`${id}-name`}
        value={value}
        autoComplete="off"
        spellCheck={false}
        aria-invalid={shown.length > 0 || undefined}
        aria-describedby={
          shown.length > 0 ? `${id}-hint ${id}-errors` : `${id}-hint`
        }
        onChange={(event) => {
          setEdited(true);
          onChange?.(event.target.value);
        }}
      />
      {shown.length > 0 && (
        <ul
          id={`${id}-errors`}
          className="flex flex-col gap-0.5 text-xs text-bad"
        >
          {shown.map((error) => (
            <li key={error}>{error}</li>
          ))}
        </ul>
      )}
      {hosts.length > 0 && (
        <div className="mt-1 text-xs text-muted">
          <p>The cluster gets these hosts:</p>
          <ul className="font-mono text-text-2">
            {hosts.map((h) => (
              <li key={h.parameter}>{h.host}</li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
