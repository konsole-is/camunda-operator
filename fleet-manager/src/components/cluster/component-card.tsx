import type { MouseEvent } from "react";
import { StatusDot } from "@/components/fleet/status-dot";
import type { CardId, ComponentCardModel } from "./cards";

type Open = (id: CardId) => void;

/**
 * ComponentCards shows one card for each model, in the order given.
 * hrefOf gives the link of a used card and of each app embedded in it. A
 * card without one, and a card that is not used, is no link. onOpen, when
 * set, takes a plain click on a link in place of the navigation.
 */
export function ComponentCards({
  cards,
  hrefOf,
  onOpen,
}: {
  cards: ComponentCardModel[];
  hrefOf?: (id: CardId) => string | undefined;
  onOpen?: Open;
}) {
  return (
    <section
      aria-label="Components"
      className="flex flex-col gap-3 rounded-lg border border-border bg-panel p-4"
    >
      <h2 className="font-heading text-base font-semibold">Components</h2>
      <ul className="grid gap-3 sm:grid-cols-[repeat(auto-fill,minmax(14rem,1fr))]">
        {cards.map((card) => (
          <li key={card.id} className="flex">
            <ComponentCard
              card={card}
              hrefOf={card.used ? hrefOf : undefined}
              onOpen={onOpen}
            />
          </li>
        ))}
      </ul>
    </section>
  );
}

function ComponentCard({
  card,
  hrefOf,
  onOpen,
}: {
  card: ComponentCardModel;
  hrefOf?: (id: CardId) => string | undefined;
  onOpen?: Open;
}) {
  const href = hrefOf?.(card.id);
  const frame = card.used
    ? "border-border bg-panel-2"
    : "border-dashed border-faint bg-panel";
  return (
    <article
      aria-label={card.label}
      className={`relative flex w-full flex-col gap-3 rounded-md border p-3 ${frame} ${href ? "hover:bg-panel" : ""}`}
    >
      <div className="flex items-center gap-2">
        <StatusDot
          label={card.label}
          condition={card.condition}
          off={!card.used}
        />
        {href ? (
          // The link covers the whole card, so a click anywhere opens it.
          <a
            href={href}
            onClick={opener(card.id, onOpen)}
            className="font-medium text-text outline-none after:absolute after:inset-0 after:rounded-md focus-visible:after:ring-3 focus-visible:after:ring-ring/50"
          >
            {card.label}
          </a>
        ) : (
          <span className="font-medium text-text">{card.label}</span>
        )}
      </div>
      {card.embedded.length > 0 && (
        <ul className="flex flex-wrap gap-1.5">
          {card.embedded.map((e) => {
            const appHref = hrefOf?.(e.process);
            const chip =
              "rounded border border-dashed border-faint px-2 py-0.5 text-xs text-text-2";
            return (
              <li key={e.process} className="flex">
                {appHref ? (
                  <a
                    href={appHref}
                    onClick={opener(e.process, onOpen)}
                    className={`relative z-10 ${chip} outline-none hover:border-text-2 hover:text-text focus-visible:ring-3 focus-visible:ring-ring/50`}
                  >
                    Embedded: {e.label}
                  </a>
                ) : (
                  <span className={chip}>Embedded: {e.label}</span>
                )}
              </li>
            );
          })}
        </ul>
      )}
      {card.used ? (
        <dl className="mt-auto grid grid-cols-2 gap-3">
          {card.numbers.map((n) => (
            <div key={n.label} className="flex flex-col-reverse gap-0.5">
              <dt className="text-xs text-muted">{n.label}</dt>
              <dd className="font-heading text-lg font-semibold text-text tabular-nums">
                {n.value}
              </dd>
            </div>
          ))}
        </dl>
      ) : (
        <p className="mt-auto text-sm text-muted">Not used</p>
      )}
    </article>
  );
}

// opener leaves a click with a modifier key or another button to the
// browser, so that a new tab still opens.
function opener(id: CardId, onOpen?: Open) {
  if (!onOpen) return undefined;
  return (event: MouseEvent<HTMLAnchorElement>) => {
    if (
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    ) {
      return;
    }
    event.preventDefault();
    onOpen(id);
  };
}
