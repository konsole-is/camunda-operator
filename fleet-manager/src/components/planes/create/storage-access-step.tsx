"use client";

import { useId, useMemo } from "react";
import type { DraftPath } from "@/components/create/use-create-flow";
import { LockedField } from "@/components/fleet/locked-field";
import { SectionCards } from "@/components/fleet/section-cards";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { runningOf } from "@/lib/planes/components";
import type { PlaneContext, PlaneDraft } from "@/lib/planes/draft";
import {
  DATABASE_SUFFIX,
  EXISTING_DATABASE_CONFIGS,
  accessSelection,
  goneSections,
  managementHost,
  planeSectionViews,
} from "@/lib/planes/storage";
import { accessUrls, planeUrlFields } from "@/lib/planes/urls";
import type {
  BuiltinObjects,
  PluginSelection,
  SectionView,
} from "@/lib/policy";
import type { ValidationError } from "@/lib/schema";
import { listOf } from "@/lib/text";

const MANAGEMENT_DATABASE_SERVER = "management-database-server";

/**
 * StorageAccessStep is step 4 of the plane create flow: the database
 * option, the access option with the URLs of the running apps, the other
 * sections of planeSectionViews, and the contract name. A section that the
 * step showed before restores its selections from draft.storage; any other
 * preselects its default. Each section reports its selections to
 * draft.storage and its errors to onSectionErrors. A pick of a section that
 * no PluginSection holds any more gets a button that empties it and its
 * errors. objects are the source kinds of builtinOption, in every namespace.
 */
export function StorageAccessStep({
  draft,
  set,
  ctx,
  objects,
  onSectionErrors,
}: {
  draft: PlaneDraft;
  set: (path: DraftPath, value: unknown) => void;
  ctx: PlaneContext;
  objects: BuiltinObjects;
  onSectionErrors: (section: string, errors: ValidationError[]) => void;
}) {
  const id = useId();
  const running = runningOf(draft);
  const views = planeSectionViews(draft, ctx, objects);
  const fields = planeUrlFields(running);
  const host = managementHost(draft, ctx);
  const picked = (view: SectionView) => draft.storage[view.name]?.[0];

  const onChange =
    (name: string) =>
    (selections: PluginSelection[], errors: ValidationError[]) => {
      set(["storage", name], selections);
      onSectionErrors(name, errors);
    };

  const gone = goneSections(draft, ctx.sections);
  const remove = (name: string) => {
    set(["storage", name], []);
    onSectionErrors(name, []);
  };

  const database = views.databases && picked(views.databases);
  const apps = (["keycloak", "identity", "webModeler"] as const)
    .filter((a) => a === "identity" || running[a])
    .map((a) => `${draft.name}-${DATABASE_SUFFIX[a]}`);

  return (
    <div className="flex max-w-3xl min-w-0 flex-col gap-8">
      {gone.map((name) => (
        <div
          key={name}
          className="flex flex-col gap-3 rounded-md border border-warn/40 bg-warn/10 p-3 text-sm text-text sm:flex-row sm:items-center sm:justify-between"
        >
          <span>{`The section ${name} is no longer offered. Its pick does not apply.`}</span>
          <Button variant="outline" size="sm" onClick={() => remove(name)}>
            {`Remove the ${name} pick`}
          </Button>
        </div>
      ))}
      {views.databases && (
        <div className="flex min-w-0 flex-col gap-3">
          <StorageCards
            view={views.databases}
            draft={draft}
            onChange={onChange(views.databases.name)}
          />
          {database?.registration === MANAGEMENT_DATABASE_SERVER && (
            <p className="text-sm text-text-2">
              {`Creates one DatabaseServer ${draft.name}-db and one Database for each app that runs: ${listOf(apps)}.`}
            </p>
          )}
          {database?.registration === EXISTING_DATABASE_CONFIGS && (
            <p className="text-sm text-text-2">
              Uses the DatabaseConfigs that you pick. Each app needs its own.
            </p>
          )}
        </div>
      )}
      <div className="flex min-w-0 flex-col gap-3">
        {views.access && (
          <StorageCards
            view={views.access}
            draft={draft}
            onChange={onChange(views.access.name)}
          />
        )}
        {accessSelection(draft) ? (
          host && (
            <div className="flex flex-col gap-3">
              <LockedField
                label="Hostname"
                value={<span className="font-mono">{host.host}</span>}
                reason={`From the pattern ${host.pattern}.`}
              />
              <ul
                aria-label="URLs"
                className="flex flex-col divide-y divide-divider rounded-md border border-border text-sm"
              >
                {Object.entries(accessUrls(host.host, fields)).map(
                  ([key, url]) => (
                    <li
                      key={key}
                      className="flex flex-col gap-0.5 px-3 py-2 sm:flex-row sm:gap-4"
                    >
                      <span className="text-text-2 sm:w-48">
                        {fields.find((f) => f.id === key)?.label}
                      </span>
                      <span className="font-mono break-all text-text">
                        {url}
                      </span>
                    </li>
                  ),
                )}
              </ul>
              <p className="text-xs text-muted">
                Paths are fixed. Each app serves under its path.
              </p>
            </div>
          )
        ) : (
          <div className="flex max-w-md flex-col gap-4">
            {fields.map((field) => (
              <div key={field.id} className="flex flex-col gap-1">
                <label
                  htmlFor={`${id}-${field.id}`}
                  className="text-sm font-medium text-text"
                >
                  {`${field.label} URL`}
                </label>
                <Input
                  id={`${id}-${field.id}`}
                  type="url"
                  required
                  value={draft.urls[field.id] ?? ""}
                  placeholder={field.placeholder}
                  autoComplete="off"
                  spellCheck={false}
                  className="font-mono"
                  onChange={(event) =>
                    set(["urls", field.id], event.target.value)
                  }
                />
              </div>
            ))}
          </div>
        )}
      </div>
      {views.others?.map((view) => (
        <StorageCards
          key={view.name}
          view={view}
          draft={draft}
          onChange={onChange(view.name)}
        />
      ))}
      <section className="flex max-w-md min-w-0 flex-col gap-1">
        <label
          htmlFor={`${id}-contract`}
          className="font-heading text-base font-semibold"
        >
          Contract name
        </label>
        <p id={`${id}-contract-hint`} className="text-sm text-muted">
          The ManagementAuthConfig that Optimize reads. Empty means the plane
          name.
        </p>
        <Input
          id={`${id}-contract`}
          value={draft.contractName}
          placeholder={draft.name}
          autoComplete="off"
          spellCheck={false}
          aria-describedby={`${id}-contract-hint`}
          onChange={(event) => set(["contractName"], event.target.value)}
        />
      </section>
    </div>
  );
}

function StorageCards({
  view,
  draft,
  onChange,
}: {
  view: SectionView;
  draft: PlaneDraft;
  onChange: (selections: PluginSelection[], errors: ValidationError[]) => void;
}) {
  // SectionCards takes a new section object as new options and resets what
  // it derived, so a live list update with the same content keeps the object.
  const content = JSON.stringify(view);
  const section = useMemo(() => JSON.parse(content) as SectionView, [content]);
  const shown = Object.hasOwn(draft.storage, view.name);
  return (
    <SectionCards
      section={section}
      defaultValue={shown ? draft.storage[view.name] : undefined}
      onChange={onChange}
    />
  );
}
