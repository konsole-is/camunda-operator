"use client";

import Link from "next/link";
import { type ReactNode, useState } from "react";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import type { ReadError } from "@/components/planes/planes-table";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type {
  Change,
  Preview,
  WriteMode,
  WriteResult,
} from "@/lib/write/types";
import type { ImagesTable } from "@/startup/bundled-data";
import { ReleaseEnvironments } from "./release-environments";
import { ReleaseSettings } from "./release-settings";
import { RELEASE_TABS, type ReleaseTab, releaseTabOf } from "./release-tabs";
import type { SourceReads } from "./rows";
import { type ReleasesData, useReleaseSources } from "./sources";

/** ReleaseActions are the writes of the release pages: the server actions on the page, fakes in tests and stories. */
export type ReleaseActions = {
  previewChange: (change: Change) => Promise<Preview>;
  applyChange: (change: Change) => Promise<WriteResult>;
};

/**
 * ReleaseDetailData is what the release page reads on the server: the name
 * of the release, the first snapshot of the sources, the images table, and
 * the Secret refs of each namespace whose Secrets an env entry can name.
 */
export type ReleaseDetailData = {
  name: string;
  sources: ReleasesData;
  imagesTable: ImagesTable;
  secrets: Readonly<Record<string, SecretRefs>>;
};

/** ReleaseDetail is the release page on the live sources, opened on tab. */
export function ReleaseDetail({
  data,
  tab,
  mode,
  actions,
}: {
  data: ReleaseDetailData;
  tab: ReleaseTab;
  mode?: WriteMode;
  actions: ReleaseActions;
}): ReactNode {
  const sources = useReleaseSources(data.sources);
  return (
    <ReleaseDetailView
      sources={sources}
      name={data.name}
      imagesTable={data.imagesTable}
      secrets={data.secrets}
      tab={tab}
      mode={mode}
      actions={actions}
    />
  );
}

/**
 * ReleaseDetailView is the page of the release name, with a tab for each
 * entry of RELEASE_TABS. A tab change sets the tab query parameter without
 * a navigation.
 */
export function ReleaseDetailView({
  sources,
  name,
  imagesTable,
  secrets,
  tab: initialTab,
  mode,
  actions,
}: {
  sources: ReleasesData & { errors: ReadError[] } & SourceReads;
  name: string;
  imagesTable: ImagesTable;
  secrets: Readonly<Record<string, SecretRefs>>;
  tab: ReleaseTab;
  mode?: WriteMode;
  actions: ReleaseActions;
}): ReactNode {
  const [tab, setTab] = useState(initialTab);
  const selectTab = (value: string) => {
    const next = releaseTabOf(value);
    setTab(next);
    window.history.replaceState(null, "", `?tab=${next}`);
  };
  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="min-w-0 font-mono text-3xl font-bold tracking-[-0.02em] break-all">
          {name}
        </h1>
        <Button asChild variant="outline">
          <Link href={`/setup/new-release?from=${encodeURIComponent(name)}`}>
            Copy this release
          </Link>
        </Button>
      </header>
      <Tabs value={tab} onValueChange={selectTab}>
        <TabsList>
          {RELEASE_TABS.map((t) => (
            <TabsTrigger key={t.value} value={t.value}>
              {t.label}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="settings">
          <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
            <ReleaseSettings
              sources={sources}
              name={name}
              imagesTable={imagesTable}
              secrets={secrets}
              mode={mode}
              actions={actions}
            />
            <ReleaseEnvironments
              release={name}
              environments={sources.environments}
            />
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
