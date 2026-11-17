"use client";

import { TriangleAlert } from "lucide-react";
import { type ReactNode, useId } from "react";
import { Checkbox } from "@/components/ui/checkbox";
import { managerKind } from "@/lib/merge";
import { offered } from "@/lib/write/ownership";
import { refKey } from "@/lib/write/refs";
import type {
  ApiError,
  ForeignField,
  ForeignManager,
  ObjectOwnership,
  ObjectRef,
} from "@/lib/write/types";

const TITLE = "Fields that another tool set";

/**
 * TakeOverChoice shows the fields of other field managers that a save
 * changes or clears, from Preview.ownership, with one checkbox that takes
 * them over. It heads the fields of each object with labelOf when
 * ownership holds more than one object. It offers no checkbox when no
 * field is offered, or when a path names more than one field of an
 * object, because the server refuses that take-over.
 * onChoose gets the new state of the checkbox. The caller holds chosen,
 * and must reset it for each new change.
 */
export function TakeOverChoice({
  ownership,
  chosen,
  onChoose,
  labelOf = defaultLabel,
}: {
  ownership: readonly ObjectOwnership[];
  chosen: boolean;
  onChoose(chosen: boolean): void;
  labelOf?(ref: ObjectRef): string;
}): ReactNode {
  const checkboxId = useId();
  const headingId = useId();
  const choosable = canChoose(ownership);
  const named = ownership
    .flatMap((o) => [...o.changed, ...o.cleared])
    .flatMap(rowManagers);
  const gitops = namesOf(named, (m) => m.kind === "gitops");
  const unknown = namesOf(named, (m) => !m.known);
  const several = ownership.length > 1;

  return (
    <section
      role="group"
      aria-label={TITLE}
      className="flex flex-col gap-3 rounded-md border border-border p-3 text-sm"
    >
      <h3 className="font-heading text-sm font-semibold">{TITLE}</h3>
      {ownership.map((object, i) => {
        const label = labelOf(object.ref);
        return (
          <div key={refKey(object.ref)} className="flex flex-col gap-1">
            {several && (
              <h4 id={`${headingId}-${i}`} className="font-medium text-text">
                {label}
              </h4>
            )}
            <ul
              {...(several
                ? { "aria-labelledby": `${headingId}-${i}` }
                : { "aria-label": label })}
              className="flex flex-col gap-1 text-text-2"
            >
              {object.changed.map((field, j) => (
                <li key={`changed-${j}`}>
                  {rowOf(field, "changed", chosen, choosable)}
                </li>
              ))}
              {object.cleared.map((field, j) => (
                <li key={`cleared-${j}`}>
                  {rowOf(field, "cleared", chosen, choosable)}
                </li>
              ))}
              {ambiguousPaths(object).map((path) => (
                <li key={`ambiguous-${path}`} className="text-text">
                  <Path>{path}</Path> names more than one field of the{" "}
                  {label}. The fleet manager does not take over a field that it
                  cannot name exactly.
                </li>
              ))}
            </ul>
          </div>
        );
      })}
      {(gitops.length > 0 || unknown.length > 0) && (
        <ul className="flex flex-col gap-1 text-text">
          {gitops.map((name) => (
            <li key={`gitops-${name}`} className="flex gap-2">
              <TriangleAlert
                aria-hidden
                className="mt-0.5 size-4 shrink-0 text-warn"
              />
              {managerKind(name) === "gitops" ? (
                <span>
                  <Path>{name}</Path> is a GitOps tool. On its next sync it sets
                  its own value again, unless the source in Git changes too.
                </span>
              ) : (
                <span>
                  <Path>{name}</Path> set a field of an object that a GitOps
                  tool manages. On its next sync the GitOps tool can set its
                  own value again, unless the source in Git changes too.
                </span>
              )}
            </li>
          ))}
          {unknown.map((name) => (
            <li key={`unknown-${name}`}>
              <Path>{name}</Path>: The fleet manager does not know this tool.
            </li>
          ))}
        </ul>
      )}
      {choosable && (
        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            <Checkbox
              id={checkboxId}
              checked={chosen}
              onCheckedChange={(on) => onChoose(on === true)}
            />
            <label htmlFor={checkboxId} className="font-medium text-text">
              Take over these fields
            </label>
          </div>
          <p className="text-muted">
            After the take-over, kubectl apply --server-side of an older file
            fails with a conflict on these fields. A plain kubectl apply of that
            file sets the old values again.
          </p>
        </div>
      )}
      {!(choosable && chosen) &&
        ownership.some((o) => o.changed.some(offered)) && (
          <p className="font-medium text-text">
            {choosable
              ? "Take over the fields to save, or go back and leave them as they are."
              : "To save, go back and leave the changed fields as they are, or change them with the tool that set them and open the page again."}
          </p>
        )}
    </section>
  );
}

