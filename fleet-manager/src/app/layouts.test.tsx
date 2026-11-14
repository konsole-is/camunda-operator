// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render as renderPage, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { ThemeProvider } from "@/components/fleet/theme-switch";
import { THEME_COOKIE } from "@/lib/theme";
import FleetLayout from "./fleet/layout";
import RootLayout from "./layout";
import SetupLayout from "./setup/layout";

const pathname = vi.hoisted(() => ({ current: "/fleet" }));
vi.mock("next/navigation", () => ({
  usePathname: () => pathname.current,
  redirect: vi.fn(),
}));

const cookie = vi.hoisted(() => ({ value: undefined as string | undefined }));
vi.mock("next/headers", () => ({
  cookies: async () => ({
    get: (name: string) =>
      name === THEME_COOKIE && cookie.value !== undefined
        ? { name, value: cookie.value }
        : undefined,
  }),
}));

afterEach(cleanup);

// The root layout gives each section layout these providers.
const render = (page: ReactElement) =>
  renderPage(
    <ThemeProvider initial="system">
      <QueryClientProvider client={new QueryClient()}>
        <GitPushesProvider>{page}</GitPushesProvider>
      </QueryClientProvider>
    </ThemeProvider>,
  );

describe("root layout", () => {
  it.each([
    [undefined, undefined],
    ["light", "light"],
    ["dark", "dark"],
    ["purple", undefined],
  ])(
    "renders the theme cookie %j as the class %j of <html>",
    async (value, className) => {
      cookie.value = value;
      const html = (await RootLayout({ children: "page" })) as ReactElement<{
        className?: string;
      }>;
      expect(html.type).toBe("html");
      expect(html.props.className).toBe(className);
    },
  );
});

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

  it("a section layout has the theme switch", () => {
    pathname.current = "/fleet";
    render(<FleetLayout>page</FleetLayout>);
    expect(screen.getByRole("group", { name: "Theme" })).toBeTruthy();
  });

  it("a section layout shows no banner while nothing waits", () => {
    pathname.current = "/fleet";
    render(<FleetLayout>page</FleetLayout>);
    render(<SetupLayout>page</SetupLayout>);
    expect(screen.queryByText("Waiting for GitOps to apply")).toBeNull();
  });
});
