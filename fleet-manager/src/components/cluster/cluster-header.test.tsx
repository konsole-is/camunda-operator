// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { ClusterHeader, type ClusterHeaderProps } from "./cluster-header";

afterEach(cleanup);

const header = (props: Partial<ClusterHeaderProps> = {}) => (
  <ClusterHeader
    name="orders"
    namespace="team-a"
    ready={{ status: "False", reason: "Degraded" }}
    environment="Production"
    version="8.9.23"
    preset="medium"
    storage="rdbms"
    links={[]}
    {...props}
  />
);

describe("ClusterHeader", () => {
  it("shows the name, the Ready reason, and the facts of the cluster", () => {
    render(header());
    expect(
      screen.getByRole("heading", { level: 1, name: "orders" }),
    ).toBeTruthy();
    for (const text of [
      /Degraded/,
      "Production",
      "team-a",
      "8.9.23",
      "medium",
      "Relational database",
    ]) {
      expect(screen.getByText(text)).toBeTruthy();
    }
  });

  it("says No environment for a namespace without the label", () => {
    render(header({ environment: undefined }));
    expect(screen.getByText("No environment")).toBeTruthy();
  });

  it("shows a dash for a fact that is not set", () => {
    render(
      header({ version: undefined, preset: undefined, storage: undefined }),
    );
    expect(screen.getAllByText("-")).toHaveLength(3);
  });

  it("names Elasticsearch storage", () => {
    render(header({ storage: "elasticsearch" }));
    expect(screen.getByText("Elasticsearch")).toBeTruthy();
  });

  it("shows the three launch links with their URLs", () => {
    render(
      header({
        links: [
          { label: "Operate", href: "https://c.example.com/operate" },
          { label: "Tasklist", href: "https://c.example.com/tasklist" },
          { label: "Admin", href: "https://c.example.com/admin" },
        ],
      }),
    );
    const links = screen.getAllByRole("link");
    expect(links.map((l) => [l.textContent, l.getAttribute("href")])).toEqual([
      ["Operate", "https://c.example.com/operate"],
      ["Tasklist", "https://c.example.com/tasklist"],
      ["Admin", "https://c.example.com/admin"],
    ]);
    expect(links[0].getAttribute("target")).toBe("_blank");
    expect(links[0].getAttribute("rel")).toBe("noreferrer");
  });

  it("shows no launch link without links", () => {
    render(header());
    expect(screen.queryAllByRole("link")).toEqual([]);
  });

  it("shows the actions", () => {
    render(header({ actions: <button>Delete</button> }));
    expect(screen.getByRole("button", { name: "Delete" })).toBeTruthy();
  });
});
