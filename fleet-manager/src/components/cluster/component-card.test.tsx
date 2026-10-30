// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ComponentCardModel } from "./cards";
import { ComponentCards } from "./component-card";

afterEach(cleanup);

const brokers: ComponentCardModel = {
  id: "zeebe",
  label: "Brokers",
  used: true,
  condition: { type: "ZeebeReady", status: "False", reason: "Degraded" },
  embedded: [],
  pods: ["orders-zeebe-0", "orders-zeebe-1", "orders-zeebe-2"],
  numbers: [
    { label: "Pods ready", value: "2/3" },
    { label: "Partitions with a leader", value: "2/3" },
  ],
};
const gateway: ComponentCardModel = {
  id: "gateway",
  label: "Gateway",
  used: true,
  condition: { type: "GatewayReady", status: "True", reason: "Healthy" },
  embedded: [{ process: "operate", label: "Operate" }],
  pods: ["orders-gateway-a"],
  numbers: [
    { label: "Pods ready", value: "1/1" },
    { label: "Highest use", value: "40 % cpu" },
  ],
};
const connectors: ComponentCardModel = {
  id: "connectors",
  label: "Connectors",
  used: false,
  embedded: [],
  pods: [],
  numbers: [],
};

// valueOf returns the value that a card shows for the term of label.
const valueOf = (card: HTMLElement, label: string) =>
  within(card).getByText(label, { selector: "dt" }).nextElementSibling
    ?.textContent;

describe("ComponentCards", () => {
  it("a used card shows its name, its dot, and its two numbers", () => {
    render(<ComponentCards cards={[brokers]} />);
    const card = within(
      screen.getByRole("region", { name: "Components" }),
    ).getByRole("article", { name: "Brokers" });
    expect(
      within(card).getByRole("img", { name: "Brokers: Not ready (Degraded)" }),
    ).toBeTruthy();
    expect(valueOf(card, "Pods ready")).toBe("2/3");
    expect(valueOf(card, "Partitions with a leader")).toBe("2/3");
  });

  it("a card that is not used says so and is not a link", () => {
    render(
      <ComponentCards
        cards={[connectors]}
        hrefOf={() => "?component=connectors"}
      />,
    );
    expect(
      screen.getByRole("img", { name: "Connectors: Not used" }),
    ).toBeTruthy();
    expect(screen.getByText("Not used")).toBeTruthy();
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("a used card with an href is a link", () => {
    render(
      <ComponentCards cards={[gateway]} hrefOf={(id) => `?component=${id}`} />,
    );
    const link = screen.getByRole("link", { name: /Gateway/ });
    expect(link.getAttribute("href")).toBe("?component=gateway");
  });

  it("a used card without an href is not a link", () => {
    render(<ComponentCards cards={[gateway]} />);
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("an embedded app shows inside its host", () => {
    render(<ComponentCards cards={[brokers, gateway]} />);
    expect(
      within(screen.getByRole("article", { name: "Gateway" })).getByText(
        "Embedded: Operate",
      ),
    ).toBeTruthy();
    expect(
      within(screen.getByRole("article", { name: "Brokers" })).queryByText(
        /Embedded/,
      ),
    ).toBeNull();
  });

  it("an embedded app of a linked card is a link of its own", () => {
    render(
      <ComponentCards cards={[gateway]} hrefOf={(id) => `?component=${id}`} />,
    );
    expect(
      screen
        .getByRole("link", { name: "Embedded: Operate" })
        .getAttribute("href"),
    ).toBe("?component=operate");
  });

  it("onOpen takes a plain click on a card or an embedded app", async () => {
    const onOpen = vi.fn();
    render(
      <ComponentCards
        cards={[gateway]}
        hrefOf={(id) => `?component=${id}`}
        onOpen={onOpen}
      />,
    );
    await userEvent.click(screen.getByRole("link", { name: "Gateway" }));
    await userEvent.click(
      screen.getByRole("link", { name: "Embedded: Operate" }),
    );
    expect(onOpen.mock.calls).toEqual([["gateway"], ["operate"]]);
  });

  it("the dot of a linked card sits above the link, so its title shows", () => {
    render(
      <ComponentCards cards={[brokers]} hrefOf={(id) => `?component=${id}`} />,
    );
    expect(
      screen.getByRole("img", { name: "Brokers: Not ready (Degraded)" })
        .parentElement?.className,
    ).toContain("z-10");
  });
});