/**
 * takeOverOpen reports whether ownership stops the save: an offered changed
 * field while chosen is false or the checkbox of TakeOverChoice is not
 * offered, or a changed field that a controller of the operator owns.
 */
export function takeOverOpen(
  ownership: readonly ObjectOwnership[],
  chosen: boolean,
): boolean {
  const changed = ownership.flatMap((o) => o.changed);
  if (changed.some((f) => !offered(f))) return true;
  return changed.some(offered) && !(chosen && canChoose(ownership));
}

/**
 * withoutTakeOverConflicts returns errors without the Conflict errors of
 * each object that has an offered changed field in ownership, because
 * TakeOverChoice names those fields. Pass it only the errors of the
 * preview without the take-over.
 */
export function withoutTakeOverConflicts(
  errors: readonly ApiError[],
  ownership: readonly ObjectOwnership[],
): ApiError[] {
  const covered = new Set(
    ownership.filter((o) => o.changed.some(offered)).map((o) => refKey(o.ref)),
  );
  return errors.filter(
    (e) =>
      e.code !== "Conflict" ||
      e.ref === undefined ||
      !covered.has(refKey(e.ref)),
  );
}

function defaultLabel(ref: ObjectRef): string {
  return ref.namespace
    ? `${ref.kind} ${ref.name} in ${ref.namespace}`
    : `${ref.kind} ${ref.name}`;
}

function rowOf(
  field: ForeignField,
  kind: "changed" | "cleared",
  chosen: boolean,
  choosable: boolean,
): ReactNode {
  const managers = rowManagers(field);
  if (managers.some((m) => m.kind === "operator"))
    return (
      <>
        <Path>{field.path}</Path>: {joined(managers)} of the operator sets it.
        The fleet manager does not take it over, because the operator sets it
        again.{kind === "changed" && " Leave it as it is to save."}
      </>
    );
  const outcome =
    kind === "changed"
      ? "The save changes it."
      : !choosable
        ? "It stays."
        : chosen
          ? "The save removes it."
          : "It stays unless you take it over.";
  return (
    <>
      <Path>{field.path}</Path>: {joined(managers)} set it. {outcome}
    </>
  );
}

function Path({ children }: { children: string }) {
  return <span className="font-mono text-text">{children}</span>;
}

// A list names each path once, so the server cannot tell two fields of one
// path apart and refuses the take-over.
function ambiguousPaths(object: ObjectOwnership): string[] {
  const fields = [...object.changed, ...object.cleared];
  return [
    ...new Set(
      fields
        .filter(offered)
        .map((f) => f.path)
        .filter((path) => fields.filter((f) => f.path === path).length > 1),
    ),
  ];
}

function canChoose(ownership: readonly ObjectOwnership[]): boolean {
  return (
    ownership.some((o) => [...o.changed, ...o.cleared].some(offered)) &&
    ownership.every((o) => ambiguousPaths(o).length === 0)
  );
}

function joined(managers: readonly ForeignManager[]): string {
  return [...new Set(managers.map((m) => m.name))].join(" and ");
}

function rowManagers(field: ForeignField): ForeignManager[] {
  const operator = field.managers.filter((m) => m.kind === "operator");
  return operator.length > 0 ? operator : field.managers;
}

function namesOf(
  managers: readonly ForeignManager[],
  which: (m: ForeignManager) => boolean,
): string[] {
  return [...new Set(managers.filter(which).map((m) => m.name))];
}
