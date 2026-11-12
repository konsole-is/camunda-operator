// @vitest-environment jsdom
import { act, renderHook } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";
import { useEditorGeneration } from "./editor-generation";

type Props = { pending: number; value?: unknown };

function hook(pending: number, value?: unknown) {
  const initialProps: Props = { pending, value };
  return renderHook((p: Props) => useEditorGeneration(p.pending, p.value), {
    initialProps,
  });
}

describe("useEditorGeneration", () => {
  it("changes the key when the pending changes end without an edit through own", () => {
    const { result, rerender } = hook(1);
    const before = result.current.generation;
    rerender({ pending: 0 });
    expect(result.current.generation).not.toBe(before);
  });

  it("keeps the key when an edit through own ends the pending changes", () => {
    const { result } = renderHook(() => {
      const [pending, setPending] = useState(1);
      return { ...useEditorGeneration(pending), setPending };
    });
    const before = result.current.generation;
    act(() => result.current.own(() => result.current.setPending(0))());
    expect(result.current.generation).toBe(before);
  });

  it("changes the key when value changes without an edit through own", () => {
    const { result, rerender } = hook(0, { cpu: "1" });
    const before = result.current.generation;
    rerender({ pending: 0, value: { cpu: "1" } });
    expect(result.current.generation).toBe(before);
    rerender({ pending: 0, value: { cpu: "2" } });
    expect(result.current.generation).not.toBe(before);
  });

  it("keeps the key when value changes after an edit through own", () => {
    const { result, rerender } = hook(0, "orders");
    const before = result.current.generation;
    act(() => result.current.own(() => {})());
    rerender({ pending: 0, value: "orders-eu" });
    rerender({ pending: 0, value: "billing" });
    expect(result.current.generation).toBe(before);
  });

  it("follows value again once the key changed after an edit through own", () => {
    const { result, rerender } = hook(1, "orders");
    act(() => result.current.own(() => {})());
    rerender({ pending: 1, value: "orders-eu" });
    rerender({ pending: 0, value: "orders" });
    const after = result.current.generation;
    rerender({ pending: 0, value: "billing" });
    expect(result.current.generation).not.toBe(after);
  });
});
