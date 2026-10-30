// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { KindMap } from "@/lib/k8s/kinds";
import type { Settings } from "@/lib/settings";
import type { WriteResult } from "@/lib/write/types";
import { SettingsForm, type SettingsFormProps } from "./settings-form";

const refresh = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh }) }));

afterEach(cleanup);
beforeEach(() => refresh.mockReset());

const full: Settings = {
  kubernetesClusterName: "prod-eu",
  "git.repository": "https://git.example.com/fleet.git",
  "git.branch": "main",
  "git.path": "clusters/prod",
  "git.credentialsSecretRef": "git",
  "secretStore.kind": "ClusterSecretStore",
  "secretStore.name": "vault",
  "prometheus.url": "http://prometheus.monitoring:9090",
  "prometheus.credentialsSecretRef": "prom",
};

const noGit: Settings = {
  ...full,
  "git.repository": "",
};

const NOT_PUSHED =
  "Settings stay in the cluster. A save writes the ConfigMap fleet-manager-settings and is never pushed to Git.";
const SAVE_FIRST = "Save first. The test uses the saved settings.";

function props(overrides: Partial<SettingsFormProps> = {}): SettingsFormProps {
  return {
    settings: full,
    resourceVersion: "7",
    namespace: "fleet",
    secretNames: ["git", "prom"],
    layoutKinds: {
      namespaced: KindMap.CamundaClusterClaim,
      clusterScoped: KindMap.Environment,
    },
    save: vi.fn<SettingsFormProps["save"]>(async () => ({
      ok: true,
      mode: "direct",
    })),
    testGit: vi.fn(async () => ({ ok: true, message: "ok" })),
    testPrometheus: vi.fn(async () => ({ ok: true, message: "ok" })),
    ...overrides,
  };
}

const field = (label: string) =>
  screen.getByLabelText(label) as HTMLInputElement | HTMLSelectElement;
const type = (label: string, value: string) =>
  fireEvent.change(field(label), { target: { value } });
const save = () =>
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
const saveButton = () =>
  screen.getByRole("button", { name: "Save" }) as HTMLButtonElement;
const gitMode = () =>
  screen.getByRole("checkbox", { name: "Git mode" }) as HTMLInputElement;

