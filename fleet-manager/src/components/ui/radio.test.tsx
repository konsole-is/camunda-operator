// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import { Radio } from "./radio";

afterEach(cleanup);

const radio = (name: string) =>
  screen.getByRole("radio", { name }) as HTMLInputElement;

function group() {
  render(
    <fieldset>
      <label>
        <Radio name="deletion" value="keep" defaultChecked /> Keep
      </label>
      <label>
        <Radio name="deletion" value="delete" /> Delete
      </label>
    </fieldset>,
  );
}

describe("Radio", () => {
  it("is a radio button that its label names", () => {
    group();
    expect(radio("Keep").checked).toBe(true);
    expect(radio("Delete").checked).toBe(false);
  });

  it("picks one radio of its group at a time", async () => {
    group();
    await userEvent.click(screen.getByText("Delete"));
    expect(radio("Delete").checked).toBe(true);
    expect(radio("Keep").checked).toBe(false);
  });
});
