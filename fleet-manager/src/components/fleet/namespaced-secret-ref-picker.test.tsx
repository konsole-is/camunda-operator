// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  type NamespacedSecretRef,
  NamespacedSecretRefPicker,
  type SecretRefsReader,
  SecretRefsReaderContext,
  staticSecretRefs,
} from "./namespaced-secret-ref-picker";

vi.mock("@/lib/k8s/hooks", () => ({
  useResources: () => {
    throw new Error("a story or test must not read live Secrets");
  },
  useWatchesFresh: () => [false],
}));

afterEach(cleanup);

const SECRETS = staticSecretRefs({
  "camunda-system": [
    { name: "oidc-credentials", keys: ["client-secret", "client-id"] },
    { name: "camunda-license", keys: ["license-key"] },
  ],
  "team-a": [{ name: "orders-oidc", keys: ["secret"] }],
});

function setup(
  props: { value?: NamespacedSecretRef; reader?: SecretRefsReader } = {},
) {
  const onChange = vi.fn<(ref: NamespacedSecretRef | undefined) => void>();
  render(
    <SecretRefsReaderContext.Provider value={props.reader ?? SECRETS}>
      <NamespacedSecretRefPicker
        label="Client secret"
        namespaces={["camunda-system", "team-a"]}
        value={props.value}
        onChange={onChange}
      />
    </SecretRefsReaderContext.Provider>,
  );
  const group = screen.getByRole("group", { name: "Client secret" });
  const select = (name: string) =>
    within(group).getByRole("combobox", { name }) as HTMLSelectElement;
  const options = (name: string) =>
    [...select(name).options].map((o) => o.textContent);
  return { onChange, select, options, group };
}

describe("NamespacedSecretRefPicker", () => {
  it("lists the namespaces and the Secrets and keys of the picked namespace", () => {
    const { select, options } = setup();
    expect(options("Namespace")).toEqual([
      "Pick a namespace",
      "camunda-system",
      "team-a",
    ]);

    fireEvent.change(select("Namespace"), {
      target: { value: "camunda-system" },
    });
    expect(options("Secret")).toEqual([
      "None",
      "oidc-credentials",
      "camunda-license",
    ]);

    fireEvent.change(select("Secret"), {
      target: { value: "oidc-credentials" },
    });
    expect(options("Key")).toEqual([
      "Pick a key",
      "client-secret",
      "client-id",
    ]);
  });

  it("reports the reference when the namespace, the Secret, and the key are picked", () => {
    const { select, onChange } = setup();

    fireEvent.change(select("Namespace"), { target: { value: "team-a" } });
    fireEvent.change(select("Secret"), { target: { value: "orders-oidc" } });
    expect(onChange).not.toHaveBeenCalled();
    fireEvent.change(select("Key"), { target: { value: "secret" } });

    expect(onChange).toHaveBeenLastCalledWith({
      namespace: "team-a",
      name: "orders-oidc",
      key: "secret",
    });
  });

  it("clears the Secret and the key and reports nothing picked when the namespace changes", () => {
    const { select, onChange } = setup({
      value: {
        namespace: "camunda-system",
        name: "oidc-credentials",
        key: "client-secret",
      },
    });

    fireEvent.change(select("Namespace"), { target: { value: "team-a" } });

    expect(select("Secret").value).toBe("");
    expect(select("Key").value).toBe("");
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith(undefined);
  });

  it("keeps a reference whose Secret does not exist and reports no change", () => {
    const { select, options, onChange } = setup({
      value: {
        namespace: "camunda-system",
        name: "camunda-license",
        key: "license-key",
      },
      reader: staticSecretRefs({}),
    });

    expect(select("Secret").value).toBe("camunda-license");
    expect(options("Secret")).toContain("camunda-license (not found)");
    expect(select("Key").value).toBe("license-key");
    expect(
      screen.getByText(
        "The Secret camunda-license does not exist in namespace camunda-system.",
      ),
    ).toBeDefined();
    expect(onChange).not.toHaveBeenCalled();
  });

  it("keeps a key that the Secret does not hold and marks it", () => {
    const { select, options, onChange } = setup({
      value: {
        namespace: "camunda-system",
        name: "camunda-license",
        key: "old-key",
      },
    });

    expect(select("Key").value).toBe("old-key");
    expect(options("Key")).toContain("old-key (not found)");
    expect(select("Key").getAttribute("aria-invalid")).toBe("true");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("keeps a namespace that is not in the list as an option", () => {
    const { select, options } = setup({
      value: { namespace: "gone", name: "s", key: "k" },
    });

    expect(select("Namespace").value).toBe("gone");
    expect(options("Namespace")).toContain("gone");
  });

  it("does not mark a Secret missing while the list is not known", () => {
    setup({
      value: {
        namespace: "camunda-system",
        name: "camunda-license",
        key: "license-key",
      },
      reader: () => ({}),
    });

    expect(screen.queryByText(/not found/)).toBeNull();
    expect(screen.queryByText(/does not exist/)).toBeNull();
  });

  it("shows the read error of the Secrets of the namespace", () => {
    setup({
      value: { namespace: "camunda-system", name: "s", key: "k" },
      reader: () => ({ error: "secrets is forbidden" }),
    });

    expect(screen.getByRole("alert").textContent).toContain(
      "secrets is forbidden",
    );
  });

  it("reads no Secrets before a namespace is picked", () => {
    const reader = vi.fn<SecretRefsReader>(() => ({ secrets: [] }));
    const { select } = setup({ reader });

    expect(reader).not.toHaveBeenCalled();
    fireEvent.change(select("Namespace"), { target: { value: "team-a" } });
    expect(reader).toHaveBeenCalledWith("team-a");
    expect(reader).not.toHaveBeenCalledWith("");
  });
});
