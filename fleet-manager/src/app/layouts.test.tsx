// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import FleetLayout from "./fleet/layout";
import SetupLayout from "./setup/layout";

const pathname = vi.hoisted(() => ({ current: "/fleet" }));
vi.mock("next/navigation", () => ({
  usePathname: () => pathname.current,
  redirect: vi.fn(),
}));

afterEach(cleanup);

describe("section layouts", () => {
  it.each([
    "/fleet",
    "/fleet/clusters/team-orders/orders",
    "/fleet/management/plane-a",
  ])("a Fleet page (%s) has no Setup navigation", (path) => {
    pathname.current = path;
    render(<FleetLayout>page</FleetLayout>);
    expect(screen.getByRole("navigation", { name: "Main" })).toBeTruthy();
    expect(screen.queryByRole("navigation", { name: "Setup" })).toBeNull();
  });

  it.each(["/setup", "/setup/environments", "/setup/presets/gold"])(
    "a Setup page (%s) has the Setup navigation",
    (path) => {
      pathname.current = path;
      render(<SetupLayout>page</SetupLayout>);
      expect(screen.getByRole("navigation", { name: "Main" })).toBeTruthy();
      expect(screen.getByRole("navigation", { name: "Setup" })).toBeTruthy();
    },
  );
});
