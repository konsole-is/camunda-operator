import Link from "next/link";
import type { ReactNode } from "react";
import type { AttentionItem } from "./attention";

/**
 * AttentionPanel lists each item with its reason and message, and links an
 * item with href to its page. Without items it says that nothing needs
 * attention.
 */
export function AttentionPanel({
  items,
}: {
  items: AttentionItem[];
}): ReactNode {
  return (
    <section
      aria-label="Needs attention"
      className="flex flex-col gap-2 rounded-lg border border-border bg-panel px-4 py-3"
    >
      <h2 className="font-heading text-sm font-semibold text-text-2">
        Needs attention
      </h2>
      {items.length === 0 ? (
        <p className="text-sm text-muted">Nothing needs attention.</p>
      ) : (
        <ul className="flex flex-col gap-1.5 text-sm">
          {items.map((item) => {
            const title = `${item.what} ${item.name}`;
            const why = [item.reason, item.message].filter(Boolean).join(": ");
            return (
              <li
                key={item.id}
                className="flex min-w-0 flex-wrap items-baseline gap-x-2 border-l-2 border-bad pl-2.5 break-words"
              >
                {item.href ? (
                  <Link
                    href={item.href}
                    className="font-medium text-text hover:text-info"
                  >
                    {title}
                  </Link>
                ) : (
                  <span className="font-medium text-text">{title}</span>
                )}
                {why && <span className="min-w-0 text-muted">{why}</span>}
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
