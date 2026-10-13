// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TopNav } from "./top-nav";

const pathname = vi.hoisted(() => ({ current: "/fleet" }));
vi.mock("next/navigation", () => ({ usePathname: () => pathname.current }));

afterEach(cleanup);

function current() {
  const nav = screen.getByRole("navigation", { name: "Main" });
  return [...nav.querySelectorAll('[aria-current="page"]')].map((el) =>
    el.textContent?.trim(),
  );
}

describe("TopNav", () => {
  it("has two items, Fleet and Setup", () => {
    render(<TopNav />);
    const links = screen
      .getAllByRole("link")
      .map((a) => [a.textContent?.trim(), a.getAttribute("href")]);
    expect(links).toEqual([
      ["Fleet", "/fleet"],
      ["Setup", "/setup"],
    ]);
  });

  it.each([
    "/fleet",
    "/fleet/clusters/team-orders/orders",
    "/fleet/management",
    "/fleet/management/new",
    "/fleet/management/plane-a",
  ])("marks Fleet on %s", (path) => {
    pathname.current = path;
    render(<TopNav />);
    expect(current()).toEqual(["Fleet"]);
  });

  it.each(["/setup", "/setup/environments", "/setup/presets/gold"])(
    "marks Setup on %s",
    (path) => {
      pathname.current = path;
      render(<TopNav />);
      expect(current()).toEqual(["Setup"]);
    },
  );

  it("marks nothing outside both sections", () => {
    pathname.current = "/fleetwide";
    render(<TopNav />);
    expect(current()).toEqual([]);
  });

  it("uses the pathname it is given over the route", () => {
    pathname.current = "/fleet";
    render(<TopNav pathname="/setup/presets" />);
    expect(current()).toEqual(["Setup"]);
  });
});
