// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { type ComponentCardModel, overviewHref } from "./cards";
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

  it("shows a section card with its headline and opens it by its id", async () => {
    const onOpen = vi.fn();
    const access: ComponentCardModel = {
      id: "section:access/gateway-access",
      label: "Access",
      used: true,
      condition: { type: "Ready", status: "True", reason: "Healthy" },
      embedded: [],
      pods: [],
      numbers: [
        { label: "Host", value: "orders.example.com" },
        { label: "Resources", value: "1" },
      ],
    };
    render(
      <ComponentCards cards={[access]} hrefOf={overviewHref} onOpen={onOpen} />,
    );
    const card = screen.getByRole("article", { name: "Access" });
    expect(valueOf(card, "Host")).toBe("orders.example.com");
    const link = within(card).getByRole("link", { name: "Access" });
    expect(link.getAttribute("href")).toBe(
      overviewHref("section:access/gateway-access"),
    );
    await userEvent.click(link);
    expect(onOpen.mock.calls).toEqual([["section:access/gateway-access"]]);
  });

  it("shows the error of a rejected request on its card", () => {
    const backup: ComponentCardModel = {
      id: "section:backup-storage",
      label: "Backup storage",
      used: true,
      condition: {
        type: "Ready",
        status: "False",
        reason: "GrantRejected",
        message: "Grant orders-read has Ready=False",
      },
      embedded: [],
      pods: [],
      numbers: [{ label: "Bucket", value: "orders-backups" }],
    };
    render(<ComponentCards cards={[backup]} />);
    expect(
      screen.getByRole("img", {
        name: "Backup storage: Not ready (GrantRejected)",
      }),
    ).toBeTruthy();
  });

  it.each([
    ["Error", "text-bad"],
    ["Warning", "text-warn"],
  ] as const)("marks a headline with %s severity", (severity, tone) => {
    const key: ComponentCardModel = {
      id: "section:encryption-key",
      label: "Encryption key",
      used: true,
      embedded: [],
      pods: [],
      numbers: [{ label: "Key", value: "Pending deletion", severity }],
    };
    render(<ComponentCards cards={[key]} />);
    const card = screen.getByRole("article", { name: "Encryption key" });
    expect(within(card).getByText("Pending deletion").className).toContain(
      tone,
    );
  });
});