describe("SettingsForm", () => {
  it("shows each key of the settings", () => {
    render(<SettingsForm {...props()} />);
    expect(field("Kubernetes cluster name").value).toBe("prod-eu");
    expect(gitMode().checked).toBe(true);
    expect(field("Repository").value).toBe("https://git.example.com/fleet.git");
    expect(field("Branch").value).toBe("main");
    expect(field("Path").value).toBe("clusters/prod");
    expect(field("Git credentials Secret").value).toBe("git");
    expect(field("SecretStore kind").value).toBe("ClusterSecretStore");
    expect(field("SecretStore name").value).toBe("vault");
    expect(field("Prometheus URL").value).toBe(
      "http://prometheus.monitoring:9090",
    );
    expect(field("Prometheus credentials Secret").value).toBe("prom");
  });

  it("a save sends only the changed keys with the version that the page read", async () => {
    const p = props();
    render(<SettingsForm {...p} />);

    type("Kubernetes cluster name", "prod-us");
    save();

    await waitFor(() =>
      expect(p.save).toHaveBeenCalledWith(
        { kubernetesClusterName: "prod-us" },
        "7",
      ),
    );
  });

  it("turning Git mode off clears git.repository only", async () => {
    const p = props();
    render(<SettingsForm {...p} />);

    fireEvent.click(gitMode());
    save();

    await waitFor(() =>
      expect(p.save).toHaveBeenCalledWith({ "git.repository": "" }, "7"),
    );
  });

  it("in Git mode the page says that settings are not pushed", () => {
    render(<SettingsForm {...props()} />);
    expect(screen.getByText(NOT_PUSHED)).toBeTruthy();

    fireEvent.click(gitMode());

    expect(screen.queryByText(NOT_PUSHED)).toBeNull();
  });

  it("shows the Git fields only in Git mode", () => {
    render(<SettingsForm {...props({ settings: noGit })} />);
    expect(screen.queryByLabelText("Repository")).toBeNull();

    fireEvent.click(gitMode());

    expect(field("Branch").value).toBe("main");
  });

  it("shows the file layout of git.path", () => {
    const { container } = render(<SettingsForm {...props()} />);
    type("Path", "fleet");
    expect(container.textContent).toContain(
      "fleet/team-a/camundaclusterclaims/orders.yaml",
    );
  });

  it("a Stale save shows the message and Reload", async () => {
    const p = props({
      save: vi.fn(async (): Promise<WriteResult> => ({
        ok: false,
        errors: [
          {
            code: "Stale",
            message:
              "The ConfigMap fleet-manager-settings changed since you opened it.",
          },
        ],
      })),
    });
    render(<SettingsForm {...p} />);

    type("Kubernetes cluster name", "prod-us");
    save();

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(
      "The ConfigMap fleet-manager-settings changed since you opened it.",
    );
    fireEvent.click(screen.getByRole("button", { name: "Reload" }));
    expect(refresh).toHaveBeenCalled();
  });

  it("an Invalid error shows at its field", async () => {
    const message = "prometheus.url must be an http:// or https:// address";
    const p = props({
      save: vi.fn(async (): Promise<WriteResult> => ({
        ok: false,
        errors: [{ code: "Invalid", path: "prometheus.url", message }],
      })),
    });
    render(<SettingsForm {...p} />);

    type("Prometheus URL", "prometheus:9090");
    save();

    await waitFor(() =>
      expect(field("Prometheus URL").getAttribute("aria-invalid")).toBe("true"),
    );
    const describedBy =
      field("Prometheus URL").getAttribute("aria-describedby");
    expect(
      describedBy
        ?.split(" ")
        .map((id) => document.getElementById(id)?.textContent),
    ).toContain(message);
  });

  it("a good save refreshes the page, and the next save sends the new version", async () => {
    const p = props();
    const { rerender } = render(<SettingsForm {...p} />);

    type("Kubernetes cluster name", "prod-us");
    save();
    await waitFor(() => expect(refresh).toHaveBeenCalled());
    expect(screen.getByRole("status").textContent).toContain(
      "Saved the settings.",
    );

    rerender(
      <SettingsForm
        {...p}
        settings={{ ...full, kubernetesClusterName: "prod-us" }}
        resourceVersion="8"
      />,
    );
    expect(saveButton().disabled).toBe(true);
    expect(screen.getByRole("status").textContent).toContain(
      "Saved the settings.",
    );

    type("Branch", "release");
    save();

    await waitFor(() =>
      expect(p.save).toHaveBeenLastCalledWith({ "git.branch": "release" }, "8"),
    );
  });

  it("a value changed while a save runs stays after the refresh", async () => {
    let finish: (result: WriteResult) => void = () => {};
    const p = props({
      save: vi.fn(
        () =>
          new Promise<WriteResult>((resolve) => {
            finish = resolve;
          }),
      ),
    });
    const { rerender } = render(<SettingsForm {...p} />);

    type("Kubernetes cluster name", "prod-us");
    save();
    type("Branch", "release");
    finish({ ok: true, mode: "direct" });
    await waitFor(() => expect(refresh).toHaveBeenCalled());
    rerender(
      <SettingsForm
        {...p}
        settings={{ ...full, kubernetesClusterName: "prod-us" }}
        resourceVersion="8"
      />,
    );

    expect(field("Kubernetes cluster name").value).toBe("prod-us");
    expect(field("Branch").value).toBe("release");
    save();
    await waitFor(() =>
      expect(p.save).toHaveBeenLastCalledWith({ "git.branch": "release" }, "8"),
    );
  });

  it("a Secret list that failed shows its error and marks no Secret as missing", () => {
    render(
      <SettingsForm
        {...props({ secretNames: { error: "secrets is forbidden" } })}
      />,
    );
    expect(
      screen.getByText(
        "Could not list the Secrets of namespace fleet: secrets is forbidden",
      ),
    ).toBeTruthy();
    expect(field("Git credentials Secret").value).toBe("git");
    expect(screen.queryByText(/not found/)).toBeNull();
  });

  it("cannot save again until the refresh brings the new version", async () => {
    const p = props();
    const { rerender } = render(<SettingsForm {...p} />);

    type("Kubernetes cluster name", "prod-us");
    save();
    await waitFor(() => expect(refresh).toHaveBeenCalled());
    type("Branch", "release");
    expect(saveButton().disabled).toBe(true);

    rerender(
      <SettingsForm
        {...p}
        settings={{ ...full, kubernetesClusterName: "prod-us" }}
        resourceVersion="8"
      />,
    );
    expect(saveButton().disabled).toBe(false);
  });

  it("a refused save can be sent again", async () => {
    const p = props({
      save: vi.fn(async (): Promise<WriteResult> => ({
        ok: false,
        errors: [{ code: "Unavailable", message: "timeout" }],
      })),
    });
    render(<SettingsForm {...p} />);

    type("Kubernetes cluster name", "prod-us");
    save();

    await screen.findByRole("alert");
    expect(saveButton().disabled).toBe(false);
  });

  it("a test result goes away when the saved settings change", async () => {
    const p = props();
    const { rerender } = render(<SettingsForm {...p} />);
    fireEvent.click(screen.getByRole("button", { name: "Test the query" }));
    await screen.findByRole("status");

    rerender(<SettingsForm {...p} resourceVersion="8" />);

    expect(screen.queryByRole("status")).toBeNull();
  });

  it("cannot save without a change", () => {
    render(<SettingsForm {...props()} />);
    expect(saveButton().disabled).toBe(true);
  });

  it("Git mode on without a repository cannot save", () => {
    render(<SettingsForm {...props({ settings: noGit })} />);

    fireEvent.click(gitMode());

    expect(
      screen.getByText("Set the repository to turn on Git mode."),
    ).toBeTruthy();
    expect(saveButton().disabled).toBe(true);
  });

  it("an invalid path cannot save in Git mode", () => {
    render(<SettingsForm {...props()} />);

    type("Path", "a/../b");

    expect(
      screen.getByText("git.path must name a folder in the repository."),
    ).toBeTruthy();
    expect(saveButton().disabled).toBe(true);
  });

  it("Test the connection reports the result of testGit", async () => {
    const testGit = vi
      .fn()
      .mockResolvedValueOnce({
        ok: true,
        message: "Connected to https://git.example.com/fleet.git.",
      })
      .mockResolvedValueOnce({
        ok: false,
        message: "https://git.example.com/fleet.git refused the credentials.",
      });
    render(<SettingsForm {...props({ testGit })} />);
    const button = screen.getByRole("button", { name: "Test the connection" });

    fireEvent.click(button);
    expect((await screen.findByRole("status")).textContent).toContain(
      "Connected to https://git.example.com/fleet.git.",
    );

    fireEvent.click(button);
    expect((await screen.findByRole("alert")).textContent).toContain(
      "https://git.example.com/fleet.git refused the credentials.",
    );
  });

  it("Test the query reports the result of testPrometheus", async () => {
    const testPrometheus = vi
      .fn()
      .mockResolvedValueOnce({
        ok: true,
        message: "Prometheus answered the query.",
      })
      .mockResolvedValueOnce({
        ok: false,
        message: "Prometheus answered 401 Unauthorized.",
      });
    render(<SettingsForm {...props({ testPrometheus })} />);
    const button = screen.getByRole("button", { name: "Test the query" });

    fireEvent.click(button);
    expect((await screen.findByRole("status")).textContent).toContain(
      "Prometheus answered the query.",
    );

    fireEvent.click(button);
    expect((await screen.findByRole("alert")).textContent).toContain(
      "Prometheus answered 401 Unauthorized.",
    );
  });

  it("the tests are disabled while the form has a change", () => {
    render(<SettingsForm {...props()} />);

    type("Branch", "release");

    for (const name of ["Test the connection", "Test the query"]) {
      expect(
        (screen.getByRole("button", { name }) as HTMLButtonElement).disabled,
      ).toBe(true);
    }
    expect(screen.getAllByText(SAVE_FIRST).length).toBeGreaterThan(0);
  });

  it("without Prometheus the page says that charts do not show", () => {
    render(
      <SettingsForm
        {...props({ settings: { ...full, "prometheus.url": "" } })}
      />,
    );
    expect(
      screen.getByText("Charts do not show. Everything else works."),
    ).toBeTruthy();
  });

  it("with Prometheus the page does not say that charts do not show", () => {
    render(<SettingsForm {...props()} />);
    expect(
      screen.queryByText("Charts do not show. Everything else works."),
    ).toBeNull();
  });
});
