// @vitest-environment jsdom
import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Change, Preview, WriteResult } from "@/lib/write/types";
import { useCreateReview } from "./use-create-review";

const change = (intent: string): Change => ({
  intent,
  apply: [
    {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaClusterClaim",
      metadata: { name: "orders", namespace: "team-a" },
    },
  ],
  create: ["CamundaClusterClaim/team-a/orders"],
});

const clean: Preview = { objects: [], errors: [] };

// deferred returns a promise and the function that settles it.
function deferred<T>() {
  let resolve: (value: T) => void = () => {};
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

function setup(over: Partial<Parameters<typeof useCreateReview>[0]> = {}) {
  const input = {
    change: change("Create the cluster orders in team-a"),
    preview: vi.fn(async () => clean),
    create: vi.fn(async (): Promise<WriteResult> => ({
      ok: true,
      mode: "direct",
    })),
    onWritten: vi.fn(),
    ...over,
  };
  const hook = renderHook((props: typeof input) => useCreateReview(props), {
    initialProps: input,
  });
  return { input, ...hook };
}

describe("useCreateReview", () => {
  it("previews the change once and turns pending off when the preview is clean", async () => {
    const { input, result, rerender } = setup();
    expect(result.current.pending).toBe(true);
    await waitFor(() => expect(result.current.pending).toBe(false));
    rerender({ ...input });
    expect(input.preview).toHaveBeenCalledTimes(1);
    expect(input.preview).toHaveBeenCalledWith(input.change);
    expect(result.current.refusals).toEqual([]);
  });

  it("previews again when the change changes, and stays pending until it returns", async () => {
    const second = deferred<Preview>();
    const preview = vi
      .fn<(c: Change) => Promise<Preview>>()
      .mockResolvedValueOnce(clean)
      .mockReturnValueOnce(second.promise);
    const { input, result, rerender } = setup({ preview });
    await waitFor(() => expect(result.current.pending).toBe(false));
    rerender({
      ...input,
      change: change("Create the cluster orders in team-b"),
    });
    expect(preview).toHaveBeenCalledTimes(2);
    expect(result.current.pending).toBe(true);
    await act(async () => second.resolve(clean));
    expect(result.current.pending).toBe(false);
  });

  it("keeps pending on while the preview has refusals, and lists them", async () => {
    const { result } = setup({
      preview: vi.fn(async () => ({
        objects: [],
        errors: [
          { code: "Invalid" as const, message: "spec.presetRef: Required" },
          {
            code: "Forbidden" as const,
            message: "The app may not create Namespaces.",
          },
        ],
      })),
    });
    await waitFor(() =>
      expect(result.current.refusals).toEqual([
        "spec.presetRef: Required",
        "The app may not create Namespaces.",
      ]),
    );
    expect(result.current.pending).toBe(true);
  });

  it("turns a rejected preview into an Unavailable refusal", async () => {
    const { result } = setup({
      preview: vi.fn(async () => {
        throw new Error("fetch failed");
      }),
    });
    await waitFor(() =>
      expect(result.current.refusals).toEqual(["Error: fetch failed"]),
    );
    expect(result.current.pending).toBe(true);
  });

  it("writes once on two quick confirms", async () => {
    const write = deferred<WriteResult>();
    const create = vi.fn(() => write.promise);
    const { input, result } = setup({ create });
    await waitFor(() => expect(result.current.pending).toBe(false));
    await act(async () => {
      void result.current.confirm();
      void result.current.confirm();
    });
    expect(create).toHaveBeenCalledTimes(1);
    expect(result.current.pending).toBe(true);
    await act(async () => write.resolve({ ok: true, mode: "direct" }));
    expect(input.onWritten).toHaveBeenCalledWith({ ok: true, mode: "direct" });
    expect(result.current.pending).toBe(false);
  });

  it("gives a thrown create to onWritten as an Unavailable refusal", async () => {
    const { input, result } = setup({
      create: vi.fn(async () => {
        throw new Error("network down");
      }),
    });
    await waitFor(() => expect(result.current.pending).toBe(false));
    await act(() => result.current.confirm());
    expect(input.onWritten).toHaveBeenCalledWith({
      ok: false,
      errors: [{ code: "Unavailable", message: "Error: network down" }],
    });
  });

  it("keeps pending on while blocked, and confirm writes nothing", async () => {
    const { input, result } = setup({ blocked: true });
    await waitFor(() => expect(input.preview).toHaveBeenCalled());
    await act(async () => {});
    expect(result.current.pending).toBe(true);
    await act(() => result.current.confirm());
    expect(input.create).not.toHaveBeenCalled();
  });

  it("writes nothing while the preview has refusals", async () => {
    const { input, result } = setup({
      preview: vi.fn(async () => ({
        objects: [],
        errors: [{ code: "Invalid" as const, message: "Refused." }],
      })),
    });
    await waitFor(() => expect(result.current.refusals).toEqual(["Refused."]));
    await act(() => result.current.confirm());
    expect(input.create).not.toHaveBeenCalled();
  });
});
