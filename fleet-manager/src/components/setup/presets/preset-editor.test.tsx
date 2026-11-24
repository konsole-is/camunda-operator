// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { SecretRefs } from "@/components/fleet/secret-ref-picker";
import type { PresetKind } from "@/lib/usage";
import { type PresetDraft, presetDraftOf } from "./draft";
import { presetFixtures } from "./fixtures";
import type { PresetObject } from "./kinds";
import { PresetEditor } from "./preset-editor";

afterEach(cleanup);

const fixtures = presetFixtures();

function setup(
  preset: PresetObject,
  secrets: Readonly<Record<string, SecretRefs>> = {},
  draft: PresetDraft = presetDraftOf(preset),
  secretsUnread: readonly string[] = [],
) {
  const drafts = vi.fn<(draft: PresetDraft) => void>();
  const errors = vi.fn<(errors: string[]) => void>();
  function Harness() {
    const [current, setCurrent] = useState(draft);
    return (
      <PresetEditor
        kind={preset.kind as PresetKind}
        draft={current}
        secrets={secrets}
        secretsUnread={secretsUnread}
        onDraft={(update) =>
          setCurrent((d) => {
            const next = update(d);
            drafts(next);
            return next;
          })
        }
        onErrors={errors}
      />
    );
  }
  render(<Harness />);
  return { drafts, errors, last: () => drafts.mock.calls.at(-1)?.[0] };
}

describe("PresetEditor of a cluster preset", () => {
  it("edits a cluster preset field and writes it into the body", async () => {
    const { last } = setup(fixtures.clusterPresets[0]);
    const brokers = screen.getByRole("region", { name: "Brokers settings" });
    const replicas = within(brokers).getByRole("group", { name: "replicas" });
    fireEvent.change(within(replicas).getByRole("spinbutton"), {
      target: { value: "5" },
    });
    await waitFor(() =>
      expect(
        (last()?.body.zeebe as Record<string, unknown> | undefined)?.replicas,
      ).toBe(5),
    );
    expect((last()?.body.zeebe as Record<string, unknown>).partitions).toBe(3);
  });

  it("offers no editor for a field that a preset cannot set", () => {
    setup(fixtures.clusterPresets[0]);
    const general = screen.getByRole("region", { name: "General settings" });
    for (const label of ["platformConfigRef", "version", "presetRef"])
      expect(within(general).queryByLabelText(label)).toBeNull();
    const connectors = screen.getByRole("region", {
      name: "Connectors settings",
    });
    expect(within(connectors).queryByLabelText("version")).toBeNull();
    expect(
      within(connectors).queryByLabelText("serviceAccount.name"),
    ).toBeNull();
  });

  it("refuses an annotation key that Kubernetes refuses, for a web app that the preset does not run standalone", async () => {
    const { drafts } = setup(fixtures.clusterPresets[0]);
    const tasklist = screen.getByRole("region", { name: "Tasklist settings" });
    const annotations = within(tasklist).getAllByRole("group", {
      name: "podAnnotations",
    })[0];
    fireEvent.click(
      within(annotations).getByRole("button", { name: "Add an entry" }),
    );
    fireEvent.change(
      within(annotations).getAllByRole("textbox", { name: "Key" }).at(-1)!,
      { target: { value: "bad key" } },
    );
    expect(
      within(annotations).getByText(
        "Use a valid annotation key, for example example.com/note",
      ),
    ).toBeTruthy();
    expect(
      drafts.mock.calls.some(([d]) =>
        JSON.stringify(d.body).includes("bad key"),
      ),
    ).toBe(false);
  });

  it("removes a field from the preset with its reset link", async () => {
    const { last } = setup(fixtures.clusterPresets[0]);
    const brokers = screen.getByRole("region", { name: "Brokers settings" });
    const replicas = within(brokers).getByRole("group", { name: "replicas" });
    fireEvent.click(
      within(replicas).getByRole("button", { name: "Remove from the preset" }),
    );
    await waitFor(() =>
      expect(last()?.body.zeebe).not.toHaveProperty("replicas"),
    );
  });

  it("shows the merge rule of each field that the draft sets in its group", () => {
    setup(fixtures.clusterPresets[0]);
    const rules = screen.getByRole("list", { name: "Merge rules of Brokers" });
    expect(rules.textContent).toContain("zeebe.replicas");
    expect(rules.textContent).toContain("A resource value replaces it");
  });

  it("edits the env lists in the scopes of the env editor, not as settings", () => {
    setup(fixtures.clusterPresets[0]);
    const settings = screen.getByRole("region", { name: "Brokers settings" });
    expect(
      within(settings).queryByRole("group", { name: "extraEnv" }),
    ).toBeNull();
    const scope = screen.getByRole("group", { name: "Brokers" });
    expect(
      within(scope)
        .getByRole("button", { name: /Brokers/ })
        .getAttribute("aria-expanded"),
    ).toBe("true");
    expect(within(scope).getByDisplayValue("JAVA_TOOL_OPTIONS")).toBeTruthy();
  });

  it("says where to change a variable of a scope that the editor cannot change", () => {
    const preset = fixtures.clusterPresets[0];
    setup(
      preset,
      {},
      {
        description: "",
        body: {
          ...preset.spec.cluster,
          extraEnv: [
            {
              name: "POD_IP",
              valueFrom: { fieldRef: { fieldPath: "status.podIP" } },
            },
          ],
        },
      },
    );
    expect(screen.getByRole("group", { name: "POD_IP" }).textContent).toContain(
      "Edit this variable with kubectl or through your Git repository",
    );
  });

  it("keeps the last valid env list and shows the YAML error again when the scope reopens", async () => {
    const { last, errors } = setup(fixtures.clusterPresets[0]);
    const brokers = screen.getByRole("group", { name: "Brokers" });
    const yaml = within(brokers).getByLabelText("extraEnvFrom of Brokers");
    const envFrom = () =>
      (last()?.body.zeebe as Record<string, unknown> | undefined)?.extraEnvFrom;
    const valid = [{ configMapRef: { name: "app" } }];
    fireEvent.change(yaml, {
      target: { value: "- configMapRef:\n    name: app\n" },
    });
    await waitFor(() => expect(envFrom()).toEqual(valid));
    fireEvent.change(yaml, { target: { value: "- configMapRef: [" } });
    await waitFor(() => expect(errors.mock.calls.at(-1)?.[0]).not.toEqual([]));
    const toggle = within(brokers).getByRole("button", { name: /Brokers/ });
    fireEvent.click(toggle);
    fireEvent.click(toggle);
    expect(
      (
        within(brokers).getByLabelText(
          "extraEnvFrom of Brokers",
        ) as HTMLTextAreaElement
      ).value,
    ).toBe("- configMapRef: [");
    expect(within(brokers).getByText(/Line 1/)).toBeTruthy();
    expect(envFrom()).toEqual(valid);
  });
});

