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
  type SecretRefsReader,
  SecretRefsReaderContext,
  staticSecretRefs,
} from "./namespaced-secret-ref-picker";
import {
  CREDENTIALS_SLOTS,
  type PickedKeys,
  S3_CREDENTIALS_SLOTS,
  SecretKeysPicker,
  type SecretKeySlot,
} from "./secret-keys-picker";

vi.mock("@/lib/k8s/hooks", () => ({
  useResources: () => {
    throw new Error("a story or test must not read live Secrets");
  },
  useWatchesFresh: () => [false],
}));

afterEach(cleanup);

const SECRETS = staticSecretRefs({
  "team-a": [
    { name: "es-user", keys: ["user", "pass", "username", "password"] },
    { name: "s3", keys: ["id", "secret"] },
  ],
  empty: [],
});

function setup(
  props: {
    slots?: readonly SecretKeySlot[];
    value?: PickedKeys;
    namespace?: string;
    reader?: SecretRefsReader;
  } = {},
) {
  const onChange = vi.fn<(ref: PickedKeys | undefined) => void>();
  render(
    <SecretRefsReaderContext.Provider value={props.reader ?? SECRETS}>
      <SecretKeysPicker
        label="Credentials"
        namespace={props.namespace ?? "team-a"}
        slots={props.slots ?? CREDENTIALS_SLOTS}
        value={props.value}
        onChange={onChange}
      />
    </SecretRefsReaderContext.Provider>,
  );
  const group = screen.getByRole("group", { name: "Credentials" });
  const select = (name: string) =>
    within(group).getByRole("combobox", { name }) as HTMLSelectElement;
  const pick = (name: string, value: string) =>
    fireEvent.change(select(name), { target: { value } });
  const options = (name: string) =>
    [...select(name).options].map((o) => o.textContent);
  return { onChange, select, pick, options, group };
}

describe("SecretKeysPicker", () => {
  it("reports the Secret and the key of each slot", () => {
    const { pick, onChange } = setup({ slots: S3_CREDENTIALS_SLOTS });
    pick("Secret", "s3");
    pick("Access key ID key", "id");
    pick("Secret access key key", "secret");
    expect(onChange).toHaveBeenLastCalledWith({
      name: "s3",
      accessKeyIdKey: "id",
      secretAccessKeyKey: "secret",
    });
  });

  it("leaves out a slot that keeps its fallback", () => {
    const { pick, onChange, options } = setup();
    pick("Secret", "es-user");
    expect(options("Username key")[0]).toBe("Default (username)");
    pick("Password key", "pass");
    expect(onChange).toHaveBeenLastCalledWith({
      name: "es-user",
      passwordKey: "pass",
    });
  });

  it("reports nothing until each slot without fallback is picked", () => {
    const { pick, onChange } = setup({ slots: S3_CREDENTIALS_SLOTS });
    pick("Secret", "s3");
    pick("Access key ID key", "id");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("reports undefined when a pick clears a complete reference", () => {
    const { pick, onChange } = setup({ value: { name: "es-user" } });
    pick("Secret", "");
    expect(onChange).toHaveBeenLastCalledWith(undefined);
  });

  it("offers only the keys of the picked Secret", () => {
    const { pick, options } = setup({ slots: S3_CREDENTIALS_SLOTS });
    pick("Secret", "s3");
    expect(options("Access key ID key")).toEqual([
      "Pick a key",
      "id",
      "secret",
    ]);
  });

  it("keeps a value whose Secret is gone, marked as not found", () => {
    const { select, group, onChange } = setup({
      value: { name: "gone", usernameKey: "user" },
    });
    expect(select("Secret").value).toBe("gone");
    expect(within(group).getByText("gone (not found)")).toBeTruthy();
    expect(select("Username key").value).toBe("user");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("marks nothing while the Secrets cannot be read", () => {
    const { select, group } = setup({
      value: { name: "gone", usernameKey: "user" },
      reader: () => ({ error: "forbidden" }),
    });
    expect(select("Secret").value).toBe("gone");
    expect(within(group).queryByText(/not found/)).toBeNull();
    expect(
      within(group).getByText(
        "Could not read the Secrets of team-a: forbidden",
      ),
    ).toBeTruthy();
  });

  it("marks nothing until the Secrets are known", () => {
    const { group } = setup({
      value: { name: "gone" },
      reader: () => ({}),
    });
    expect(within(group).queryByText(/not found/)).toBeNull();
  });

  it("shows that the namespace has no Secret", () => {
    const { group } = setup({ namespace: "empty" });
    expect(
      within(group).getByText("The namespace empty has no Secret."),
    ).toBeTruthy();
  });
});
