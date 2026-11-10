"use client";

import type { ValidationError } from "@/lib/schema";
import type { EnvironmentDraft } from "./draft";
import { Swatch } from "./environment-name";
import { EditorSection, TextField } from "./form-parts";

/** BasicsSection edits the name of a new environment, the display name, and the color. */
export function BasicsSection({
  draft,
  isNew,
  errors,
  onName,
  onSpec,
}: {
  draft: EnvironmentDraft;
  isNew: boolean;
  errors: readonly ValidationError[];
  onName: (name: string) => void;
  onSpec: (spec: EnvironmentDraft["spec"]) => void;
}) {
  const errorAt = (path: string) =>
    errors.find((e) => e.path === path)?.message;
  return (
    <EditorSection title="Basics">
      {isNew ? (
        <TextField
          label="Name"
          value={draft.name}
          mono
          hint="A DNS name of at most 63 characters. It cannot change later."
          error={errorAt("name")}
          onChange={onName}
        />
      ) : (
        <div className="flex flex-col gap-1">
          <p className="text-sm font-medium text-text">Name</p>
          <p className="font-mono text-sm">{draft.name}</p>
        </div>
      )}
      <TextField
        label="Display name"
        value={draft.spec.displayName ?? ""}
        error={errorAt("spec.displayName")}
        onChange={(displayName) => onSpec({ ...draft.spec, displayName })}
      />
      <TextField
        label="Color"
        value={draft.spec.color ?? ""}
        mono
        placeholder="#2E7D32"
        error={errorAt("spec.color")}
        onChange={(color) => onSpec({ ...draft.spec, color })}
        after={<Swatch color={draft.spec.color} />}
      />
    </EditorSection>
  );
}
