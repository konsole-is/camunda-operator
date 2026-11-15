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
  within(screen.getByRole("group", { name: label }));

async function open(label: string) {
  await userEvent.click(scope(label).getByRole("button", { expanded: false }));
  return scope(label);
}

describe("EnvScopes", () => {
  it("offers every env scope of a release and no Optimize scope", () => {
    setup();
    expect(
      screen
        .getAllByRole("button", { expanded: false })
        .map((b) => b.closest("[role=group]")?.getAttribute("aria-label")),
    ).toEqual([
      "Every workload",
      "Connectors",
      "Brokers",
      "Gateway",
      "Operate",
      "Tasklist",
      "Admin",
    ]);
    expect(screen.queryByRole("group", { name: "Optimize" })).toBeNull();
  });

  it("opens the scopes that hold entries and shows their count", () => {
    setup({
      version: "8.9.0",
      zeebe: { extraEnv: [{ name: "A", value: "1" }] },
    });
    const toggle = scope("Brokers").getByRole("button", { expanded: true });
    expect(toggle.textContent).toContain("1");
    expect(
      scope("Brokers").getByRole("group", { name: "Variables of Brokers" }),
    ).toBeTruthy();
    expect(
      scope("Gateway").getByRole("button", { expanded: false }),
    ).toBeTruthy();
    expect(scope("Gateway").queryByRole("textbox")).toBeNull();
  });

  it("edits the variables of a scope without YAML", async () => {
    const onChange = setup();
    const brokers = await open("Brokers");
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
    const gateway = await open("Gateway");
    await userEvent.click(
      gateway.getByRole("button", { name: "Add a variable" }),
    );
    const [, errors] = onChange.mock.calls.at(-1) ?? [];
    expect(errors).toEqual([
      { path: "gateway.extraEnv.0.name", message: "Enter a name" },
    ]);
  });

  it("edits extraEnvFrom as YAML only", async () => {
    const onChange = setup();
    const tasklist = await open("Tasklist");
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
      tasklist
        .getAllByRole("textbox")
        .filter((box) => box.tagName === "TEXTAREA"),
    ).toHaveLength(1);
  });

  it("keeps the edits of a scope when it is closed and opened again", async () => {
    const onChange = setup();
    const operate = await open("Operate");
    await userEvent.click(
      operate.getByRole("button", { name: "Add a variable" }),
    );
    await userEvent.click(
      scope("Operate").getByRole("button", { expanded: true }),
    );
    expect(scope("Operate").queryByRole("textbox")).toBeNull();
    const [, errors] = onChange.mock.calls.at(-1) ?? [];
    expect(errors).toEqual([
      { path: "operate.extraEnv.0.name", message: "Enter a name" },
    ]);
  });
});
