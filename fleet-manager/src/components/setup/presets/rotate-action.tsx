"use client";

import { type ReactNode, useEffect, useRef, useState } from "react";
import { ChangeReview } from "@/components/fleet/change-review";
import { Button } from "@/components/ui/button";
import type { Resource } from "@/lib/k8s/kinds";
import type { Change, WriteMode, WriteResult } from "@/lib/write/types";
import { rotationChange, rotationTargets } from "./rotation";
import type { PresetActions, PresetSources } from "./sources";

/**
 * RotateAction offers the rotation of the admin password of the clusters of
 * rotationTargets, with a review of the dry run before the write. It offers
 * nothing while a list of unread is not read, or when no cluster takes the
 * rotation. The button is disabled while disabled is set or without mode.
 * The review closes when the targets become unknown or none before its
 * write starts. onReviewing gets true when the review opens and false when it
 * closes. show gets the result of the write.
 */
export function RotateAction({
  preset,
  sources,
  unread,
  mode,
  disabled,
  actions,
  show,
  onReviewing,
}: {
  preset: Resource<"CamundaClusterPreset">;
  sources: PresetSources;
  unread: readonly string[];
  mode?: WriteMode;
  disabled: boolean;
  actions: PresetActions;
  show: (result: WriteResult, message: string) => void;
  onReviewing?: (open: boolean) => void;
}): ReactNode {
  const [change, setChange] = useState<Change>();
  const [writing, setWriting] = useState(false);
  const targets = rotationTargets(preset, sources);
  const n = targets.length;
  const close = () => setChange(undefined);
  if (change && !writing && (unread.length > 0 || n === 0)) close();
  const open = change !== undefined;
  const reviewing = useRef(onReviewing);
  useEffect(() => {
    reviewing.current = onReviewing;
  });
  useEffect(() => reviewing.current?.(open), [open]);
  useEffect(() => () => reviewing.current?.(false), []);

  if (unread.length > 0)
    return (
      <p className="text-sm text-muted">
        The rotation is possible when the fleet manager can read{" "}
        {unread.join(", ")}.
      </p>
    );
  if (n === 0)
    return (
      <p className="text-sm text-muted">
        No cluster takes a rotation from this preset. A rotation reaches each
        cluster with basic auth that sets no auth.basic of its own.
      </p>
    );

  return (
    <div className="flex flex-col gap-4">
      <div>
        <Button
          variant="outline"
          disabled={disabled || !mode || change !== undefined}
          onClick={() => setChange(rotationChange(preset, new Date()))}
        >
          Rotate the admin password of {n} {n === 1 ? "cluster" : "clusters"}
        </Button>
      </div>
      {change && mode && (
        <section
          aria-label="Review the rotation"
          className="rounded-md border border-border bg-panel p-5"
        >
          <ChangeReview
            change={change}
            mode={mode}
            diffLabel="Changes to the preset"
            effects={[
              `The admin password of each of these clusters changes: ${targets
                .map((t) => `${t.namespace}/${t.name}`)
                .join(", ")}.`,
              "A cluster that runs Connectors restarts its Connectors pods.",
            ]}
            confirm={{ label: "Rotate" }}
            savedMessage="The rotation of the admin password is saved."
            preview={actions.previewChange}
            apply={actions.applyChange}
            show={(result, message) => {
              show(result, message);
              close();
            }}
            onCancel={close}
            onWriting={setWriting}
          />
        </section>
      )}
    </div>
  );
}
