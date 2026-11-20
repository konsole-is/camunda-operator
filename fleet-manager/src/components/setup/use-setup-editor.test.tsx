// @vitest-environment jsdom
import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { REVISION_ANNOTATION } from "@/lib/labels";
import type { WriteResult } from "@/lib/write/types";
import { type SetupObject, useSetupEditor } from "./use-setup-editor";

const router = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));

afterEach(cleanup);
beforeEach(() => router.push.mockReset());

const hrefOf = (name: string, namespace?: string) =>
  `/setup/x/${namespace ?? "-"}/${name}`;

function object(
  kind: string,
  name: string,
  namespace: string | undefined,
  revision?: string,
): SetupObject {
  return {
    kind,
    metadata: {
      name,
      ...(namespace !== undefined && { namespace }),
      resourceVersion: "1",
      ...(revision && { annotations: { [REVISION_ANNOTATION]: revision } }),
    },
  };
}

function editor(live: SetupObject | undefined, isNew = false, listed = []) {
  return renderHook(
    (props: { live: SetupObject | undefined; listed: readonly string[] }) =>
      useSetupEditor({
        live: props.live,
        isNew,
        listed: props.listed,
        draftOf: (o) => o?.metadata.name ?? "",
        listHref: "/setup/x",
        hrefOf,
      }),
    {
      initialProps: { live, listed } as {
        live: SetupObject | undefined;
        listed: readonly string[];
      },
    },
  );
}

const pushed = (key: string): WriteResult => ({
  ok: true,
  mode: "git",
  commit: "c1",
  revisions: { [key as `${string}/${string}/${string}`]: "abc" },
  paths: {},
  deleted: {},
});

describe("useSetupEditor", () => {
  it("waits for the pushed revision of a namespaced object", () => {
    const live = object("ObjectStorageConfig", "backups", "team-a");
    const hook = editor(live);
    act(() =>
      hook.result.current.written(
        pushed("ObjectStorageConfig/team-a/backups"),
        "backups",
      ),
    );
    expect(hook.result.current.locked).toBe(true);

    hook.rerender({
      live: object("ObjectStorageConfig", "backups", "team-a", "abc"),
      listed: [],
    });
    expect(hook.result.current.locked).toBe(false);
  });

  it("keeps the cluster-scoped lookup of an object without namespace", () => {
    const live = object("Environment", "staging", undefined);
    const hook = editor(live);
    act(() =>
      hook.result.current.written(pushed("Environment//staging"), "staging"),
    );
    expect(hook.result.current.locked).toBe(true);

    hook.rerender({
      live: object("Environment", "staging", undefined, "abc"),
      listed: [],
    });
    expect(hook.result.current.locked).toBe(false);
  });

  it("moves to the page of a new object with its namespace", () => {
    const hook = editor(undefined, true);
    act(() =>
      hook.result.current.written(
        { ok: true, mode: "direct" },
        "backups",
        "team-a",
      ),
    );
    hook.rerender({ live: undefined, listed: ["backups"] });
    expect(router.push).toHaveBeenCalledWith("/setup/x/team-a/backups");
  });
});
