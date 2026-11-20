import { notFound } from "next/navigation";
import { ReleaseDetailLive } from "@/components/setup/releases/release-detail";
import { releaseTabOf } from "@/components/setup/releases/release-tabs";
import { loadImagesTable } from "@/lib/bundled/images";
import { get } from "@/lib/k8s/read";
import { listSecretRefsIn } from "@/lib/secrets";
import { releaseSecretNamespaces } from "@/lib/usage";
import { writeMode } from "@/lib/write/writer";
import {
  applyReleaseChange,
  applyReleaseMove,
  previewReleaseChange,
  previewReleaseMove,
} from "../actions";
import { loadReleases, loadRolloutLists } from "../load";

// The page reads the cache of the server. A static prerender would call the
// API server at build time.
export const dynamic = "force-dynamic";

export default async function ReleasePage({
  params,
  searchParams,
}: PageProps<"/setup/releases/[name]">) {
  const { name } = await params;
  const { tab } = await searchParams;
  const [found, sources, rollout, mode, imagesTable] = await Promise.all([
    get("CamundaRelease", undefined, name),
    loadReleases(),
    loadRolloutLists(),
    writeMode().catch(() => undefined),
    loadImagesTable(),
  ]);
  if (!found) notFound();
  // The Secret lists are read once, when the page opens.
  const secrets = await listSecretRefsIn(
    releaseSecretNamespaces(name, sources),
  );
  return (
    <ReleaseDetailLive
      data={{ name, sources, rollout, imagesTable, secrets }}
      tab={releaseTabOf(tab)}
      mode={mode}
      actions={{
        previewChange: previewReleaseChange,
        applyChange: applyReleaseChange,
        previewMove: previewReleaseMove,
        applyMove: applyReleaseMove,
      }}
    />
  );
}
