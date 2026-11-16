import { CircleAlert, TriangleAlert } from "lucide-react";
import type { MouseEvent } from "react";
import { StatusDot } from "@/components/fleet/status-dot";
import { cn } from "@/lib/utils";
import type { CardId, CardNumber, ComponentCardModel } from "./cards";
import { statValueClass } from "./stat-tile";

/**
 * ComponentCards shows one card for each model, in the order given.
 * hrefOf gives the link of a used card and of each app embedded in it. A
 * card without one, and a card that is not used, is no link. onOpen, when
 * set, takes a plain click on a link in place of the navigation.
 */
export function ComponentCards<Id extends string = CardId>({
  cards,
  hrefOf,
  onOpen,
}: {
  cards: ComponentCardModel<Id>[];
  hrefOf?: (id: Id) => string | undefined;
  onOpen?: (id: Id) => void;
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

function ComponentCard<Id extends string>({
  card,
  hrefOf,
  onOpen,
}: {
  card: ComponentCardModel<Id>;
  hrefOf?: (id: Id) => string | undefined;
  onOpen?: (id: Id) => void;
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
        {/* Above the link that covers the card, so its title shows. */}
        <span className="relative z-10 flex">
          <StatusDot
            label={card.label}
            condition={card.condition}
            off={!card.used}
          />
        </span>
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
          {card.numbers.map((n, i) => (
            <div key={i} className="flex min-w-0 flex-col-reverse gap-0.5">
              <dt className="text-xs text-muted">{n.label}</dt>
              <dd className={numberClass(n)}>
                {n.severity && <SeverityMark severity={n.severity} />}
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

/**
 * SeverityMark is the mark in front of a value with a Warning or Error
 * severity: an icon, and the severity in words for a screen reader.
 */
export function SeverityMark({ severity }: { severity: "Warning" | "Error" }) {
  const Icon = severity === "Error" ? CircleAlert : TriangleAlert;
  return (
    <>
      <Icon aria-hidden className="mr-1 inline size-3.5 align-[-0.125em]" />
      <span className="sr-only">{severity}: </span>
    </>
  );
}

// numberClass keeps the style of a stat, and shows a value with a severity
// in the tone of that severity.
function numberClass(n: CardNumber): string {
  if (!n.severity) return cn(statValueClass(n.value), "break-words");
  return cn(
    "text-sm break-words",
    n.severity === "Error" ? "text-bad" : "text-warn",
  );
}

// opener leaves a click with a modifier key or another button to the
// browser, so that a new tab still opens.
function opener<Id extends string>(id: Id, onOpen?: (id: Id) => void) {
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
