// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StrictMode, useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import type { Step } from "@/lib/fields";
import { type FieldError, without, withValue } from "@/lib/session";
import {
  accessSection,
  type SettingsPage,
  settingsGroupOf,
  steps,
} from "../../../test/fakes/settings";
import { settingsSummary } from "./editable-settings";
import { SettingsEditor } from "./settings-editor";
import type { Process } from "./topology";

afterEach(cleanup);

const SHRINK = "Volumes cannot shrink. The operator keeps the current size.";
const EMBEDDED_NOTE =
  "Applies when Tasklist runs standalone. The exception is extraEnv and extraEnvFrom: they apply to the host now.";
const SECRETS: SecretRefs = [{ name: "orders-oidc", keys: ["secret"] }];

function renderEditor(
  host: Process | "cluster",
  process: Process | "cluster",
  page: SettingsPage = {},
  errors?: FieldError[],
) {
  const set = vi.fn();
  const reset = vi.fn();
  const { group, settings } = settingsGroupOf(host, process, page);
  render(
    <SettingsEditor
      group={group}
      settings={settings}
      secrets={SECRETS}
      errors={errors}
      set={set}
      reset={reset}
    />,
  );
  return { set, reset };
}

// settingRow returns the row of a setting: the outermost group of its name,
// because an editor can hold a group of the same name.
function settingRow(label: string): HTMLElement {
  return screen.getAllByRole("group", { name: label })[0];
}

