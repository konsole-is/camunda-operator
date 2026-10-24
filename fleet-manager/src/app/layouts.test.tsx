// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render as renderPage, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import FleetLayout from "./fleet/layout";
import SetupLayout from "./setup/layout";

const pathname = vi.hoisted(() => ({ current: "/fleet" }));
vi.mock("next/navigation", () => ({
  usePathname: () => pathname.current,
  redirect: vi.fn(),
}));

afterEach(cleanup);

// The root layout gives each section layout these providers.
const render = (page: ReactElement) =>
  renderPage(
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>{page}</GitPushesProvider>
    </QueryClientProvider>,
  );

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

  it("a section layout shows no banner while nothing waits", () => {
    pathname.current = "/fleet";
    render(<FleetLayout>page</FleetLayout>);
    render(<SetupLayout>page</SetupLayout>);
    expect(screen.queryByText("Waiting for GitOps to apply")).toBeNull();
  });
});
