// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "@/components/fleet/namespaced-secret-ref-picker";
import { FakeEventSource } from "../../../../test/fakes/event-source";
import { ConnectionDetailLive } from "./connection-detail";
import { connectionFixtures } from "./fixtures";
import { CONNECTION_SOURCE_KINDS } from "./source-kinds";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

beforeEach(() => {
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("ConnectionDetailLive", () => {
  it("refuses Delete while the server has no current list of a user kind, and offers it again after the next snapshot", async () => {
    const initial = { ...connectionFixtures(), readErrors: [] };
    initial.databaseConfigs = [];
    render(
      <QueryClientProvider client={new QueryClient()}>
        <GitPushesProvider>
          <SecretRefsReaderContext.Provider value={staticSecretRefs({})}>
            <ConnectionDetailLive
              initial={initial}
              target={{
                kind: "DatabaseServerConfig",
                namespace: "team-a",
                name: "postgres",
              }}
              mode="direct"
              actions={{
                previewChange: vi.fn(async () => ({ objects: [], errors: [] })),
                applyChange: vi.fn(async () => ({
                  ok: true as const,
                  mode: "direct" as const,
                })),
              }}
            />
          </SecretRefsReaderContext.Provider>
        </GitPushesProvider>
      </QueryClientProvider>,
    );
    await waitFor(() =>
      expect(FakeEventSource.instances.at(-1)?.ids()).toHaveLength(
        Object.keys(CONNECTION_SOURCE_KINDS).length,
      ),
    );
    const source = FakeEventSource.instances.at(-1)!;
    const sendSnapshots = () => {
      for (const [key, kind] of Object.entries(CONNECTION_SOURCE_KINDS)) {
        source.emit("snapshot", {
          watch: new URLSearchParams({ kind }).toString(),
          kind,
          objects: initial[key as keyof typeof CONNECTION_SOURCE_KINDS],
        });
      }
    };
    sendSnapshots();
    await waitFor(() =>
      expect(screen.getByLabelText("Type postgres to confirm")).toBeTruthy(),
    );

    source.emit("failure", {
      watch: "kind=DatabaseServer",
      kind: "DatabaseServer",
      message: "The fleet manager has no current list of DatabaseServer",
    });

    await waitFor(() =>
      expect(
        screen.getByText(
          "Delete is possible when the fleet manager can read DatabaseServer.",
        ),
      ).toBeTruthy(),
    );
    expect(screen.queryByLabelText("Type postgres to confirm")).toBeNull();

    sendSnapshots();
    await waitFor(() =>
      expect(screen.getByLabelText("Type postgres to confirm")).toBeTruthy(),
    );
  });
});
