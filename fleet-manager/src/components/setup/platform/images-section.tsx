"use client";

import type { ReactNode } from "react";
import { EditorSection, TextField } from "@/components/setup/form-parts";
import {
  type DefaultRepositories,
  IMAGE_FIELDS,
  type ImageField,
  type RepositoryDefault,
} from "@/lib/merge";
import type { ValidationError } from "@/lib/schema";
import type { PlatformDraft } from "./draft";

/** IMAGE_LABELS is the label of each image field in the form. */
export const IMAGE_LABELS: Readonly<Record<ImageField, string>> = {
  camunda: "Orchestration cluster",
  connectors: "Connectors",
  optimize: "Optimize",
  identity: "Management Identity",
  console: "Console",
  webModelerRestapi: "Web Modeler REST API",
  webModelerWebsockets: "Web Modeler websockets",
  keycloak: "Keycloak",
  postgres: "PostgreSQL",
};

/**
 * ImagesSection edits the image repositories of a platform config. Under
 * each field it lists the default repository of defaults, with the range of
 * Camunda minors of each default when defaults hold more than one minor.
 * usersKnown is false while the users of the config are not known.
 */
export function ImagesSection({
  images,
  defaults,
  errors,
  usersKnown = true,
  onChange,
}: {
  images: PlatformDraft["images"];
  defaults: DefaultRepositories;
  errors: ValidationError[];
  usersKnown?: boolean;
  onChange: (images: PlatformDraft["images"]) => void;
}): ReactNode {
  const ranged = defaults.minors.length > 1;
  return (
    <EditorSection title="Image repositories">
      <p className="text-sm text-muted">
        Each value is a repository with its registry and no tag. The tag comes
        from the version of each cluster, management plane, or database server.
      </p>
      {defaults.missing.length > 0 && (
        <p className="text-sm text-warn">
          The images table has no defaults for Camunda{" "}
          {defaults.missing.join(", ")}.
        </p>
      )}
      {!defaults.fromUsers && defaults.minors.length > 0 && (
        <p className="text-sm text-muted">
          {usersKnown
            ? "No cluster or management plane uses this config."
            : "The users of this config are not known."}{" "}
          The defaults are those of Camunda {defaults.minors[0]}, the newest in
          the images table.
        </p>
      )}
      <div className="flex flex-col gap-5">
        {(Object.keys(IMAGE_FIELDS) as ImageField[]).map((field) => {
          const label = IMAGE_LABELS[field];
          const fieldDefaults = defaults.fields[field];
          return (
            <div key={field} className="flex min-w-0 flex-col gap-1.5">
              <TextField
                label={label}
                value={images?.[field] ?? ""}
                mono
                placeholder={
                  fieldDefaults.length === 1
                    ? fieldDefaults[0].repository
                    : undefined
                }
                error={
                  errors.find((e) => e.path === `spec.images.${field}`)?.message
                }
                onChange={(value) => onChange({ ...images, [field]: value })}
              />
              <ul
                aria-label={`Defaults of ${label}`}
                className="flex flex-col gap-0.5 text-xs text-muted"
              >
                {fieldDefaults.length === 0 ? (
                  <li>No default in the images table.</li>
                ) : (
                  fieldDefaults.map((d) => (
                    <li key={d.from} className="break-all">
                      {describeDefault(d, ranged)}
                    </li>
                  ))
                )}
              </ul>
            </div>
          );
        })}
      </div>
    </EditorSection>
  );
}

function describeDefault(d: RepositoryDefault, ranged: boolean): string {
  if (!ranged) return d.repository;
  return d.from === d.to
    ? `${d.repository} for Camunda ${d.from}`
    : `${d.repository} for Camunda ${d.from} to ${d.to}`;
}
