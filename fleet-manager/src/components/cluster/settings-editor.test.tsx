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
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import type { FieldError } from "@/lib/session";
import {
  accessSection,
  type SettingsPage,
  settingsGroupOf,
  steps,
} from "../../../test/fakes/settings";
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
});
