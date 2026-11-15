"use client";

import { Check, CircleAlert } from "lucide-react";
import { type ReactNode, useEffect, useId, useRef } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/** WizardStep is one step of a create flow. errors are the open errors of the step. */
export type WizardStep = { id: string; label: string; errors?: string[] };

type StepState = "Done" | "Current" | "Not started" | "Has errors";

/**
 * WizardStepper shows the steps of a create flow, numbered in order, with
 * the state of each: done before the current step, the current step, or not
 * started. A step with errors shows "Has errors". The current step renders
 * as children in a card, so children bring no card of their own. On a wide
 * screen the steps are a list left of the card. In AppShell the list moves
 * into the empty space beside the page as far as it fits, and the card
 * grows into the room it leaves. On a narrow screen the steps sit above the
 * card, with the position of the current step. The card opens with the
 * label of the current step as a heading. When the current step changes,
 * focus moves to that heading, but not on the first render. "Next" stays
 * blocked while the current step or a step before it has errors. The
 * footer lists those errors, each with the label of its step. The last
 * step has neither "Next" nor "Back": its review holds the confirm and its
 * own Back. The first step has no "Back". With onStep, a step before the
 * current one is a button that goes back to it.
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
  const blocking = steps
    .slice(0, index + 1)
    .flatMap((s) =>
      (s.errors ?? []).map((error) => ({ step: s.label, error })),
    );
  const back = index > 0 && index < steps.length - 1 && onBack !== undefined;
  const next = index < steps.length - 1;
  const heading = useRef<HTMLHeadingElement>(null);
  const id = steps[index]?.id;
  // Strict Mode runs an effect twice on mount, so a "mounted" flag would
  // move focus on the first render.
  const shown = useRef(id);
  useEffect(() => {
    if (shown.current === id) return;
    shown.current = id;
    heading.current?.focus();
  }, [id]);
  return (
    // The list and its gap are 220 px wide, so the card lines up with the
    // page once the list has moved all the way into the gutter.
    <div className="flex min-w-0 flex-col gap-4 lg:ml-[calc(-1*min(var(--page-gutter,0px),220px))] lg:grid lg:grid-cols-[196px_minmax(0,1fr)] lg:items-start lg:gap-6">
      <div className="flex flex-col gap-3 lg:sticky lg:top-6">
        <div className="flex flex-col gap-0.5 lg:hidden">
          <p className="text-sm text-muted">
            Step {index + 1} of {steps.length}
          </p>
        </div>
        <nav aria-label="Steps">
          <ol className="flex items-center lg:flex-col lg:items-stretch lg:gap-1.5">
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
                      className="group rounded-lg text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50 lg:w-full"
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
      </div>
      <div className="flex min-w-0 flex-col rounded-lg border border-border bg-panel @container">
        <div className="flex flex-col gap-4 p-4 md:p-6">
          <h2
            ref={heading}
            tabIndex={-1}
            className="font-heading text-lg font-semibold outline-none"
          >
            {steps[index]?.label}
          </h2>
          <div className="min-w-0">{children}</div>
        </div>
        {(blocking.length > 0 || back || next) && (
          <div className="flex flex-col gap-2 border-t border-divider px-4 py-3 md:px-6 md:py-4">
            {blocking.length > 0 && (
              <ul
                id={errorsId}
                className="flex flex-col gap-0.5 text-sm text-bad"
              >
                {blocking.map((e, i) => (
                  <li key={i}>
                    <span className="font-medium">{e.step}:</span> {e.error}
                  </li>
                ))}
              </ul>
            )}
            <div className="flex gap-2">
              {back && (
                <Button variant="ghost" onClick={onBack}>
                  Back
                </Button>
              )}
              {next && (
                <Button
                  disabled={blocking.length > 0}
                  aria-describedby={blocking.length ? errorsId : undefined}
                  onClick={() => onNext?.()}
                >
                  {nextLabel}
                </Button>
              )}
            </div>
          </div>
        )}
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
        "flex items-center gap-3 rounded-lg text-sm lg:min-h-13 lg:border lg:px-3 lg:py-2",
        current
          ? "text-text lg:border-accent lg:bg-panel"
          : "text-muted group-hover:text-text lg:border-transparent lg:group-hover:bg-panel",
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
        state === "Not started" && "border-border bg-panel-2 text-muted",
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
