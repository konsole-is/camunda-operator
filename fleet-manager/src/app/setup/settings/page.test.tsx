// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { metricsServerFound } from "@/lib/observe/kube";
import { listSecretRefs } from "@/lib/secrets";
import { appPermissions, readSettings } from "@/lib/settings";
import { saveSettings } from "./actions";
import SettingsPage, { dynamic } from "./page";

vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh: vi.fn() }) }));
vi.mock("@/lib/settings", () => ({
  readSettings: vi.fn(),
  appPermissions: vi.fn(),
}));
vi.mock("@/lib/secrets", () => ({ listSecretRefs: vi.fn() }));
vi.mock("@/lib/observe/kube", () => ({ metricsServerFound: vi.fn() }));
vi.mock("./actions", () => ({
  saveSettings: vi.fn(async () => ({ ok: true, mode: "direct" })),
  testGitConnection: vi.fn(),
  testPrometheusQuery: vi.fn(),
}));

const settings = {
  kubernetesClusterName: "prod-eu",
  "git.repository": "",
  "git.branch": "",
  "git.path": "",
  "git.credentialsSecretRef": "",
  "secretStore.kind": "",
  "secretStore.name": "",
  "prometheus.url": "",
  "prometheus.credentialsSecretRef": "prom",
};

beforeEach(() => {
  vi.clearAllMocks();
  process.env.FLEET_NAMESPACE = "fleet";
  process.env.FLEET_SERVICE_NAME = "camunda-operator-fleet-manager";
  vi.mocked(readSettings).mockResolvedValue({ settings, resourceVersion: "7" });
  vi.mocked(listSecretRefs).mockResolvedValue([
    { name: "prom", keys: ["password-key-name"] },
    { name: "git", keys: ["token-key-name"] },
  ]);
  vi.mocked(metricsServerFound).mockResolvedValue(true);
  vi.mocked(appPermissions).mockResolvedValue({
    rules: [
      {
        apiGroups: [""],
        resources: ["configmaps"],
        resourceNames: ["fleet-manager-settings"],
        verbs: ["get"],
      },
    ],
    incomplete: false,
  });
});

afterEach(cleanup);

describe("SettingsPage", () => {
  it("reads the cluster on each request", () => {
    expect(dynamic).toBe("force-dynamic");
  });

  it("renders the settings that it read with their version", async () => {
    render(await SettingsPage());
    const name = screen.getByLabelText(
      "Kubernetes cluster name",
    ) as HTMLInputElement;
    expect(name.value).toBe("prod-eu");

    fireEvent.change(name, { target: { value: "prod-us" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(saveSettings).toHaveBeenCalledWith(
        { kubernetesClusterName: "prod-us" },
        "7",
      ),
    );
  });

  it("lists the Secrets of FLEET_NAMESPACE by name", async () => {
    const { container } = render(await SettingsPage());
    expect(listSecretRefs).toHaveBeenCalledWith("fleet");
    const options = Array.from(
      (
        screen.getByLabelText(
          "Prometheus credentials Secret",
        ) as HTMLSelectElement
      ).options,
    ).map((o) => o.textContent);
    expect(options).toEqual(["None", "prom", "git"]);
    expect(container.textContent).not.toContain("key-name");
  });

  it.each([
    [true, "metrics-server found"],
    [false, "metrics-server not found"],
  ])("shows metrics-server found or not found (%s)", async (found, text) => {
    vi.mocked(metricsServerFound).mockResolvedValue(found);
    const { container } = render(await SettingsPage());
    expect(metricsServerFound).toHaveBeenCalledWith("fleet");
    expect(container.textContent).toContain(text);
  });

  it("shows the port-forward command with FLEET_SERVICE_NAME", async () => {
    const { container } = render(await SettingsPage());
    expect(container.textContent).toContain(
      "kubectl -n fleet port-forward service/camunda-operator-fleet-manager 3000:3000",
    );
  });

  it("a settings read that fails shows the error and no form", async () => {
    vi.mocked(readSettings).mockRejectedValue(
      new Error("configmaps is forbidden"),
    );
    render(await SettingsPage());
    expect(
      screen.getByText("Could not read the settings: configmaps is forbidden"),
    ).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Save" })).toBeNull();
    expect(screen.queryByLabelText("Kubernetes cluster name")).toBeNull();
  });

  it("a permissions read that fails still shows the rest of the page", async () => {
    vi.mocked(appPermissions).mockRejectedValue(new Error("forbidden"));
    const { container } = render(await SettingsPage());
    expect(
      screen.getByText("Could not read the permissions: forbidden"),
    ).toBeTruthy();
    expect(screen.getByLabelText("Kubernetes cluster name")).toBeTruthy();
    expect(container.textContent).toContain("metrics-server found");
  });

  it("a metrics-server check that fails shows its error", async () => {
    vi.mocked(metricsServerFound).mockRejectedValue(new Error("timeout"));
    const { container } = render(await SettingsPage());
    expect(container.textContent).toContain(
      "Could not check for metrics-server: timeout",
    );
  });
});
