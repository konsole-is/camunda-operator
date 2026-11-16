import type { ReactNode } from "react";
import { stringify } from "yaml";
import { EditorSection } from "@/components/setup/form-parts";
import type { PresetKind } from "@/lib/usage";
import { cannotSet, presetParts } from "./merge-rules";

/**
 * PresetSettings shows each field that body sets with its value and its
 * merge rule, one section for each part, then the fields that a preset of
 * kind cannot set.
 */
export function PresetSettings({
  kind,
  body,
}: {
  kind: PresetKind;
  body: Record<string, unknown> | undefined;
}): ReactNode {
  const parts = presetParts(kind, body);
  return (
    <>
      {parts.length === 0 && (
        <p className="text-sm text-muted">
          The preset sets no field. Each resource sets its own.
        </p>
      )}
      {parts.map((part) => (
        <EditorSection key={part.title} title={part.title}>
          <section aria-label={part.title} className="flex flex-col">
            {part.settings.map((setting) => (
              <div
                key={setting.path}
                className="grid gap-x-6 gap-y-1 border-t border-border py-2.5 first:border-t-0 first:pt-0 sm:grid-cols-[minmax(10rem,14rem)_1fr_auto] sm:items-baseline"
              >
                <span className="font-mono text-sm break-all text-text">
                  {setting.path}
                </span>
                <Value value={setting.value} />
                <span className="text-sm text-muted sm:text-right">
                  {setting.rule}
                </span>
              </div>
            ))}
          </section>
        </EditorSection>
      ))}
      <EditorSection title="Fields a preset cannot set">
        <p className="max-w-2xl text-sm text-muted">
          Each resource sets these itself. The version comes from the release.
        </p>
        <ul
          aria-label="Fields a preset cannot set"
          className="flex flex-wrap gap-2"
        >
          {cannotSet(kind).map((path) => (
            <li
              key={path}
              className="rounded-sm border border-border px-2 py-0.5 font-mono text-xs text-text-2"
            >
              {path}
            </li>
          ))}
        </ul>
      </EditorSection>
    </>
  );
}

function Value({ value }: { value: unknown }) {
  if (typeof value === "object" && value !== null)
    return (
      <pre className="overflow-x-auto rounded-sm bg-panel-2 px-2 py-1 font-mono text-xs text-text">
        {stringify(value).trimEnd()}
      </pre>
    );
  return <span className="font-mono text-sm text-text">{String(value)}</span>;
}
