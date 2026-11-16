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
  it("edits an Elasticsearch preset as YAML and reports a parse error", async () => {
    const { last, errors } = setup(fixtures.elasticsearchPresets[0]);
    const yaml = screen.getByLabelText("spec.cluster");
    fireEvent.change(yaml, { target: { value: "replicas: 5\n" } });
    await waitFor(() => expect(last()?.body).toEqual({ replicas: 5 }));
    fireEvent.change(yaml, { target: { value: "replicas: [5\n" } });
    await waitFor(() => expect(errors.mock.calls.at(-1)?.[0]).toHaveLength(1));
    expect(last()?.body).toEqual({ replicas: 5 });
  });

  it("labels the body of a PostgreSQL preset spec.server", () => {
    setup(fixtures.databasePresets[1]);
    expect(
      (screen.getByLabelText("spec.server") as HTMLTextAreaElement).value,
    ).toContain("walStorageSize: 5Gi");
  });

  it("refuses a body that is not a map", async () => {
    const { errors } = setup(fixtures.databasePresets[1]);
    fireEvent.change(screen.getByLabelText("spec.server"), {
      target: { value: "- a\n" },
    });
    await waitFor(() =>
      expect(errors.mock.calls.at(-1)?.[0]).toEqual([
        "The body must be a map of fields.",
      ]),
    );
    expect(screen.getByText("The body must be a map of fields.")).toBeTruthy();
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
