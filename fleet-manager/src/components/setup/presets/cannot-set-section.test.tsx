// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { CannotSetSection } from "./cannot-set-section";

afterEach(cleanup);

describe("CannotSetSection", () => {
  it("lists the fields that a preset cannot set", () => {
    render(<CannotSetSection kind="DatabaseServerPreset" />);
    const list = screen.getByRole("list", {
      name: "Fields a preset cannot set",
    });
    expect(
      within(list)
        .getAllByRole("listitem")
        .map((i) => i.textContent),
    ).toEqual([
      "presetRef",
      "releaseRef",
      "databaseServerConfig",
      "suspend",
      "version",
    ]);
  });
});
