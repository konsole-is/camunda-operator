"use client";

import type { ReactNode } from "react";
import type { ReadError, SourceReads } from "@/lib/k8s/unread";
import type { WriteMode } from "@/lib/write/types";
import type { ImagesTable } from "@/startup/bundled-data";
import type { ReleaseActions } from "./release-detail";
import { ReleaseSettings } from "./release-settings";
import { type ReleasesData, useReleaseSources } from "./sources";

/**
 * ReleaseCreate is the create page of a release on the live sources. It
 * starts as a copy of the release that from names, when it names one. A new
 * release has no user and no environment allows it yet, so its env entries
 * can name no Secret of a namespace.
 */
export function ReleaseCreate({
  data,
  mode,
  actions,
}: {
  data: { sources: ReleasesData; imagesTable: ImagesTable; from?: string };
  mode?: WriteMode;
  actions: ReleaseActions;
}): ReactNode {
  const sources = useReleaseSources(data.sources);
  return (
    <ReleaseCreateView
      sources={sources}
      imagesTable={data.imagesTable}
      from={data.from}
      mode={mode}
      actions={actions}
    />
  );
}

/** ReleaseCreateView is the create page of a release on sources. */
export function ReleaseCreateView({
  sources,
  imagesTable,
  from,
  mode,
  actions,
}: {
  sources: ReleasesData & { errors: ReadError[] } & SourceReads;
  imagesTable: ImagesTable;
  from?: string;
  mode?: WriteMode;
  actions: ReleaseActions;
}): ReactNode {
  return (
    <div className="flex flex-col gap-6">
      <h1 className="font-heading text-3xl font-bold tracking-[-0.02em]">
        New release
      </h1>
      <ReleaseSettings
        sources={sources}
        copyFrom={from}
        imagesTable={imagesTable}
        secrets={{}}
        mode={mode}
        actions={actions}
      />
    </div>
  );
}
