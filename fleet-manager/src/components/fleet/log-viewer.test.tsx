// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import type { LogLine } from "@/lib/observe/types";
import { LogViewer } from "./log-viewer";

afterEach(cleanup);

describe("LogViewer", () => {
  const line = (message: string, level = "info"): LogLine => ({
    raw: message,
    message,
    level,
    time: "2026-10-15T11:59:00Z",
    fields: {},
  });

  it("keeps the shown lines while paused and counts the new ones", async () => {
    const { rerender } = render(<LogViewer lines={[line("a"), line("b")]} />);
    await userEvent.click(screen.getByRole("button", { name: "Pause" }));
    rerender(
      <LogViewer lines={[line("a"), line("b"), line("c"), line("d")]} />,
    );
    expect(screen.queryByText("c")).toBeNull();
    expect(screen.getByText("2 new lines")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Follow" }));
    expect(screen.getByText("d")).toBeTruthy();
  });

  it("keeps the shown lines while paused when the buffer drops old lines", async () => {
    const { rerender } = render(<LogViewer lines={[line("a"), line("b")]} />);
    await userEvent.click(screen.getByRole("button", { name: "Pause" }));
    rerender(<LogViewer lines={[line("b"), line("c")]} />);
    expect(screen.getByText("a")).toBeTruthy();
    expect(screen.queryByText("c")).toBeNull();
    expect(screen.getByText("1 new line")).toBeTruthy();
  });

  it("counts a new line that repeats a shown line", async () => {
    const shown = line("retrying");
    const { rerender } = render(<LogViewer lines={[shown]} />);
    await userEvent.click(screen.getByRole("button", { name: "Pause" }));
    rerender(<LogViewer lines={[shown, line("retrying")]} />);
    expect(screen.getByText("1 new line")).toBeTruthy();
  });

  it("keeps the level of the paused lines in the level list", async () => {
    const warn = line("slow disk", "warn");
    const { rerender } = render(<LogViewer lines={[warn]} />);
    await userEvent.click(screen.getByRole("button", { name: "Pause" }));
    rerender(<LogViewer lines={[line("started")]} />);
    expect(screen.getByRole("option", { name: "warn" })).toBeTruthy();
  });

  it("keeps an opened line open on that line when the buffer moves", async () => {
    const withFields = (message: string): LogLine => ({
      ...line(message),
      fields: { pod: message },
    });
    const a = withFields("a");
    const b = withFields("b");
    const { rerender } = render(<LogViewer lines={[a, b]} />);
    await userEvent.click(screen.getByText("a"));
    expect(screen.getByText("a").closest("details")?.open).toBe(true);
    rerender(<LogViewer lines={[b, withFields("c")]} />);
    expect(screen.getByText("b").closest("details")?.open).toBe(false);
  });

  it("takes the keyboard focus, so a keyboard user can scroll it", () => {
    render(<LogViewer lines={[line("a")]} />);
    const log = screen.getByRole("log");
    log.focus();
    expect(document.activeElement).toBe(log);
  });

  it("filters by level and by text", async () => {
    render(
      <LogViewer
        lines={[
          line("started"),
          line("disk full", "error"),
          line("retry", "warn"),
        ]}
      />,
    );
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Level" }),
      "error",
    );
    expect(screen.queryByText("started")).toBeNull();
    expect(screen.getByText("disk full")).toBeTruthy();
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Level" }),
      "",
    );
    await userEvent.type(
      screen.getByRole("searchbox", { name: "Filter" }),
      "retr",
    );
    expect(screen.getByText("retry")).toBeTruthy();
    expect(screen.queryByText("disk full")).toBeNull();
  });
});