describe("PresetEditor of an Elasticsearch or PostgreSQL preset", () => {
  it("edits a field of an Elasticsearch preset with its field editor and writes it into the body", async () => {
    const { last } = setup(fixtures.elasticsearchPresets[0]);
    const settings = screen.getByRole("region", {
      name: "Elasticsearch settings",
    });
    const replicas = within(settings).getByRole("group", { name: "replicas" });
    fireEvent.change(within(replicas).getByRole("spinbutton"), {
      target: { value: "12" },
    });
    await waitFor(() => expect(last()?.body.replicas).toBe(12));
    expect(last()?.body.storageSize).toBe("64Gi");
  });

  it("edits a field of a PostgreSQL preset with its field editor and writes it into the body", async () => {
    const { last } = setup(fixtures.databasePresets[1]);
    const settings = screen.getByRole("region", {
      name: "PostgreSQL settings",
    });
    const instances = within(settings).getByRole("group", {
      name: "instances",
    });
    fireEvent.change(within(instances).getByRole("spinbutton"), {
      target: { value: "12" },
    });
    await waitFor(() => expect(last()?.body.instances).toBe(12));
    expect(last()?.body.walStorageSize).toBe("5Gi");
  });

  it("shows the merge rule of each field that an Elasticsearch or a PostgreSQL preset sets", () => {
    setup(fixtures.elasticsearchPresets[0]);
    const elasticsearch = screen.getByRole("list", {
      name: "Merge rules of Elasticsearch",
    });
    expect(
      within(elasticsearch).getByText("replicas").closest("li")?.textContent,
    ).toContain("A resource value replaces it");
    expect(
      within(elasticsearch).getByText("resources").closest("li")?.textContent,
    ).toContain("Replaces whole");
    cleanup();
    setup(fixtures.databasePresets[1]);
    const server = screen.getByRole("list", {
      name: "Merge rules of PostgreSQL",
    });
    expect(
      within(server).getByText("walStorageSize").closest("li")?.textContent,
    ).toContain("A resource value replaces it");
  });

  it("offers no editor for a field that an Elasticsearch or a PostgreSQL preset cannot set", () => {
    setup(fixtures.elasticsearchPresets[0]);
    const elasticsearch = screen.getByRole("region", {
      name: "Elasticsearch settings",
    });
    for (const label of [
      "presetRef",
      "releaseRef",
      "secondaryStorageConfig",
      "suspend",
      "version",
    ])
      expect(
        within(elasticsearch).queryByRole("group", { name: label }),
      ).toBeNull();
    cleanup();
    setup(fixtures.databasePresets[1]);
    const server = screen.getByRole("region", { name: "PostgreSQL settings" });
    for (const label of [
      "presetRef",
      "releaseRef",
      "databaseServerConfig",
      "suspend",
      "version",
    ])
      expect(within(server).queryByRole("group", { name: label })).toBeNull();
    expect(
      within(server).getByRole("group", { name: "platformConfigRef" }),
    ).toBeTruthy();
  });

  it("edits extraEnv of an Elasticsearch preset as a setting that replaces the list of a resource", () => {
    const preset = fixtures.elasticsearchPresets[0];
    setup(
      preset,
      {},
      {
        description: "",
        body: {
          ...preset.spec.cluster,
          extraEnv: [{ name: "ES_JAVA_OPTS", value: "-Xms2g" }],
        },
      },
    );
    const settings = screen.getByRole("region", {
      name: "Elasticsearch settings",
    });
    expect(
      within(
        // The env editor names its own group after the setting too.
        within(settings).getAllByRole("group", { name: "extraEnv" })[0],
      ).getByDisplayValue("ES_JAVA_OPTS"),
    ).toBeTruthy();
    const rules = screen.getByRole("list", {
      name: "Merge rules of Elasticsearch",
    });
    expect(
      within(rules).getByText("extraEnv").closest("li")?.textContent,
    ).toContain("Replaces whole");
  });

  it("refuses a replicas value below the minimum of the CRD and keeps the last valid body", async () => {
    const { last, errors } = setup(fixtures.elasticsearchPresets[0]);
    const replicas = within(
      screen.getByRole("region", { name: "Elasticsearch settings" }),
    ).getByRole("group", { name: "replicas" });
    fireEvent.change(within(replicas).getByRole("spinbutton"), {
      target: { value: "0" },
    });
    expect(await within(replicas).findByText("Enter 1 or more")).toBeTruthy();
    await waitFor(() =>
      expect(errors.mock.calls.at(-1)?.[0]).toEqual([
        "replicas: Enter 1 or more",
      ]),
    );
    expect(last()?.body.replicas ?? 3).toBe(3);
  });

  it("keeps a body field that the field schema does not know when another field changes", async () => {
    const preset = fixtures.elasticsearchPresets[0];
    const { last } = setup(
      preset,
      {},
      {
        description: "",
        body: { ...preset.spec.cluster, futureField: "kept" },
      },
    );
    const replicas = within(
      screen.getByRole("region", { name: "Elasticsearch settings" }),
    ).getByRole("group", { name: "replicas" });
    fireEvent.change(within(replicas).getByRole("spinbutton"), {
      target: { value: "12" },
    });
    await waitFor(() => expect(last()?.body.replicas).toBe(12));
    expect(last()?.body.futureField).toBe("kept");
  });

  it("keeps an empty storage class, an empty block, and variables from a ConfigMap or the pod when another field changes", async () => {
    const preset = fixtures.elasticsearchPresets[0];
    const kept = {
      storageClassName: "",
      serviceAccount: {},
      extraEnv: [
        {
          name: "NODE_NAME",
          valueFrom: { fieldRef: { fieldPath: "spec.nodeName" } },
        },
        {
          name: "ES_SETTINGS",
          valueFrom: { configMapKeyRef: { name: "es-settings", key: "yml" } },
        },
      ],
    };
    const { last } = setup(
      preset,
      {},
      {
        description: "",
        body: { ...preset.spec.cluster, ...kept },
      },
    );
    const replicas = within(
      screen.getByRole("region", { name: "Elasticsearch settings" }),
    ).getByRole("group", { name: "replicas" });
    fireEvent.change(within(replicas).getByRole("spinbutton"), {
      target: { value: "12" },
    });
    await waitFor(() => expect(last()?.body.replicas).toBe(12));
    expect(last()?.body).toEqual({
      ...preset.spec.cluster,
      ...kept,
      replicas: 12,
    });
  });

  it("keeps an empty storage class and an empty block of a PostgreSQL preset when another field changes", async () => {
    const preset = fixtures.databasePresets[1];
    const kept = { storageClassName: "", serviceAccount: {} };
    const { last } = setup(
      preset,
      {},
      {
        description: "",
        body: { ...preset.spec.server, ...kept },
      },
    );
    const instances = within(
      screen.getByRole("region", { name: "PostgreSQL settings" }),
    ).getByRole("group", { name: "instances" });
    fireEvent.change(within(instances).getByRole("spinbutton"), {
      target: { value: "12" },
    });
    await waitFor(() => expect(last()?.body.instances).toBe(12));
    expect(last()?.body).toEqual({
      ...preset.spec.server,
      ...kept,
      instances: 12,
    });
  });

  it("says where to change a variable from the pod that the editor cannot change", () => {
    const preset = fixtures.elasticsearchPresets[0];
    setup(
      preset,
      {},
      {
        description: "",
        body: {
          ...preset.spec.cluster,
          extraEnv: [
            {
              name: "NODE_NAME",
              valueFrom: { fieldRef: { fieldPath: "spec.nodeName" } },
            },
          ],
        },
      },
    );
    const settings = screen.getByRole("region", {
      name: "Elasticsearch settings",
    });
    expect(
      within(settings).getByRole("group", { name: "NODE_NAME" }).textContent,
    ).toContain(
      "Edit this variable with kubectl or through your Git repository",
    );
    expect(within(settings).queryByText(/YAML of the claim/)).toBeNull();
  });

  it("removes a field from a PostgreSQL preset with its reset link", async () => {
    const { last } = setup(fixtures.databasePresets[1]);
    const wal = within(
      screen.getByRole("region", { name: "PostgreSQL settings" }),
    ).getByRole("group", { name: "walStorageSize" });
    fireEvent.click(
      within(wal).getByRole("button", { name: "Remove from the preset" }),
    );
    await waitFor(() =>
      expect(last()?.body).not.toHaveProperty("walStorageSize"),
    );
  });

  it("shows a Secret key that is missing in a user namespace", () => {
    const preset = fixtures.elasticsearchPresets[0];
    setup(
      preset,
      {
        "team-a": [{ name: "db-credentials", keys: ["password"] }],
        "team-b": [{ name: "db-credentials", keys: ["user"] }],
      },
      {
        description: "",
        body: {
          extraEnv: [
            {
              name: "PASSWORD",
              valueFrom: {
                secretKeyRef: { name: "db-credentials", key: "password" },
              },
            },
          ],
        },
      },
    );
    expect(
      screen.getByText("db-credentials/password is missing in team-b."),
    ).toBeTruthy();
  });
});

describe("PresetEditor description", () => {
  it("edits the description", async () => {
    const { last } = setup(fixtures.databasePresets[1]);
    fireEvent.change(screen.getByLabelText("Description"), {
      target: { value: "For the plane" },
    });
    await waitFor(() => expect(last()?.description).toBe("For the plane"));
  });

  it("names the lists that it cannot read in place of the Secret lists", () => {
    setup(fixtures.elasticsearchPresets[0], {}, undefined, ["CamundaCluster"]);
    expect(screen.getByRole("alert").textContent).toBe(
      "The Secrets of the users are not known while the fleet manager cannot read CamundaCluster. Reload the page when it can read them.",
    );
  });
});
