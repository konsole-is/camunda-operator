"use client";

import { useState } from "react";
import { stringify } from "yaml";
import { SelectField } from "@/components/setup/form-parts";
import type { TemplateError } from "@/lib/gotemplate";
import type { PreviewClaim } from "./preview";

/**
 * TemplatePreview shows what a template parameter renders to for one of
 * claims, which the admin picks: a string as it is, any other value as
 * YAML. An error of the render shows with the path of its string. Without
 * claims it shows empty.
 */
export function TemplatePreview({
  claims,
  empty,
  render,
}: {
  claims: readonly PreviewClaim[];
  empty: string;
  render: (
    claim: PreviewClaim,
  ) => { value: unknown } | (TemplateError & { path: string });
}) {
  const [picked, setPicked] = useState(0);
  if (claims.length === 0) return <p className="text-sm text-muted">{empty}</p>;
  const claim = claims[Math.min(picked, claims.length - 1)];
  const result = render(claim);
  return (
    <div className="flex min-w-0 flex-col gap-2 rounded-md border border-divider p-3">
      <SelectField
        label="Preview for"
        value={String(claims.indexOf(claim))}
        options={claims.map(
          (c, i) => [String(i), `${c.name} in ${c.namespace}`] as const,
        )}
        onChange={(value) => setPicked(Number(value))}
      />
      <p className="text-xs text-muted">
        The preview shows what a new claim gets. An existing claim keeps the
        policy it recorded.
      </p>
      {claim.example && (
        <p className="text-xs text-muted">
          No claim exists yet, so the preview uses the example claim{" "}
          {claim.name} in {claim.namespace}.
        </p>
      )}
      {"error" in result ? (
        <p role="alert" className="text-sm text-bad">
          {result.path ? `${result.path}: ${result.error}` : result.error}
        </p>
      ) : (
        <pre className="max-h-72 overflow-auto rounded bg-bg p-3 font-mono text-xs whitespace-pre-wrap text-text-2">
          {typeof result.value === "string"
            ? result.value
            : stringify(result.value)}
        </pre>
      )}
    </div>
  );
}
