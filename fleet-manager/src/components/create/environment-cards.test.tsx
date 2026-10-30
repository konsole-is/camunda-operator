// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import { EnvironmentCards, environmentReady } from "./environment-cards";

afterEach(cleanup);

function environment(
  name: string,
  over: {
    displayName?: string;
    color?: string;
    ready?: { status: "True" | "False" | "Unknown"; message: string };
  } = {},
): Environment {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name },
    spec: {
      ...(over.displayName && { displayName: over.displayName }),
      ...(over.color && { color: over.color }),
    },
    ...(over.ready && {
      status: {
        conditions: [
          {
            type: "Ready",
            status: over.ready.status,
            reason: "Test",
            message: over.ready.message,
            lastTransitionTime: "2026-10-30T00:00:00Z",
          },
        ],
      },
    }),
  };
}

const prod = environment("prod", {
  displayName: "Production",
  color: "#2E7D32",
  ready: { status: "True", message: "" },
});
const dev = environment("dev");

describe("EnvironmentCards", () => {
  it("shows the name, the color, and the counts of each environment", () => {
    render(
      <EnvironmentCards
        environments={[prod, dev]}
        counts={{
          prod: { namespaces: 3, clusters: 2 },
          dev: { namespaces: 1, clusters: 0 },
        }}
      />,
    );
    const group = screen.getByRole("radiogroup", { name: "Environment" });
    const radios = within(group).getAllByRole("radio");
    expect(radios.map((r) => r.closest("label")?.textContent)).toEqual([
      expect.stringContaining("dev"),
      expect.stringContaining("Production"),
    ]);
    expect(screen.getByText("1 namespace · 0 clusters")).toBeTruthy();
    expect(screen.getByText("3 namespaces · 2 clusters")).toBeTruthy();
    const swatches = document.querySelectorAll("[data-swatch]");
    expect(swatches).toHaveLength(1);
    expect((swatches[0] as HTMLElement).style.backgroundColor).toBe(
      "rgb(46, 125, 50)",
    );
  });

  it("shows no swatch for a color that is not a hex color", () => {
    render(
      <EnvironmentCards
        environments={[environment("x", { color: "red; content: ''" })]}
        counts={{}}
      />,
    );
    expect(document.querySelectorAll("[data-swatch]")).toHaveLength(0);
  });

  it("picks an environment on a click", async () => {
    const onChange = vi.fn();
    render(
      <EnvironmentCards
        environments={[prod, dev]}
        counts={{}}
        onChange={onChange}
      />,
    );
    await userEvent.click(screen.getByRole("radio", { name: /Production/ }));
    expect(onChange).toHaveBeenCalledWith("prod");
  });

  it("marks the picked environment", () => {
    render(
      <EnvironmentCards environments={[prod, dev]} counts={{}} value="prod" />,
    );
    expect(
      (screen.getByRole("radio", { name: /Production/ }) as HTMLInputElement)
        .checked,
    ).toBe(true);
  });

  it("disables an environment that is not ready and shows its message", async () => {
    const onChange = vi.fn();
    const old = environment("old", {
      ready: {
        status: "False",
        message: 'CamundaRelease "8.9-old" does not exist',
      },
    });
    render(
      <EnvironmentCards environments={[old]} counts={{}} onChange={onChange} />,
    );
    const radio = screen.getByRole("radio", { name: /old/ });
    expect(radio.getAttribute("aria-disabled")).toBe("true");
    expect(
      screen.getByText('Not ready: CamundaRelease "8.9-old" does not exist'),
    ).toBeTruthy();
    await userEvent.click(radio);
    expect(onChange).not.toHaveBeenCalled();
  });

  it("says only Not ready for a condition with no message", () => {
    const old = environment("old", { ready: { status: "False", message: "" } });
    render(<EnvironmentCards environments={[old]} counts={{}} />);
    expect(screen.getByText("Not ready")).toBeTruthy();
  });

  it("shows the extra content of each card", () => {
    render(
      <EnvironmentCards
        environments={[prod]}
        counts={{}}
        extra={(e) => <span>{`Served by plane-${e.metadata.name}`}</span>}
      />,
    );
    expect(screen.getByText("Served by plane-prod")).toBeTruthy();
  });
});

describe("EnvironmentCards in multiple mode", () => {
  it("offers a checkbox for each environment, by object name next to the display name", () => {
    render(
      <EnvironmentCards
        multiple
        environments={[prod, dev]}
        counts={{}}
        value={[]}
        onChange={vi.fn()}
      />,
    );
    const group = screen.getByRole("group", { name: "Environments" });
    expect(
      within(group)
        .getAllByRole("checkbox")
        .map((c) => c.closest("label")?.textContent),
    ).toEqual([
      expect.stringContaining("dev"),
      expect.stringMatching(/prod.*Production/),
    ]);
    expect(screen.queryByRole("radio")).toBeNull();
  });

  it("checks each environment of value", () => {
    render(
      <EnvironmentCards
        multiple
        environments={[prod, dev]}
        counts={{}}
        value={["prod"]}
        onChange={vi.fn()}
      />,
    );
    const box = (name: RegExp) =>
      screen.getByRole("checkbox", { name }) as HTMLInputElement;
    expect(box(/prod/).checked).toBe(true);
    expect(box(/dev/).checked).toBe(false);
  });

  it("adds and removes an environment on a click, in the order of the cards", async () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <EnvironmentCards
        multiple
        environments={[prod, dev]}
        counts={{}}
        value={["prod"]}
        onChange={onChange}
      />,
    );
    await userEvent.click(screen.getByRole("checkbox", { name: /dev/ }));
    expect(onChange).toHaveBeenLastCalledWith(["dev", "prod"]);
    rerender(
      <EnvironmentCards
        multiple
        environments={[prod, dev]}
        counts={{}}
        value={["dev", "prod"]}
        onChange={onChange}
      />,
    );
    await userEvent.click(screen.getByRole("checkbox", { name: /prod/ }));
    expect(onChange).toHaveBeenLastCalledWith(["dev"]);
  });

  it("disables an environment that is not ready", () => {
    const old = environment("old", {
      ready: { status: "False", message: "x" },
    });
    render(
      <EnvironmentCards
        multiple
        environments={[old]}
        counts={{}}
        value={[]}
        onChange={vi.fn()}
      />,
    );
    expect(
      (screen.getByRole("checkbox", { name: /old/ }) as HTMLInputElement)
        .disabled,
    ).toBe(true);
  });
});

describe("environmentReady", () => {
  it("is ready without a status", () => {
    expect(environmentReady(dev)).toEqual({ ready: true });
  });

  it("is ready with Ready=Unknown", () => {
    expect(
      environmentReady(
        environment("x", { ready: { status: "Unknown", message: "wait" } }),
      ),
    ).toEqual({ ready: true });
  });

  it("is not ready with Ready=False, with its message", () => {
    expect(
      environmentReady(
        environment("x", { ready: { status: "False", message: "gone" } }),
      ),
    ).toEqual({ ready: false, message: "gone" });
  });
});
