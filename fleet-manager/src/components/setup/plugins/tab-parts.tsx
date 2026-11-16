import type { ReactNode } from "react";

/** NotKnown says that what it stands for is not known, because kinds are not read. */
export function NotKnown({ kinds }: { kinds: readonly string[] }): ReactNode {
  return (
    <p className="text-sm text-muted">
      {`Not known: ${kinds.join(", ")} ${kinds.length > 1 ? "are" : "is"} not read.`}
    </p>
  );
}

/** ValueError shows text as the error of a value that the page cannot use. */
export function ValueError({ text }: { text: string }): ReactNode {
  return (
    <p role="alert" className="text-sm text-bad">
      {text}
    </p>
  );
}

/** LINK is the class of a link in the text of a tab, with a 24 px hit area. */
export const LINK =
  "relative text-info underline-offset-2 hit-area hover:underline";
