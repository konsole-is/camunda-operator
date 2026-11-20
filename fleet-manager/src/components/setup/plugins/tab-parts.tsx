import type { ReactNode } from "react";

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
