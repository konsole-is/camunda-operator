"use client";

import { type ReactNode, useId, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

/**
 * TypedConfirm is the confirm of a write that cannot be undone. Its button
 * stays disabled until the typed text is exactly name: same case, no extra
 * spaces. pending blocks the button while the write runs.
 */
export function TypedConfirm({
  name,
  confirmLabel,
  children,
  pending = false,
  onConfirm,
}: {
  name: string;
  confirmLabel: string;
  children?: ReactNode;
  pending?: boolean;
  onConfirm?: () => void;
}) {
  const [typed, setTyped] = useState("");
  return (
    <div className="flex max-w-md flex-col gap-3">
      {children && <div className="text-sm text-text-2">{children}</div>}
      <TypedField name={name} value={typed} onChange={setTyped} />
      <Button
        variant="destructive"
        className="self-start"
        disabled={typed !== name || pending}
        onClick={() => onConfirm?.()}
      >
        {confirmLabel}
      </Button>
    </div>
  );
}

/**
 * TypedField is the labeled input of a typed confirm, "Type name to
 * confirm". Its owner holds the value and compares it to name.
 */
export function TypedField({
  name,
  value,
  onChange,
}: {
  name: string;
  value: string;
  onChange: (value: string) => void;
}) {
  const id = useId();
  return (
    <div className="flex flex-col gap-3">
      <label htmlFor={id} className="text-sm text-muted">
        Type <span className="font-medium text-text">{name}</span> to confirm
      </label>
      <Input
        id={id}
        value={value}
        autoComplete="off"
        spellCheck={false}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}
