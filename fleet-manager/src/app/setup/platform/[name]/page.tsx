import { notFound } from "next/navigation";
import { PlatformDetailLive } from "@/components/setup/platform/platform-detail";
import { loadImagesTable } from "@/lib/bundled/images";
import { get } from "@/lib/k8s/read";
import { writeMode } from "@/lib/write/writer";
import {
  applyPlatformConfigChange,
  previewPlatformConfigChange,
} from "../actions";
import { loadPlatformSources } from "../load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function PlatformConfigPage({
  params,
}: PageProps<"/setup/platform/[name]">) {
  const { name } = await params;
  const [found, sources, mode, imagesTable] = await Promise.all([
    get("CamundaPlatformConfig", undefined, name),
    loadPlatformSources(),
    writeMode().catch(() => undefined),
    loadImagesTable(),
  ]);
  if (!found) notFound();
  return (
    <PlatformDetailLive
      initial={sources}
      imagesTable={imagesTable}
      name={name}
      mode={mode}
      actions={{
        previewChange: previewPlatformConfigChange,
        applyChange: applyPlatformConfigChange,
      }}
    />
  );
}
