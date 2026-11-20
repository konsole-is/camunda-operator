// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  pluginRegistration,
  registrationEvent,
} from "@/components/setup/plugins/fixtures";
import { get, list } from "@/lib/k8s/read";
import RegistrationPage, { dynamic } from "./page";

const served = vi.hoisted(() => ({
  registration: undefined as unknown,
  events: [] as unknown[],
  eventsFail: false,
}));
vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
}));
vi.mock("@/lib/k8s/read", () => ({
  get: vi.fn(async () => served.registration),
  list: vi.fn(async (kind: string) => {
    if (kind === "PluginRegistration")
      return served.registration ? [served.registration] : [];
    if (kind !== "Event") return [];
    if (served.eventsFail) throw new Error("events are forbidden");
    return served.events;
  }),
}));
vi.mock("@/lib/bundled", () => ({ getBundledData: vi.fn() }));
// Each watch answers with its first snapshot, except Event while its
// server read fails.
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (_kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
  }),
  useWatchesFresh: (lists: { kind: string }[]) =>
    lists.map(({ kind }) => !(kind === "Event" && served.eventsFail)),
}));

afterEach(() => {
  cleanup();
  vi.mocked(list).mockClear();
  vi.mocked(get).mockClear();
  served.registration = undefined;
  served.events = [];
  served.eventsFail = false;
});

const params = (name: string, tab?: string) => ({
  params: Promise.resolve({ name }),
  searchParams: Promise.resolve(tab === undefined ? {} : { tab }),
});

const registration = {
  ...pluginRegistration("aws-kms", { label: "New KMS key" }),
  metadata: { name: "aws-kms", uid: "kms-1" },
};

describe("RegistrationPage", () => {
  it("shows not found for a registration that does not exist", async () => {
    await expect(RegistrationPage(params("gone"))).rejects.toThrow(
      "NEXT_NOT_FOUND",
    );
    expect(get).toHaveBeenCalledWith("PluginRegistration", undefined, "gone");
  });

  it("reads the Grants of the registration by its label", async () => {
    served.registration = registration;
    render(await RegistrationPage(params("aws-kms")));
    expect(list).toHaveBeenCalledWith("Grant", {
      labelSelector: "plugins.camunda.io/registration=aws-kms",
    });
    expect(dynamic).toBe("force-dynamic");
  });

  it("shows the events of the registration on the Health tab", async () => {
    served.registration = registration;
    served.events = [
      {
        ...registrationEvent(
          "aws-kms",
          "Warning",
          "CredentialsInvalid",
          "Role cannot be assumed",
        ),
        regarding: {
          kind: "PluginRegistration",
          name: "aws-kms",
          uid: "kms-1",
        },
      },
      {
        ...registrationEvent(
          "aws-kms",
          "Normal",
          "Old",
          "An older registration",
        ),
        regarding: {
          kind: "PluginRegistration",
          name: "aws-kms",
          uid: "kms-0",
        },
      },
    ];
    render(await RegistrationPage(params("aws-kms", "health")));
    expect(screen.getByText("Role cannot be assumed")).toBeTruthy();
    expect(screen.queryByText("An older registration")).toBeNull();
  });

  it("renders the page when the events cannot be read", async () => {
    served.registration = registration;
    served.eventsFail = true;
    render(await RegistrationPage(params("aws-kms", "health")));
    expect(
      screen.getByRole("heading", { level: 1, name: "New KMS key" }),
    ).toBeTruthy();
    expect(
      screen.getByText(
        "Not known: the fleet manager has no current list of Event.",
      ),
    ).toBeTruthy();
  });

  it("opens the About tab for an unknown tab", async () => {
    served.registration = registration;
    render(await RegistrationPage(params("aws-kms", "retain")));
    expect(
      screen.getByRole("tab", { name: "About" }).getAttribute("aria-selected"),
    ).toBe("true");
  });
});