describe("SettingsEditor", () => {
  it("a locked setting shows its reason and no editor", () => {
    renderEditor("cluster", "cluster", {
      claim: {
        plugins: [{ section: "access", registration: "gateway-access" }],
      },
      sections: [accessSection()],
    });
    const url = settingRow("externalUrl");
    expect(within(url).getByText("Filled by Access")).toBeDefined();
    expect(within(url).queryByRole("textbox")).toBeNull();
  });

  it("a number writes its claim value", async () => {
    const { set } = renderEditor("tasklist", "tasklist");
    await userEvent.type(
      within(settingRow("replicas")).getByRole("spinbutton"),
      "2",
    );
    expect(set).toHaveBeenLastCalledWith(
      steps("spec", "cluster", "tasklist", "replicas"),
      2,
    );
  });

  it("an invalid number does not reach set", async () => {
    const { set } = renderEditor("tasklist", "tasklist");
    const input = within(settingRow("replicas")).getByRole("spinbutton");
    fireEvent.change(input, { target: { value: "-1" } });
    expect(set).not.toHaveBeenCalled();
    expect(input.getAttribute("aria-invalid")).toBe("true");
    expect(
      within(settingRow("replicas")).getByText("Enter 0 or more"),
    ).toBeDefined();
  });

  it("YAML that fails its schema does not reach set", async () => {
    const { set } = renderEditor("zeebe", "zeebe");
    const yaml = within(settingRow("scheduling.nodeAffinity")).getByRole(
      "textbox",
    );
    fireEvent.change(yaml, {
      target: { value: "requiredDuringSchedulingIgnoredDuringExecution: 3" },
    });
    expect(set).not.toHaveBeenCalled();
  });

  it("an emptied map resets the field", async () => {
    const { set, reset } = renderEditor("zeebe", "zeebe", {
      claim: { cluster: { zeebe: { podLabels: { team: "a" } } } },
    });
    await userEvent.click(
      within(settingRow("podLabels")).getByRole("button", {
        name: "Remove entry 1",
      }),
    );
    expect(reset).toHaveBeenLastCalledWith(
      steps("spec", "cluster", "zeebe", "podLabels"),
    );
    expect(set).not.toHaveBeenCalled();
  });

  it("offers no empty map over a map of the preset, which a claim map cannot hide", async () => {
    renderEditor("zeebe", "zeebe", {
      preset: { zeebe: { podLabels: { team: "a" } } },
    });
    const row = settingRow("podLabels");
    await userEvent.click(
      within(row).getByRole("button", { name: "Remove entry 1" }),
    );
    expect(
      within(row).queryByRole("button", { name: "Set to an empty map" }),
    ).toBeNull();
  });

  it("Reset to preset resets the claim value", async () => {
    const { reset } = renderEditor("zeebe", "zeebe", {
      preset: { zeebe: { replicas: 3 } },
      claim: { cluster: { zeebe: { replicas: 5 } } },
    });
    const replicas = settingRow("replicas");
    expect(within(replicas).getByText("Changed in the claim")).toBeDefined();
    await userEvent.click(
      within(replicas).getByRole("button", { name: "Reset to preset" }),
    );
    expect(reset).toHaveBeenCalledWith(
      steps("spec", "cluster", "zeebe", "replicas"),
    );
  });

  it("a preset value has no reset", () => {
    renderEditor("zeebe", "zeebe", { preset: { zeebe: { replicas: 3 } } });
    const replicas = settingRow("replicas");
    expect(within(replicas).getByText("From preset")).toBeDefined();
    expect(
      within(replicas).queryByRole("button", { name: "Reset to preset" }),
    ).toBeNull();
  });

  it("an embedded group notes where its settings apply", () => {
    renderEditor("gateway", "tasklist", {
      cluster: { gateway: { mode: "Standalone" } },
    });
    expect(screen.getByText(EMBEDDED_NOTE)).toBeDefined();
  });

  it("a smaller storage size says volumes cannot shrink", async () => {
    renderEditor("zeebe", "zeebe", {
      claim: { cluster: { zeebe: { storageSize: "32Gi" } } },
    });
    const amount = within(settingRow("storageSize")).getByRole("textbox", {
      name: "Amount",
    });
    await userEvent.clear(amount);
    await userEvent.type(amount, "16");
    expect(screen.getByText(SHRINK)).toBeDefined();
  });

  it("a smaller storage size than a number says volumes cannot shrink", async () => {
    renderEditor("zeebe", "zeebe", {
      claim: { cluster: { zeebe: { storageSize: 34359738368 } } },
    });
    const amount = within(settingRow("storageSize")).getByRole("textbox", {
      name: "Amount",
    });
    await userEvent.clear(amount);
    await userEvent.type(amount, "1000");
    expect(screen.getByText(SHRINK)).toBeDefined();
  });

  it("a plain amount keeps a storage size that is a number a number", async () => {
    const { set } = renderEditor("zeebe", "zeebe", {
      claim: { cluster: { zeebe: { storageSize: 34359738368 } } },
    });
    const amount = within(settingRow("storageSize")).getByRole("textbox", {
      name: "Amount",
    });
    await userEvent.clear(amount);
    await userEvent.type(amount, "68719476736");
    expect(set).toHaveBeenLastCalledWith(
      steps("spec", "cluster", "zeebe", "storageSize"),
      68719476736,
    );
  });

  it("a whole amount typed after a clear keeps a storage size that is a number a number", async () => {
    const set = vi.fn();
    const reset = vi.fn();
    const preset = { zeebe: { storageSize: "10Gi" } };
    const editor = (page: SettingsPage) => {
      const { group, settings } = settingsGroupOf("zeebe", "zeebe", page);
      return (
        <SettingsEditor
          group={group}
          settings={settings}
          secrets={SECRETS}
          set={set}
          reset={reset}
        />
      );
    };
    const { rerender } = render(
      editor({
        preset,
        claim: { cluster: { zeebe: { storageSize: 34359738368 } } },
      }),
    );
    const amount = () =>
      within(settingRow("storageSize")).getByRole("textbox", {
        name: "Amount",
      });
    await userEvent.clear(amount());
    expect(reset).toHaveBeenLastCalledWith(
      steps("spec", "cluster", "zeebe", "storageSize"),
    );
    rerender(editor({ preset }));
    await userEvent.type(amount(), "68719476736");
    expect(set).toHaveBeenLastCalledWith(
      steps("spec", "cluster", "zeebe", "storageSize"),
      68719476736,
    );
  });

  it("an amount with a unit writes a storage size that is a number as a string", async () => {
    const { set } = renderEditor("zeebe", "zeebe", {
      claim: { cluster: { zeebe: { storageSize: 34359738368 } } },
    });
    const row = within(settingRow("storageSize"));
    await userEvent.clear(row.getByRole("textbox", { name: "Amount" }));
    await userEvent.type(row.getByRole("textbox", { name: "Amount" }), "64");
    await userEvent.selectOptions(
      row.getByRole("combobox", { name: "Unit" }),
      "Gi",
    );
    expect(set).toHaveBeenLastCalledWith(
      steps("spec", "cluster", "zeebe", "storageSize"),
      "64Gi",
    );
  });

  it("a Secret reference is picked by name and key", async () => {
    const { set } = renderEditor("cluster", "cluster");
    const ref = settingRow("auth.clientSecretRef");
    expect(within(ref).queryByRole("textbox")).toBeNull();
    await userEvent.selectOptions(
      within(ref).getByRole("combobox", { name: "Secret" }),
      "orders-oidc",
    );
    await userEvent.selectOptions(
      within(ref).getByRole("combobox", { name: "Key" }),
      "secret",
    );
    expect(set).toHaveBeenLastCalledWith(
      steps("spec", "cluster", "auth", "clientSecretRef"),
      { name: "orders-oidc", key: "secret" },
    );
  });

  it("a Secret reference survives a Secret list that is empty or late", () => {
    const set = vi.fn();
    const reset = vi.fn();
    const ref = { name: "orders-oidc", key: "secret" };
    const { group, settings } = settingsGroupOf("cluster", "cluster", {
      claim: { cluster: { auth: { clientSecretRef: ref } } },
    });
    const editor = (secrets: SecretRefs) => (
      <SettingsEditor
        group={group}
        settings={settings}
        secrets={secrets}
        set={set}
        reset={reset}
      />
    );
    const { rerender } = render(editor([]));
    rerender(editor(SECRETS));
    expect(
      within(settingRow("auth.clientSecretRef")).getByRole("combobox", {
        name: "Secret",
      }),
    ).toHaveProperty("value", "orders-oidc");
    expect(set).not.toHaveBeenCalled();
    expect(reset).not.toHaveBeenCalled();
  });

  it("a late Secret list does not write the variables of a Secret", () => {
    const set = vi.fn();
    const reset = vi.fn();
    const { group, settings } = settingsGroupOf("zeebe", "zeebe", {
      preset: { zeebe: { extraEnv: [{ name: "LOG_LEVEL", value: "info" }] } },
      claim: {
        cluster: {
          zeebe: {
            extraEnv: [
              {
                name: "CLIENT_SECRET",
                valueFrom: {
                  secretKeyRef: { name: "orders-oidc", key: "secret" },
                },
              },
            ],
          },
        },
      },
    });
    const editor = (secrets: SecretRefs) => (
      <SettingsEditor
        group={group}
        settings={settings}
        secrets={secrets}
        set={set}
        reset={reset}
      />
    );
    const { rerender } = render(editor([]));
    rerender(editor(SECRETS));
    expect(set).not.toHaveBeenCalled();
    expect(reset).not.toHaveBeenCalled();
  });

  it("a variable from a Secret writes its reference", async () => {
    const { set } = renderEditor("zeebe", "zeebe");
    const env = settingRow("extraEnv");
    await userEvent.click(
      within(env).getByRole("button", { name: "Add a variable" }),
    );
    await userEvent.type(
      within(env).getByRole("textbox", { name: "Name" }),
      "S",
    );
    await userEvent.selectOptions(
      within(env).getByRole("combobox", { name: "Source" }),
      "secret",
    );
    await userEvent.selectOptions(
      within(env).getByRole("combobox", { name: "Secret" }),
      "orders-oidc",
    );
    await userEvent.selectOptions(
      within(env).getByRole("combobox", { name: "Key" }),
      "secret",
    );
    expect(set).toHaveBeenLastCalledWith(
      steps("spec", "cluster", "zeebe", "extraEnv"),
      [
        {
          name: "S",
          valueFrom: { secretKeyRef: { name: "orders-oidc", key: "secret" } },
        },
      ],
    );
  });

  it("a picked Secret reference survives a Secret that goes away", async () => {
    const set = vi.fn();
    const reset = vi.fn();
    const { group, settings } = settingsGroupOf("cluster", "cluster", {});
    const editor = (secrets: SecretRefs) => (
      <SettingsEditor
        group={group}
        settings={settings}
        secrets={secrets}
        set={set}
        reset={reset}
      />
    );
    const { rerender } = render(editor(SECRETS));
    const ref = settingRow("auth.clientSecretRef");
    await userEvent.selectOptions(
      within(ref).getByRole("combobox", { name: "Secret" }),
      "orders-oidc",
    );
    await userEvent.selectOptions(
      within(ref).getByRole("combobox", { name: "Key" }),
      "secret",
    );
    expect(set).toHaveBeenCalledTimes(1);
    reset.mockClear();
    rerender(editor([]));
    expect(reset).not.toHaveBeenCalled();
  });

  it("a reset inside a block names the block that it resets", async () => {
    const tolerations = [{ key: "dedicated", operator: "Exists" }];
    const { reset } = renderEditor("zeebe", "zeebe", {
      preset: { zeebe: { scheduling: { tolerations } } },
      claim: {
        cluster: {
          zeebe: {
            scheduling: { tolerations: [{ key: "spot", operator: "Exists" }] },
          },
        },
      },
    });
    const row = settingRow("scheduling.tolerations");
    expect(
      within(row).getByText(
        "Every scheduling field of the claim goes back to the preset.",
      ),
    ).toBeDefined();
    await userEvent.click(
      within(row).getByRole("button", { name: "Reset scheduling to preset" }),
    );
    expect(reset).toHaveBeenCalledWith(
      steps("spec", "cluster", "zeebe", "scheduling"),
    );
  });

  it("a list that each layer appends to says how many entries come first", () => {
    renderEditor("zeebe", "zeebe", {
      preset: { zeebe: { extraEnvFrom: [{ configMapRef: { name: "a" } }] } },
      claim: {
        cluster: { zeebe: { extraEnvFrom: [{ secretRef: { name: "b" } }] } },
      },
    });
    const row = settingRow("extraEnvFrom");
    expect(
      within(row).getByText("The preset or the release adds 1 before these."),
    ).toBeDefined();
    expect(within(row).getByRole("textbox")).toHaveProperty(
      "value",
      expect.not.stringContaining("configMapRef"),
    );
  });

  it("a refusal of the server shows at its field", () => {
    renderEditor("zeebe", "zeebe", {}, [
      {
        path: "spec.cluster.zeebe.replicas",
        message: "replicas must be odd",
      },
    ]);
    expect(
      within(settingRow("replicas")).getByText("replicas must be odd"),
    ).toBeDefined();
    expect(
      within(settingRow("partitions")).queryByText("replicas must be odd"),
    ).toBeNull();
  });

  it("a refusal of the server describes the inputs of the rows at its field", () => {
    renderEditor(
      "zeebe",
      "zeebe",
      { claim: { cluster: { zeebe: { podLabels: { team: "a" } } } } },
      [{ path: "spec.cluster.zeebe.podLabels", message: "too many labels" }],
    );
    expect(
      within(settingRow("podLabels")).getByRole("textbox", {
        name: "Key",
        description: "too many labels",
      }),
    ).toBeDefined();
  });
});

