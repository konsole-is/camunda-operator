import { notFound } from "next/navigation";
import { presetKindOf } from "@/components/setup/presets/kinds";
import type { PresetObject } from "@/components/setup/presets/kinds";
import { PresetDetailLive } from "@/components/setup/presets/preset-detail";
import {
  secretNamespaces,
  secretsUnread,
} from "@/components/setup/presets/secrets";
import { get } from "@/lib/k8s/read";
import { listSecretRefsIn } from "@/lib/secrets";
import { writeMode } from "@/lib/write/writer";
import { applyPresetChange, previewPresetChange } from "../../actions";
import { loadPresets } from "../../load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function PresetPage({
  params,
}: PageProps<"/setup/presets/[kind]/[name]">) {
  const { kind: slug, name } = await params;
  const kind = presetKindOf(slug);
  if (!kind) notFound();
  const [found, sources, mode] = await Promise.all([
    get(kind, undefined, name),
    loadPresets(),
    writeMode().catch(() => undefined),
  ]);
  if (!found) notFound();
  // The Secret lists are read once, when the page opens.
  const secrets = await listSecretRefsIn(
    secretNamespaces(found as PresetObject, sources, sources.readErrors),
  );
  return (
    <PresetDetailLive
      initial={sources}
      secrets={secrets}
      secretsUnread={secretsUnread(kind, sources.readErrors)}
      kind={kind}
      name={name}
      mode={mode}
      actions={{
        previewChange: previewPresetChange,
        applyChange: applyPresetChange,
      }}
    />
  );
}
