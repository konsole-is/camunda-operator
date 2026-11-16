// @vitest-environment jsdom
import { cleanup, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NamespaceSecrets } from "./namespace-secrets";

const live = vi.hoisted(() => ({
  items: [] as unknown[],
  error: undefined as string | undefined,
  asked: [] as { kind: string; opts: unknown }[],
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, opts: unknown) => {
    live.asked.push({ kind, opts });
    return { items: live.items, error: live.error };
  },
}));

beforeEach(() => {
  live.items = [];
  live.error = undefined;
  live.asked = [];
});
afterEach(cleanup);

const secret = (name: string, keys: string[]) => ({
  metadata: { name, namespace: "team-a" },
  keys,
});

describe("NamespaceSecrets", () => {
  it("reports the Secret refs of its namespace", () => {
    live.items = [secret("db", ["password"]), secret("es", ["user", "pass"])];
    const onChange = vi.fn();
    render(<NamespaceSecrets namespace="team-a" onChange={onChange} />);
    expect(onChange).toHaveBeenCalledWith(
      [
        { name: "db", keys: ["password"] },
        { name: "es", keys: ["user", "pass"] },
      ],
      undefined,
    );
    expect(live.asked).toContainEqual({
      kind: "Secret",
      opts: { namespace: "team-a" },
    });
  });

  it("reports the read error with the refs it has", () => {
    live.items = [secret("db", ["password"])];
    live.error = "forbidden";
    const onChange = vi.fn();
    render(<NamespaceSecrets namespace="team-a" onChange={onChange} />);
    expect(onChange).toHaveBeenCalledWith(
      [{ name: "db", keys: ["password"] }],
      "forbidden",
    );
  });

  it("does not report again while the refs stay the same", () => {
    live.items = [secret("db", ["password"])];
    const onChange = vi.fn();
    const { rerender } = render(
      <NamespaceSecrets namespace="team-a" onChange={onChange} />,
    );
    live.items = [secret("db", ["password"])];
    rerender(<NamespaceSecrets namespace="team-a" onChange={onChange} />);
    expect(onChange).toHaveBeenCalledTimes(1);
  });
});
