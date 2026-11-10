"use client";

import type { WriteMode } from "@/lib/write/types";
import {
  EnvironmentEditor,
  type EnvironmentActions,
} from "./environment-editor";
import type { EnvironmentSources } from "./sources";

const FAKE_ACTIONS: EnvironmentActions = {
  previewChange: async () => ({ objects: [], errors: [] }),
  applyChange: async () => ({ ok: true, mode: "direct" }),
  previewLabel: async () => ({ objects: [], errors: [] }),
  applyLabel: async () => ({ ok: true, mode: "direct" }),
};

/**
 * EnvironmentEditorStory shows EnvironmentEditor in the gallery. A story
 * passes no event handler, so the writes of this wrapper preview clean and
 * succeed without a call to the server.
 */
export function EnvironmentEditorStory({
  sources,
  name,
  mode,
}: {
  sources: EnvironmentSources;
  name?: string;
  mode: WriteMode;
}) {
  return (
    <EnvironmentEditor
      sources={{ ...sources, errors: [] }}
      name={name}
      mode={mode}
      actions={FAKE_ACTIONS}
    />
  );
}
