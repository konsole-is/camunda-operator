"use client";

import { Check, CircleAlert } from "lucide-react";
import { type ReactNode, useId } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/** WizardStep is one step of a create flow. errors are the open errors of the step. */
export type WizardStep = { id: string; label: string; errors?: string[] };

type StepState = "Done" | "Current" | "Not started" | "Has errors";

/**
 * WizardStepper shows the steps of a create flow, numbered in order, with
 * the state of each: done before the current step, the current step, or not
 * started. A step with errors shows "Has errors". On a wide screen the
 * steps are a list in a side card next to the current step, which renders
 * as children. On a narrow screen the card sits above the step and shows
 * the label of the current step only. "Next" stays blocked while the
 * current step has errors, and the errors show under it. The last step has
 * no "Next": its review holds the confirm. The first step has no "Back".
 * With onStep, a step before the current one is a button that goes back to
 * it.
 */
export function WizardStepper({
  steps,
  current,
  children,
  nextLabel = "Next",
  onNext,
  onBack,
  onStep,
}: {
  steps: WizardStep[];
  current: string;
  children: ReactNode;
  nextLabel?: string;
  onNext?: () => void;
  onBack?: () => void;
  onStep?: (id: string) => void;
}) {
  const errorsId = useId();
  const index = Math.max(
    0,
    steps.findIndex((s) => s.id === current),
  );
  const errors = steps[index]?.errors ?? [];
  return (
    <div className="flex min-w-0 flex-col gap-6 lg:grid lg:grid-cols-[17rem_minmax(0,1fr)] lg:items-start lg:gap-8">
      <aside className="flex flex-col gap-3 rounded-lg border border-border bg-panel p-4 lg:sticky lg:top-6 lg:gap-1 lg:p-2">
        <div className="flex flex-col gap-0.5 lg:px-3 lg:pt-2 lg:pb-1">
          <p className="text-sm text-muted">
            Step {index + 1} of {steps.length}
          </p>
          {/* The list names the current step to a screen reader already. */}
          <p
            aria-hidden
            className="font-heading text-base font-semibold lg:hidden"
          >
            {steps[index]?.label}
          </p>
        </div>
        <nav aria-label="Steps">
          <ol className="flex items-center lg:flex-col lg:items-stretch lg:gap-0.5">
            {steps.map((step, i) => {
              const state: StepState = step.errors?.length
                ? "Has errors"
                : i < index
                  ? "Done"
                  : i === index
                    ? "Current"
                    : "Not started";
              const body = (
                <StepBody
                  index={i}
                  label={step.label}
                  state={state}
                  current={i === index}
                />
              );
              return (
                <li
                  key={step.id}
                  aria-current={i === index ? "step" : undefined}
                  className="flex flex-1 items-center last:flex-none lg:flex-none"
                >
                  {i < index && onStep ? (
                    <button
                      type="button"
                      onClick={() => onStep(step.id)}
                      className="rounded-md text-left outline-none hover:text-text focus-visible:ring-3 focus-visible:ring-ring/50 lg:w-full lg:hover:bg-panel-2"
                    >
                      {body}
                    </button>
                  ) : (
                    <div className="lg:w-full">{body}</div>
                  )}
                  {i < steps.length - 1 && (
                    <span
                      aria-hidden
                      className="mx-1.5 h-px min-w-2 flex-1 bg-border lg:hidden"
                    />
                  )}
                </li>
              );
            })}
          </ol>
        </nav>
      </aside>
      <div className="flex min-w-0 flex-col gap-6">
        <div>{children}</div>
        <div className="flex flex-col gap-2">
          {errors.length > 0 && (
            <ul
              id={errorsId}
              className="flex flex-col gap-0.5 text-sm text-bad"
            >
              {errors.map((e, i) => (
                <li key={i}>{e}</li>
              ))}
            </ul>
          )}
          <div className="flex gap-2">
            {index > 0 && onBack && (
              <Button variant="ghost" onClick={onBack}>
                Back
              </Button>
            )}
            {index < steps.length - 1 && (
              <Button
                disabled={errors.length > 0}
                aria-describedby={errors.length ? errorsId : undefined}
                onClick={() => onNext?.()}
              >
                {nextLabel}
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

// StepBody is the mark of a step and, on a wide screen, its label and its
// state. On a narrow screen only the mark shows.
function StepBody({
  index,
  label,
  state,
  current,
}: {
  index: number;
  label: string;
  state: StepState;
  current: boolean;
}) {
  const shown = state === "Done" || state === "Has errors";
  return (
    <span
      className={cn(
        "flex items-center gap-3 rounded-md text-sm lg:min-h-11 lg:px-3 lg:py-1.5",
        current ? "text-text lg:bg-panel-2" : "text-muted",
      )}
    >
      <StepMark index={index} state={state} />
      <span className="flex min-w-0 flex-col max-lg:sr-only">
        <span className={cn(current && "font-medium")}>{label}</span>
        <span
          className={cn(
            shown ? "text-xs" : "sr-only",
            state === "Has errors" ? "text-bad" : "text-muted",
          )}
        >
          <span className="sr-only">, </span>
          {state}
        </span>
      </span>
    </span>
  );
}

function StepMark({ index, state }: { index: number; state: StepState }) {
  return (
    <span
      aria-hidden
      className={cn(
        "flex size-7 shrink-0 items-center justify-center rounded-full border text-xs font-semibold",
        state === "Current" && "border-accent bg-accent text-bg",
        state === "Done" && "border-transparent bg-ok/15 text-ok",
        state === "Has errors" && "border-bad text-bad",
        state === "Not started" && "border-border text-muted",
      )}
    >
      {state === "Done" ? (
        <Check className="size-4" />
      ) : state === "Has errors" ? (
        <CircleAlert className="size-4" />
      ) : (
        index + 1
      )}
    </span>
  );
}
