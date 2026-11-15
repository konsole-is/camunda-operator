// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { CamundaReleaseSpec } from "@/generated/crds/camundarelease";
import { EnvScopes } from "./env-scopes";

afterEach(cleanup);

function setup(spec: CamundaReleaseSpec = { version: "8.9.0" }) {
  const onChange =
    vi.fn<
      (
        update: (s: CamundaReleaseSpec) => CamundaReleaseSpec,
        errors: unknown[],
      ) => void
    >();
  render(
    <EnvScopes
      spec={spec}
      secrets={[{ name: "orders-db", keys: ["password"] }]}
      missingIn={() => []}
      onChange={onChange}
    />,
  );
  return onChange;
}

const scope = (label: string) =>
  screen
    .getByText(label, { selector: "summary *, summary" })
    .closest("details") as HTMLElement;

describe("EnvScopes", () => {
  it("offers every env scope of a release and no Optimize scope", () => {
    setup();
    const summaries = [...document.querySelectorAll("summary")].map(
      (s) => s.textContent,
    );
    expect(summaries).toEqual([
      "Every workload0",
      "Connectors0",
      "Brokers0",
      "Gateway0",
      "Operate0",
      "Tasklist0",
      "Admin0",
    ]);
  });

  it("opens the scopes that hold entries", () => {
    setup({
      version: "8.9.0",
      zeebe: { extraEnv: [{ name: "A", value: "1" }] },
    });
    expect(scope("Brokers").hasAttribute("open")).toBe(true);
    expect(scope("Gateway").hasAttribute("open")).toBe(false);
  });

  it("edits the variables of a scope without YAML", async () => {
    const onChange = setup();
    const brokers = within(scope("Brokers"));
    await userEvent.click(
      brokers.getByRole("button", { name: "Add a variable" }),
    );
    await userEvent.type(brokers.getByRole("textbox", { name: "Name" }), "X");
    await userEvent.type(brokers.getByRole("textbox", { name: "Value" }), "1");
    const [update, errors] = onChange.mock.calls.at(-1) ?? [];
    const spec = update?.({ version: "8.9.0" });
    expect(spec?.zeebe).toEqual({ extraEnv: [{ name: "X", value: "1" }] });
    expect(spec?.extraEnv).toBeUndefined();
    expect(errors).toEqual([]);
  });

  it("reports the errors of a scope with the scope in the path", async () => {
    const onChange = setup();
    const gateway = within(scope("Gateway"));
    await userEvent.click(
      gateway.getByRole("button", { name: "Add a variable" }),
    );
    const [, errors] = onChange.mock.calls.at(-1) ?? [];
    expect(errors).toEqual([
      { path: "gateway.extraEnv.0.name", message: "Enter a name" },
    ]);
  });

  it("edits extraEnvFrom as YAML only", () => {
    const onChange = setup();
    const tasklist = within(scope("Tasklist"));
    fireEvent.change(
      tasklist.getByRole("textbox", { name: "extraEnvFrom of Tasklist" }),
      { target: { value: "- configMapRef:\n    name: tuning\n" } },
    );
    const [update, errors] = onChange.mock.calls.at(-1) ?? [];
    expect(update?.({ version: "8.9.0" }).tasklist).toEqual({
      extraEnvFrom: [{ configMapRef: { name: "tuning" } }],
    });
    expect(errors).toEqual([]);
    expect(
      screen
        .getAllByRole("textbox")
        .filter((box) => box.tagName === "TEXTAREA"),
    ).toHaveLength(7);
  });
});
