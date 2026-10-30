"use client";

import { Check, CircleAlert } from "lucide-react";
import { type ReactNode, useId } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/** WizardStep is one step of a create flow. errors are the open errors of the step. */
export type WizardStep = { id: string; label: string; errors?: string[] };

/**
 * WizardStepper shows the steps of a create flow, numbered in order, with
 * the state of each: done before the current step, the current step, or not
 * started. A step with errors shows "Has errors". It renders the current
 * step as children. "Next" stays blocked while the current step has errors,
 * and the errors show under it. The last step has no "Next": its review
 * holds the confirm. The first step has no "Back". With onStep, a step
 * before the current one is a button that goes back to it.
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
    <div className="flex min-w-0 flex-col gap-6">
      <nav aria-label="Steps">
        <ol className="flex flex-wrap gap-x-6 gap-y-2">
          {steps.map((step, i) => {
            const state = step.errors?.length
              ? "Has errors"
              : i < index
                ? "Done"
                : i === index
                  ? "Current"
                  : "Not started";
            return (
              <li
                key={step.id}
                aria-current={i === index ? "step" : undefined}
                className={cn(
                  "flex items-center gap-2 text-sm",
                  i === index ? "text-text" : "text-muted",
                )}
              >
                {i < index && onStep ? (
                  <button
                    type="button"
                    onClick={() => onStep(step.id)}
                    className="-m-1 flex items-center gap-2 rounded-md p-1 hover:text-text focus-visible:ring-2 focus-visible:ring-accent focus-visible:outline-none"
                  >
                    <StepMark index={i} state={state} current={i === index} />
                    <span>{step.label}</span>
                  </button>
                ) : (
                  <>
                    <StepMark index={i} state={state} current={i === index} />
                    <span>{step.label}</span>
                  </>
                )}
                <span className="sr-only">{`, ${state}`}</span>
              </li>
            );
          })}
        </ol>
      </nav>
      <div>{children}</div>
      <div className="flex flex-col gap-2">
        {errors.length > 0 && (
          <ul id={errorsId} className="flex flex-col gap-0.5 text-sm text-bad">
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
  );
}

function StepMark({
  index,
  state,
  current,
}: {
  index: number;
  state: string;
  current: boolean;
}) {
  return (
    <span
      aria-hidden
      className={cn(
        "flex size-6 items-center justify-center rounded-full border text-xs",
        current ? "border-accent text-accent" : "border-border",
        state === "Has errors" && "border-bad text-bad",
      )}
    >
      {state === "Done" ? (
        <Check className="size-3.5" />
      ) : state === "Has errors" ? (
        <CircleAlert className="size-3.5" />
      ) : (
        index + 1
      )}
    </span>
  );
}
