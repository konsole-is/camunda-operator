import { StatusDot } from "@/components/fleet/status-dot";
import type { CardId, ComponentCardModel } from "./cards";

/**
 * ComponentCards shows one card for each model, in the order given.
 * hrefOf gives the link of a used card. A card without one, and a card that
 * is not used, is no link.
 */
export function ComponentCards({
  cards,
  hrefOf,
}: {
  cards: ComponentCardModel[];
  hrefOf?: (id: CardId) => string | undefined;
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
              href={card.used ? hrefOf?.(card.id) : undefined}
            />
          </li>
        ))}
      </ul>
    </section>
  );
}

function ComponentCard({
  card,
  href,
}: {
  card: ComponentCardModel;
  href?: string;
}) {
  const body = (
    <>
      <div className="flex items-center gap-2">
        <StatusDot
          label={card.label}
          condition={card.condition}
          off={!card.used}
        />
        <span className="font-medium text-text">{card.label}</span>
      </div>
      {card.embedded.length > 0 && (
        <ul className="flex flex-wrap gap-1.5">
          {card.embedded.map((e) => (
            <li
              key={e.process}
              className="rounded border border-dashed border-faint px-2 py-0.5 text-xs text-text-2"
            >
              Embedded: {e.label}
            </li>
          ))}
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
    </>
  );
  const frame = card.used
    ? "border-border bg-panel-2"
    : "border-dashed border-faint bg-panel";
  return (
    <article
      aria-label={card.label}
      className={`flex w-full flex-col rounded-md border ${frame}`}
    >
      {href ? (
        <a
          href={href}
          className="flex flex-1 flex-col gap-3 rounded-md p-3 outline-none hover:bg-panel focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {body}
        </a>
      ) : (
        <div className="flex flex-1 flex-col gap-3 p-3">{body}</div>
      )}
    </article>
  );
}
