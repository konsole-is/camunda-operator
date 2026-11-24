import type { ReactNode } from "react";
import { EditorSection } from "@/components/setup/form-parts";
import type { PresetKind } from "@/lib/usage";
import { cannotSet } from "./merge-rules";

/** CannotSetSection lists the fields that a preset of kind cannot set. */
export function CannotSetSection({ kind }: { kind: PresetKind }): ReactNode {
  return (
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
  );
}
