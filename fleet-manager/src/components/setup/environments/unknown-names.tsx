"use client";

import { Button } from "@/components/ui/button";

/**
 * UnknownNames lists names of the spec that name no object, under label,
 * each with a Remove button. It renders nothing for no name.
 */
export function UnknownNames({
  label,
  text,
  names,
  onRemove,
}: {
  label: string;
  text: string;
  names: readonly string[];
  onRemove: (name: string) => void;
}) {
  if (names.length === 0) return null;
  return (
    <div
      role="group"
      aria-label={label}
      className="flex flex-col gap-2 rounded-md border border-bad/40 bg-bad/10 p-3 text-sm"
    >
      <p className="text-text">{text}</p>
      <ul className="flex flex-wrap gap-2">
        {names.map((name) => (
          <li key={name} className="flex items-center gap-2">
            <span className="font-mono">{name}</span>
            <Button
              variant="outline"
              size="xs"
              aria-label={`Remove ${name}`}
              onClick={() => onRemove(name)}
            >
              Remove
            </Button>
          </li>
        ))}
      </ul>
    </div>
  );
}
