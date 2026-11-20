// @vitest-environment jsdom
import { act, cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { type DisclosureCount, DisclosureGroup } from "./disclosure-group";

afterEach(cleanup);

function Live({
  counts = [{ text: "0" }],
  heading,
  name,
  role,
  onToggle = vi.fn(),
}: {
  counts?: readonly DisclosureCount[];
  heading?: boolean;
  name?: string;
  role?: "group" | "region";
  onToggle?: (open: boolean) => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <DisclosureGroup
      label="Brokers"
      name={name}
      role={role}
      heading={heading}
      counts={counts}
      open={open}
      onToggle={(next) => {
        onToggle(next);
        setOpen(next);
      }}
    >
      <input aria-label="replicas" />
    </DisclosureGroup>
  );
}

const group = () => screen.getByRole("group", { name: "Brokers" });
const toggle = () => within(group()).getByRole("button");

describe("DisclosureGroup", () => {
  it("is a group named label whose body is hidden while it is closed", () => {
    render(<Live />);
    expect(toggle().getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("textbox", { name: "replicas" })).toBeNull();
    const body = document.getElementById(
      toggle().getAttribute("aria-controls")!,
    );
    expect(body?.getAttribute("hidden")).toBe("until-found");
    expect(body?.contains(screen.getByRole("textbox", { hidden: true }))).toBe(
      true,
    );
  });

  it("opens when the find function of the browser matches text in its closed body", () => {
    const onToggle = vi.fn();
    render(<Live onToggle={onToggle} />);
    const body = document.getElementById(
      toggle().getAttribute("aria-controls")!,
    )!;
    act(() => {
      body.dispatchEvent(new Event("beforematch"));
    });
    expect(onToggle).toHaveBeenLastCalledWith(true);
    expect(toggle().getAttribute("aria-expanded")).toBe("true");
    expect(body.hasAttribute("hidden")).toBe(false);
  });

  it("ignores a match of the find function in a closed element inside its open body", async () => {
    const onToggle = vi.fn();
    render(<Live onToggle={onToggle} />);
    await userEvent.click(toggle());
    onToggle.mockClear();
    act(() => {
      screen
        .getByRole("textbox", { name: "replicas" })
        .dispatchEvent(new Event("beforematch", { bubbles: true }));
    });
    expect(onToggle).not.toHaveBeenCalled();
    expect(toggle().getAttribute("aria-expanded")).toBe("true");
  });

  it("hides its body again for the find function once it closes", async () => {
    render(<Live />);
    await userEvent.click(toggle());
    await userEvent.click(toggle());
    expect(
      document
        .getElementById(toggle().getAttribute("aria-controls")!)
        ?.getAttribute("hidden"),
    ).toBe("until-found");
  });

  it("asks onToggle for the other state on a click on its row", async () => {
    const onToggle = vi.fn();
    render(<Live onToggle={onToggle} />);
    await userEvent.click(toggle());
    expect(onToggle).toHaveBeenLastCalledWith(true);
    expect(toggle().getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByRole("textbox", { name: "replicas" })).toBeTruthy();
    await userEvent.click(toggle());
    expect(onToggle).toHaveBeenLastCalledWith(false);
    expect(screen.queryByRole("textbox", { name: "replicas" })).toBeNull();
  });

  it("names its row with the label and the text of each count", () => {
    render(<Live counts={[{ text: "3" }]} />);
    expect(toggle().textContent).toBe("Brokers 3");
    expect(screen.getByRole("button", { name: "Brokers 3" })).toBeTruthy();
  });

  it("reads the label of a count in place of its text", () => {
    render(
      <Live
        counts={[
          { text: "1", label: "1 changed in the claim" },
          { text: "2 errors", bad: true },
        ]}
      />,
    );
    expect(
      screen.getByRole("button", {
        name: "Brokers 1 changed in the claim 2 errors",
      }),
    ).toBeTruthy();
  });

  it("gives a bad count the color of an error and the others none", () => {
    render(
      <Live
        counts={[
          { text: "1", label: "1 changed in the claim" },
          { text: "1 error", bad: true },
        ]}
      />,
    );
    const pills = toggle().querySelectorAll("[data-count]");
    expect(pills).toHaveLength(2);
    expect(pills[0].className).not.toMatch(/text-bad/);
    expect(pills[1].className).toMatch(/text-bad/);
    expect(pills[1].textContent).toContain("1 error");
  });

  it("puts its row in a heading of level 3 named by the label alone with heading", () => {
    render(<Live heading counts={[{ text: "1", label: "1 changed" }]} />);
    const title = screen.getByRole("heading", { level: 3, name: "Brokers" });
    expect(title.contains(toggle())).toBe(true);
  });

  it("has no heading without heading", () => {
    render(<Live />);
    expect(screen.queryByRole("heading")).toBeNull();
  });

  it("takes the role and the name that the caller gives", () => {
    render(<Live role="region" name="Brokers settings" />);
    const region = screen.getByRole("region", { name: "Brokers settings" });
    expect(
      within(region).getByRole("button", { name: /^Brokers/ }),
    ).toBeTruthy();
    expect(screen.queryByRole("group", { name: "Brokers" })).toBeNull();
  });
});