describe("SettingsEditor collapsed", () => {
  function renderClosed(page: SettingsPage = {}, errors?: FieldError[]) {
    const set = vi.fn();
    const invalid = vi.fn();
    const { group, settings } = settingsGroupOf("zeebe", "zeebe", page);
    const ui = (shown?: FieldError[]) => (
      <SettingsEditor
        collapsed
        group={group}
        settings={settings}
        secrets={SECRETS}
        errors={shown}
        set={set}
        reset={vi.fn()}
        invalid={invalid}
      />
    );
    const view = render(ui(errors));
    return {
      set,
      invalid,
      settings,
      rerender: (shown?: FieldError[]) => view.rerender(ui(shown)),
    };
  }
  const section = () =>
    screen.getByRole("region", { name: "Brokers settings" });
  // row is the button that opens and closes the group.
  const row = () =>
    within(
      screen.getByRole("heading", { level: 3, name: "Brokers" }),
    ).getByRole("button");
  const isOpen = () => row().getAttribute("aria-expanded") === "true";
  const body = () =>
    document.getElementById(row().getAttribute("aria-controls")!)!;
  const replicas = () =>
    within(settingRow("replicas")).getByRole<HTMLInputElement>("spinbutton");
  const REFUSAL: FieldError = {
    path: "spec.cluster.zeebe.replicas",
    message: "replicas must be odd",
  };
  const PRESET_BOTH =
    "The CPU limit of 1 and the CPU request of 2 come from the preset. The limit is below the request.";
  const PRESET_LIMIT =
    "The CPU limit of 1 comes from the preset and is below the CPU request of 2.";
  const cpuLimit = () =>
    within(
      within(settingRow("resources")).getByRole("group", { name: "CPU limit" }),
    ).getByRole("textbox", { name: "Amount" });

  // MergedHost holds a claim: each change goes to the claim, and the
  // settings come from the claim merged over a preset that sets the
  // resources of process. Tasklist runs embedded in a standalone gateway.
  function MergedHost({
    preset,
    invalid,
    process = "zeebe",
    onClaim,
    initial = {},
  }: {
    preset: object;
    invalid: (at: Step[], message: string | undefined) => void;
    process?: "zeebe" | "tasklist";
    onClaim?: (cluster: object) => void;
    initial?: object;
  }) {
    const [cluster, setCluster] = useState<object>(initial);
    onClaim?.(cluster);
    const tasklist = process === "tasklist";
    const { group, settings } = settingsGroupOf(
      tasklist ? "gateway" : "zeebe",
      process,
      {
        preset: { [process]: { resources: preset } } as SettingsPage["preset"],
        claim: {
          cluster: tasklist
            ? { ...cluster, gateway: { mode: "Standalone" } }
            : cluster,
        },
      },
    );
    return (
      <SettingsEditor
        collapsed
        group={group}
        settings={settings}
        secrets={SECRETS}
        set={(at, value) => setCluster((c) => withValue(c, at.slice(2), value))}
        reset={(at) => setCluster((c) => without(c, at.slice(2)))}
        invalid={invalid}
      />
    );
  }

  const amount = (label: string) =>
    within(
      within(settingRow("resources")).getByRole("group", { name: label }),
    ).getByRole("textbox", { name: "Amount" });

  it("starts closed with its rows in the page", () => {
    renderClosed();
    expect(isOpen()).toBe(false);
    expect(body().hidden).toBe(true);
    expect(
      within(body()).getByRole("spinbutton", {
        name: "replicas",
        hidden: true,
      }),
    ).toBeDefined();
  });

  it("opens and closes on a click on its row", async () => {
    renderClosed();
    await userEvent.click(row());
    expect(isOpen()).toBe(true);
    await userEvent.click(row());
    expect(isOpen()).toBe(false);
  });

  it("counts the settings changed in the claim on its row", () => {
    renderClosed({
      preset: { zeebe: { replicas: 3 } },
      claim: { cluster: { zeebe: { partitions: 6, storageSize: "10Gi" } } },
    });
    expect(
      within(section()).getByRole("button", {
        name: "Brokers 2 changed in the claim",
      }),
    ).toBe(row());
    expect(within(row()).getByText("2")).toBeDefined();
  });

  it("counts 0 on its row without a setting changed in the claim", () => {
    renderClosed({ preset: { zeebe: { replicas: 3 } } });
    expect(
      within(section()).getByRole("button", {
        name: "Brokers 0 changed in the claim",
      }),
    ).toBe(row());
  });

  it("says first in its body where the values of its settings come from", async () => {
    const { settings } = renderClosed({
      preset: { zeebe: { replicas: 3 } },
      claim: { cluster: { zeebe: { partitions: 6 } } },
    });
    await userEvent.click(row());
    expect(body().querySelector("p")?.textContent).toBe(
      settingsSummary(settings),
    );
    expect(settingsSummary(settings)).toContain("1 from the preset");
  });

  it("shows the number of settings with an error on its row in words and in the color of an error", async () => {
    renderClosed();
    await userEvent.click(row());
    fireEvent.change(replicas(), { target: { value: "-1" } });
    expect(
      within(section()).getByRole("button", {
        name: "Brokers 0 changed in the claim 1 error",
      }),
    ).toBe(row());
    const pill = within(row()).getByText("1 error");
    expect(pill.className).toMatch(/text-bad/);
  });

  it("keeps a typed value while it is closed", async () => {
    const { set } = renderClosed();
    await userEvent.click(row());
    await userEvent.type(replicas(), "2");
    await userEvent.click(row());
    await userEvent.click(row());
    expect(replicas().value).toBe("2");
    expect(set).toHaveBeenLastCalledWith(
      steps("spec", "cluster", "zeebe", "replicas"),
      2,
    );
  });

  it("opens when a refusal of the server arrives at one of its settings", () => {
    const { rerender } = renderClosed();
    expect(isOpen()).toBe(false);
    rerender([REFUSAL]);
    expect(isOpen()).toBe(true);
    expect(
      within(settingRow("replicas")).getByText("replicas must be odd"),
    ).toBeDefined();
  });

  it("starts open with a refusal of the server at one of its settings", () => {
    renderClosed({}, [REFUSAL]);
    expect(isOpen()).toBe(true);
  });

  it("stays closed for a refusal of the server at a field of another group", () => {
    renderClosed({}, [
      { path: "spec.cluster.gateway.replicas", message: "too few" },
    ]);
    expect(isOpen()).toBe(false);
    expect(row().textContent).not.toMatch(/error/);
  });

  it("says on its row that a setting has an error after the user closes it", async () => {
    const { invalid } = renderClosed();
    await userEvent.click(row());
    fireEvent.change(replicas(), { target: { value: "-1" } });
    await userEvent.click(row());
    expect(isOpen()).toBe(false);
    expect(row().textContent).toContain("1 error");
    expect(invalid).toHaveBeenLastCalledWith(
      steps("spec", "cluster", "zeebe", "replicas"),
      "replicas: Enter 0 or more",
    );
  });

  it("drops the error from its row once the value is valid", async () => {
    renderClosed();
    await userEvent.click(row());
    fireEvent.change(replicas(), { target: { value: "-1" } });
    fireEvent.change(replicas(), { target: { value: "1" } });
    expect(row().textContent).not.toMatch(/error/);
  });

  it.each([
    [
      "tolerations",
      { scheduling: { tolerations: [{ effect: "NoSchedule" }] } },
      "scheduling.tolerations",
      "Enter a key, or use the operator Exists",
    ],
    [
      "a map of labels",
      { podLabels: { "bad key!": "a" } },
      "podLabels",
      undefined,
    ],
    [
      "environment variables",
      { extraEnv: [{ name: "", value: "a" }] },
      "extraEnv",
      "Enter a name",
    ],
    [
      "a storage size",
      { storageSize: "0Gi" },
      "storageSize",
      "Enter a size above 0",
    ],
    ["a number", { replicas: -1 }, "replicas", "Enter 0 or more"],
    [
      "YAML",
      {
        scheduling: {
          nodeAffinity: { requiredDuringSchedulingIgnoredDuringExecution: 3 },
        },
      },
      "scheduling.nodeAffinity",
      undefined,
    ],
  ])(
    "opens at once with an error that the editor of %s shows for a value from below the claim, and reports it",
    (_, zeebe, label, message) => {
      const { invalid, set } = renderClosed({
        preset: { zeebe } as SettingsPage["preset"],
      });
      expect(isOpen()).toBe(true);
      // The cluster gets the value from below the claim, so its error
      // blocks the change as an error of the user does.
      expect(invalid).toHaveBeenLastCalledWith(
        steps("spec", "cluster", "zeebe", ...label.split(".")),
        expect.stringMatching(new RegExp(`^${label.replace(".", "\\.")}: `)),
      );
      expect(set).not.toHaveBeenCalled();
      expect(row().textContent).toContain("1 error");
      if (message)
        expect(within(section()).getAllByText(message)).toHaveLength(1);
    },
  );

  it("shows a limit of the preset below its request once, in the words of the merge, and leaves the report to the host", () => {
    const { invalid } = renderClosed({
      preset: {
        zeebe: { resources: { requests: { cpu: "2" }, limits: { cpu: "1" } } },
      },
    });
    expect(
      within(section()).queryByText("The limit is below the request"),
    ).toBeNull();
    expect(within(settingRow("resources")).getByRole("alert").textContent).toBe(
      PRESET_BOTH,
    );
    expect(isOpen()).toBe(true);
    expect(row().textContent).toContain("1 error");
    expect(invalid).not.toHaveBeenCalled();
  });

  it("warns about the resources of an embedded app and does not block", () => {
    const invalid = vi.fn();
    const { group, settings } = settingsGroupOf("gateway", "tasklist", {
      cluster: { gateway: { mode: "Standalone" } },
      preset: {
        tasklist: {
          resources: { requests: { cpu: "2" }, limits: { cpu: "1" } },
        },
      },
    });
    render(
      <SettingsEditor
        collapsed
        group={group}
        settings={settings}
        secrets={SECRETS}
        set={vi.fn()}
        reset={vi.fn()}
        invalid={invalid}
      />,
    );
    const tasklist = screen.getByRole("region", { name: "Tasklist settings" });
    const heading = within(
      within(tasklist).getByRole("heading", { level: 3 }),
    ).getByRole("button");
    expect(heading.getAttribute("aria-expanded")).toBe("false");
    expect(heading.textContent).not.toMatch(/error/);
    expect(invalid).not.toHaveBeenCalled();
    const resources = within(tasklist).getAllByRole("group", {
      name: "resources",
      hidden: true,
    })[0];
    expect(within(resources).queryByRole("alert", { hidden: true })).toBeNull();
    expect(resources.textContent).toContain(PRESET_BOTH);
    expect(
      within(resources).queryByText("The limit is below the request", {
        ignore: false,
      }),
    ).toBeNull();
  });

  it("counts a merged error at mount also in strict mode", () => {
    const invalid = vi.fn();
    const { group, settings } = settingsGroupOf("zeebe", "zeebe", {
      preset: {
        zeebe: { resources: { requests: { cpu: "2" }, limits: { cpu: "1" } } },
      },
    });
    render(
      <StrictMode>
        <SettingsEditor
          collapsed
          group={group}
          settings={settings}
          secrets={SECRETS}
          set={vi.fn()}
          reset={vi.fn()}
          invalid={invalid}
        />
      </StrictMode>,
    );
    expect(isOpen()).toBe(true);
    expect(row().textContent).toContain("1 error");
    expect(invalid).not.toHaveBeenCalled();
  });

  it("keeps the error when a cleared limit leaves the limit of the preset below the request", async () => {
    const invalid = vi.fn();
    const onClaim = vi.fn();
    render(
      <MergedHost
        preset={{ requests: { cpu: "2" }, limits: { cpu: "1" } }}
        invalid={invalid}
        onClaim={onClaim}
      />,
    );
    await userEvent.clear(cpuLimit());
    expect(
      within(section()).queryByText("The limit is below the request"),
    ).toBeNull();
    const alert = within(settingRow("resources")).getByRole("alert");
    expect(alert.textContent).toBe(PRESET_BOTH);
    expect(cpuLimit().getAttribute("aria-describedby")).toContain(alert.id);
    expect(row().textContent).toContain("1 error");
    expect(invalid).not.toHaveBeenCalled();
    // The claim gets no copy of the request of the preset.
    expect(onClaim).toHaveBeenLastCalledWith({});
  });

  it("names the preset as the source of the limit once the claim sets the request", async () => {
    const invalid = vi.fn();
    const onClaim = vi.fn();
    render(
      <MergedHost
        preset={{ limits: { cpu: "1" } }}
        invalid={invalid}
        onClaim={onClaim}
      />,
    );
    await userEvent.click(row());
    await userEvent.clear(cpuLimit());
    await userEvent.type(amount("CPU request"), "2");
    expect(onClaim).toHaveBeenLastCalledWith({
      zeebe: { resources: { requests: { cpu: "2" } } },
    });
    expect(within(settingRow("resources")).getByRole("alert").textContent).toBe(
      PRESET_LIMIT,
    );
    expect(row().textContent).toContain("1 error");
    expect(invalid).not.toHaveBeenCalled();
  });

  it("drops the error once the limit of the claim is not below the request", async () => {
    const invalid = vi.fn();
    render(
      <MergedHost
        preset={{ requests: { cpu: "2" }, limits: { cpu: "1" } }}
        invalid={invalid}
      />,
    );
    await userEvent.clear(cpuLimit());
    await userEvent.type(cpuLimit(), "3");
    expect(within(settingRow("resources")).queryByRole("alert")).toBeNull();
    expect(row().textContent).not.toMatch(/error/);
    expect(invalid).not.toHaveBeenCalled();
  });

  it("writes only the typed key to the claim", async () => {
    const onClaim = vi.fn();
    render(
      <MergedHost
        preset={{ requests: { cpu: "2" }, limits: { cpu: "1" } }}
        invalid={vi.fn()}
        onClaim={onClaim}
      />,
    );
    await userEvent.clear(cpuLimit());
    await userEvent.type(cpuLimit(), "3");
    expect(onClaim).toHaveBeenLastCalledWith({
      zeebe: { resources: { limits: { cpu: "3" } } },
    });
  });

  it("writes no copy of an integer quantity of the preset beside an edit", async () => {
    const onClaim = vi.fn();
    render(
      <MergedHost
        preset={{ requests: { cpu: 1 } }}
        invalid={vi.fn()}
        onClaim={onClaim}
      />,
    );
    await userEvent.click(row());
    await userEvent.type(amount("Memory request"), "2");
    expect(onClaim).toHaveBeenLastCalledWith({
      zeebe: { resources: { requests: { memory: "2Gi" } } },
    });
  });

  it("keeps an integer quantity of the claim beside an edit", async () => {
    const onClaim = vi.fn();
    render(
      <MergedHost
        preset={{}}
        initial={{ zeebe: { resources: { requests: { cpu: 2 } } } }}
        invalid={vi.fn()}
        onClaim={onClaim}
      />,
    );
    await userEvent.click(row());
    await userEvent.type(amount("Memory limit"), "4");
    expect(onClaim).toHaveBeenLastCalledWith({
      zeebe: { resources: { requests: { cpu: 2 }, limits: { memory: "4Gi" } } },
    });
  });

  it.each([
    ["a key that the claim holds", { requests: { cpu: "1" } }],
    ["a key that only the preset gives", {}],
  ])(
    "writes a set, a clear, and a clear then another value of %s",
    async (_, claimed) => {
      const onClaim = vi.fn();
      render(
        <MergedHost
          preset={{ requests: { cpu: "1" }, limits: { memory: "8Gi" } }}
          initial={{ zeebe: { resources: claimed } }}
          invalid={vi.fn()}
          onClaim={onClaim}
        />,
      );
      await userEvent.click(row());
      const request = () => amount("CPU request");
      await userEvent.type(request(), "5");
      expect(onClaim).toHaveBeenLastCalledWith({
        zeebe: { resources: { requests: { cpu: "15" } } },
      });
      await userEvent.clear(request());
      expect(onClaim).toHaveBeenLastCalledWith({});
      await userEvent.type(request(), "3");
      expect(onClaim).toHaveBeenLastCalledWith({
        zeebe: { resources: { requests: { cpu: "3" } } },
      });
      // The amount that the field showed at open gives back the claim of then.
      await userEvent.clear(request());
      await userEvent.type(request(), "1");
      expect(onClaim).toHaveBeenLastCalledWith(
        Object.keys(claimed).length > 0
          ? { zeebe: { resources: claimed } }
          : {},
      );
    },
  );

  it("lets a fix of one of two faults of the preset through, and drops its message", async () => {
    const invalid = vi.fn();
    const onClaim = vi.fn();
    render(
      <MergedHost
        preset={{
          requests: { cpu: "2", memory: "2Gi" },
          limits: { cpu: "1", memory: "1Gi" },
        }}
        invalid={invalid}
        onClaim={onClaim}
      />,
    );
    await userEvent.clear(cpuLimit());
    await userEvent.type(cpuLimit(), "3");
    expect(onClaim).toHaveBeenLastCalledWith({
      zeebe: { resources: { limits: { cpu: "3" } } },
    });
    expect(
      within(settingRow("resources"))
        .getAllByRole("alert")
        .map((a) => a.textContent),
    ).toEqual([
      "The memory limit of 1Gi and the memory request of 2Gi come from the preset. The limit is below the request.",
    ]);
  });

  it.each(["1.0", "1000m"])(
    "takes a typed limit of %s that equals the limit of the preset as the same fault",
    async (typed) => {
      const onClaim = vi.fn();
      render(
        <MergedHost
          preset={{ requests: { cpu: "2" }, limits: { cpu: "1" } }}
          invalid={vi.fn()}
          onClaim={onClaim}
        />,
      );
      await userEvent.clear(cpuLimit());
      const millicores = typed.endsWith("m");
      // The unit comes first, so that no amount on the way is valid.
      if (millicores)
        await userEvent.selectOptions(
          within(
            within(settingRow("resources")).getByRole("group", {
              name: "CPU limit",
            }),
          ).getByRole("combobox", { name: "Unit" }),
          "m",
        );
      await userEvent.type(cpuLimit(), millicores ? typed.slice(0, -1) : typed);
      expect(
        within(section()).queryByText("The limit is below the request"),
      ).toBeNull();
      expect(onClaim).toHaveBeenLastCalledWith({
        zeebe: { resources: { limits: { cpu: typed } } },
      });
      expect(
        within(settingRow("resources")).getByRole("alert").textContent,
      ).toBe(
        `The CPU request of 2 comes from the preset and is above the CPU limit of ${typed}.`,
      );
    },
  );

  async function openTasklist() {
    await userEvent.click(
      within(
        within(
          screen.getByRole("region", { name: "Tasklist settings" }),
        ).getByRole("heading", { level: 3 }),
      ).getByRole("button"),
    );
  }

  it("lets an edit beside a fault of the preset in an embedded group reach the claim", async () => {
    const invalid = vi.fn();
    const onClaim = vi.fn();
    render(
      <MergedHost
        process="tasklist"
        preset={{ requests: { cpu: "2" }, limits: { cpu: "1" } }}
        invalid={invalid}
        onClaim={onClaim}
      />,
    );
    await openTasklist();
    await userEvent.type(amount("Memory request"), "2");
    expect(onClaim).toHaveBeenLastCalledWith({
      tasklist: { resources: { requests: { memory: "2Gi" } } },
    });
    expect(invalid).not.toHaveBeenCalled();
  });

  it("warns about a typed limit below its request in an embedded group", async () => {
    const invalid = vi.fn();
    const onClaim = vi.fn();
    render(
      <MergedHost
        process="tasklist"
        preset={{ requests: { cpu: "2" }, limits: { cpu: "3" } }}
        invalid={invalid}
        onClaim={onClaim}
      />,
    );
    await openTasklist();
    await userEvent.clear(amount("CPU limit"));
    await userEvent.type(amount("CPU limit"), "1");
    expect(onClaim).toHaveBeenLastCalledWith({
      tasklist: { resources: { limits: { cpu: "1" } } },
    });
    expect(invalid).not.toHaveBeenCalled();
    const resources = settingRow("resources");
    expect(within(resources).queryByRole("alert")).toBeNull();
    expect(resources.textContent).toContain(
      "The CPU request of 2 comes from the preset and is above the CPU limit of 1. Correct it before Tasklist runs standalone.",
    );
  });

  it("still blocks a typed amount that is not a number in an embedded group", async () => {
    const invalid = vi.fn();
    render(<MergedHost process="tasklist" preset={{}} invalid={invalid} />);
    await openTasklist();
    await userEvent.type(amount("CPU limit"), "x");
    expect(invalid).toHaveBeenLastCalledWith(
      steps("spec", "cluster", "tasklist", "resources"),
      "resources: Enter a number",
    );
  });

  it("shows no merged error for a valid preset and a request of the claim below its limit", async () => {
    const invalid = vi.fn();
    render(<MergedHost preset={{ limits: { cpu: "4" } }} invalid={invalid} />);
    await userEvent.click(row());
    const request = within(settingRow("resources")).getByRole("group", {
      name: "CPU request",
    });
    await userEvent.type(
      within(request).getByRole("textbox", { name: "Amount" }),
      "2",
    );
    expect(within(settingRow("resources")).queryByRole("alert")).toBeNull();
    expect(invalid).not.toHaveBeenCalled();
  });

  it("stays closed after the user closes it while its count of errors stays the same", async () => {
    const { rerender } = renderClosed({}, [REFUSAL]);
    expect(isOpen()).toBe(true);
    await userEvent.click(row());
    expect(isOpen()).toBe(false);
    rerender([{ ...REFUSAL }]);
    expect(isOpen()).toBe(false);
    expect(row().textContent).toContain("1 error");
  });

  it("counts a refusal at one field of a block that a layer sets once, at the row of that field", () => {
    renderClosed(
      {
        preset: {
          zeebe: {
            scheduling: { tolerations: [{ key: "a", operator: "Exists" }] },
          },
        },
      },
      [
        {
          path: "spec.cluster.zeebe.scheduling.tolerations",
          message: "too many tolerations",
        },
      ],
    );
    expect(row().textContent).toContain("1 error");
    expect(within(section()).getAllByText("too many tolerations")).toHaveLength(
      1,
    );
    expect(
      within(settingRow("scheduling.tolerations")).getByText(
        "too many tolerations",
      ),
    ).toBeDefined();
  });

  it("shows a refusal at a block that a layer sets at each row of the block", () => {
    renderClosed(
      {
        preset: {
          zeebe: {
            scheduling: { tolerations: [{ key: "a", operator: "Exists" }] },
          },
        },
      },
      [{ path: "spec.cluster.zeebe.scheduling", message: "bad block" }],
    );
    const shown = within(section()).getAllByText("bad block");
    expect(shown.length).toBeGreaterThan(1);
    expect(row().textContent).toContain(`${shown.length} errors`);
    expect(within(row()).getByText(`${shown.length} errors`)).toBeDefined();
  });

  it("counts a refusal of the server on its row", () => {
    renderClosed({}, [REFUSAL]);
    expect(row().textContent).toContain("1 error");
  });

  it("is not a disclosure without collapsed", () => {
    renderEditor("zeebe", "zeebe");
    expect(
      screen
        .getByRole("heading", { level: 3, name: "Brokers" })
        .querySelector("button"),
    ).toBeNull();
    expect(
      within(section()).queryByRole("button", { expanded: false }),
    ).toBeNull();
  });
});
