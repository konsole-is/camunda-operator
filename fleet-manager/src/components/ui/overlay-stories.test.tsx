// @vitest-environment jsdom
import type { ReactRenderer } from "@storybook/nextjs-vite";
import { composeStory } from "storybook/preview-api";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import * as dialogStories from "./dialog.stories";
import * as popoverStories from "./popover.stories";
import * as tooltipStories from "./tooltip.stories";

afterEach(cleanup);

// jsdom has no ResizeObserver, and the Radix tooltip measures its trigger.
globalThis.ResizeObserver ??= class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

// The composeStory of @storybook/nextjs-vite needs the Vite plugin of
// Storybook, which Vitest does not load. The stories need no Next.js mock.
const DialogOpen = composeStory<ReactRenderer>(
  dialogStories.Open,
  dialogStories.default,
);
const PopoverOpen = composeStory<ReactRenderer>(
  popoverStories.Open,
  popoverStories.default,
);
const TooltipOpen = composeStory<ReactRenderer>(
  tooltipStories.Open,
  tooltipStories.default,
);

describe("the overlay stories", () => {
  it("open the Dialog, and Escape closes it", async () => {
    render(<DialogOpen />);
    expect(screen.getByRole("dialog")).toBeDefined();
    fireEvent.keyDown(document.activeElement ?? document.body, {
      key: "Escape",
    });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("open the Popover, and Escape closes it", async () => {
    render(<PopoverOpen />);
    expect(screen.getByRole("dialog", { name: "Filters" })).toBeDefined();
    fireEvent.keyDown(document.activeElement ?? document.body, {
      key: "Escape",
    });
    await waitFor(() =>
      expect(screen.queryByRole("dialog", { name: "Filters" })).toBeNull(),
    );
  });

  it("open the Tooltip, and Escape closes it", async () => {
    render(<TooltipOpen />);
    expect(screen.getAllByRole("tooltip").length).toBeGreaterThan(0);
    fireEvent.keyDown(document.activeElement ?? document.body, {
      key: "Escape",
    });
    await waitFor(() => expect(screen.queryByRole("tooltip")).toBeNull());
  });
});
