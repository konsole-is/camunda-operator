import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { THEME_COOKIE, type ThemeChoice, themeCookie } from "@/lib/theme";
import { ThemeProvider, ThemeSwitch } from "./theme-switch";

afterEach(() => {
  cleanup();
  document.documentElement.className = "";
  document.cookie = themeCookie("system");
  vi.restoreAllMocks();
});

function renderSwitch(initial: ThemeChoice) {
  render(
    <ThemeProvider initial={initial}>
      <ThemeSwitch />
    </ThemeProvider>,
  );
}

function pressed() {
  const group = screen.getByRole("group", { name: "Theme" });
  return [...group.querySelectorAll('[aria-pressed="true"]')].map((b) =>
    b.getAttribute("aria-label"),
  );
}

describe("ThemeSwitch", () => {
  it("offers System, Light, and Dark", () => {
    renderSwitch("system");
    const group = screen.getByRole("group", { name: "Theme" });
    expect(
      [...group.querySelectorAll("button")].map((b) =>
        b.getAttribute("aria-label"),
      ),
    ).toEqual(["System", "Light", "Dark"]);
  });

  it.each([
    ["system", "System"],
    ["light", "Light"],
    ["dark", "Dark"],
  ] as const)("marks only the initial choice %s", (initial, label) => {
    renderSwitch(initial);
    expect(pressed()).toEqual([label]);
  });

  it("changes the class of <html> and keeps the choice in the cookie", () => {
    renderSwitch("system");
    fireEvent.click(screen.getByRole("button", { name: "Light" }));
    expect([...document.documentElement.classList]).toEqual(["light"]);
    expect(document.cookie).toContain(`${THEME_COOKIE}=light`);
    expect(pressed()).toEqual(["Light"]);

    fireEvent.click(screen.getByRole("button", { name: "Dark" }));
    expect([...document.documentElement.classList]).toEqual(["dark"]);
    expect(document.cookie).toContain(`${THEME_COOKIE}=dark`);
    expect(pressed()).toEqual(["Dark"]);
  });

  it("goes back to the setting of the system with no class and no cookie", () => {
    document.documentElement.className = "dark";
    document.cookie = themeCookie("dark");
    renderSwitch("dark");
    fireEvent.click(screen.getByRole("button", { name: "System" }));
    expect([...document.documentElement.classList]).toEqual([]);
    expect(document.cookie).not.toContain(THEME_COOKIE);
    expect(pressed()).toEqual(["System"]);
  });

  it("throws outside a ThemeProvider", () => {
    // React logs the error of a failed render before it throws it.
    vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => render(<ThemeSwitch />)).toThrow(/ThemeProvider/);
  });
});
