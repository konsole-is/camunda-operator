// @vitest-environment jsdom
import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { type CreateStep, useCreateFlow } from "./use-create-flow";

type Draft = {
  name: string;
  spec?: { presetRef?: string; replicas?: number };
  namespace?: { labels?: Record<string, string> };
};

const steps = (
  check: (d: Draft) => string[] = (d) => (d.name ? [] : ["Enter a name"]),
): CreateStep<Draft>[] => [
  { id: "a", label: "A", check },
  { id: "b", label: "B" },
  { id: "c", label: "C" },
];

function setup(initial: Draft = { name: "" }) {
  return renderHook(
    ({ s }: { s: CreateStep<Draft>[] }) => useCreateFlow({ steps: s, initial }),
    { initialProps: { s: steps() } },
  );
}

describe("useCreateFlow", () => {
  it("sets a nested value and keeps the rest of the draft", () => {
    const { result } = setup({ name: "orders", spec: { replicas: 3 } });
    const before = result.current.draft;
    act(() => result.current.set(["spec", "presetRef"], "small"));
    expect(result.current.draft).toEqual({
      name: "orders",
      spec: { replicas: 3, presetRef: "small" },
    });
    expect(result.current.draft).not.toBe(before);
    expect(before.spec).toEqual({ replicas: 3 });
  });

  it("sets a key that holds a dot", () => {
    const { result } = setup();
    act(() =>
      result.current.set(
        ["namespace", "labels", "team.example.com/owner"],
        "search",
      ),
    );
    expect(result.current.draft.namespace?.labels).toEqual({
      "team.example.com/owner": "search",
    });
  });

  it("applies each set of one event", () => {
    const { result } = setup();
    act(() => {
      result.current.set(["name"], "orders");
      result.current.set(["spec", "presetRef"], "small");
    });
    expect(result.current.draft).toEqual({
      name: "orders",
      spec: { presetRef: "small" },
    });
  });

  it("deletes a key on undefined", () => {
    const { result } = setup({ name: "orders", spec: { presetRef: "small" } });
    act(() => result.current.set(["spec", "presetRef"], undefined));
    expect(result.current.draft).toEqual({ name: "orders", spec: {} });
  });

  it("blocks next while the current step has errors", () => {
    const { result } = setup();
    act(() => result.current.next());
    expect(result.current.current).toBe("a");
    expect(result.current.errors.a).toEqual(["Enter a name"]);
  });

  it("moves on once the errors are gone", () => {
    const { result } = setup();
    act(() => result.current.set(["name"], "orders"));
    act(() => result.current.next());
    expect(result.current.current).toBe("b");
  });

  it("reports no errors for a step that is not reached", () => {
    const { result } = setup();
    expect(result.current.errors.b).toBeUndefined();
    expect(result.current.errors.c).toBeUndefined();
  });

  it("keeps the errors of an earlier step", () => {
    const { result, rerender } = setup({ name: "orders" });
    act(() => result.current.next());
    rerender({ s: steps(() => ["Taken"]) });
    expect(result.current.current).toBe("b");
    expect(result.current.errors.a).toEqual(["Taken"]);
  });

  it("goes back with back and with go to an earlier step only", () => {
    const { result } = setup({ name: "orders" });
    act(() => result.current.next());
    act(() => result.current.next());
    expect(result.current.current).toBe("c");
    act(() => result.current.go("b"));
    expect(result.current.current).toBe("b");
    act(() => result.current.go("c"));
    expect(result.current.current).toBe("b");
    act(() => result.current.go("nope"));
    expect(result.current.current).toBe("b");
    act(() => result.current.back());
    expect(result.current.current).toBe("a");
    act(() => result.current.back());
    expect(result.current.current).toBe("a");
  });

  it("reads the newest check of a step", () => {
    const { result, rerender } = setup({ name: "orders" });
    expect(result.current.errors.a).toEqual([]);
    rerender({
      s: steps((d) => (d.name === "orders" ? ["Taken"] : [])),
    });
    expect(result.current.errors.a).toEqual(["Taken"]);
    act(() => result.current.next());
    expect(result.current.current).toBe("a");
  });
});
