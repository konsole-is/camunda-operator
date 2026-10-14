// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "./tabs";

afterEach(cleanup);

function renderTabs(orientation?: "horizontal" | "vertical") {
  render(
    <Tabs defaultValue="one" orientation={orientation}>
      <TabsList>
        <TabsTrigger value="one">One</TabsTrigger>
        <TabsTrigger value="two">Two</TabsTrigger>
      </TabsList>
      <TabsContent value="one">First</TabsContent>
      <TabsContent value="two">Second</TabsContent>
    </Tabs>,
  );
}

describe("Tabs", () => {
  it("is horizontal when the caller gives no orientation", () => {
    renderTabs();
    expect(screen.getByRole("tablist").getAttribute("aria-orientation")).toBe(
      "horizontal",
    );
  });

  it("tells assistive technology that vertical tabs are vertical", () => {
    renderTabs("vertical");
    expect(screen.getByRole("tablist").getAttribute("aria-orientation")).toBe(
      "vertical",
    );
  });

  it("moves between vertical tabs with the down arrow key", async () => {
    renderTabs("vertical");
    const first = screen.getByRole("tab", { name: "One" });
    first.focus();
    fireEvent.keyDown(first, { key: "ArrowDown" });
    await waitFor(() =>
      expect(document.activeElement).toBe(
        screen.getByRole("tab", { name: "Two" }),
      ),
    );
  });
});
