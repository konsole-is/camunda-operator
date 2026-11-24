// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyPlatformConfigChange,
  previewPlatformConfigChange,
} from "@/app/setup/platform/actions";
import { CHANGES_OFF } from "@/components/cluster/edit/cluster-edit";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "@/components/fleet/namespaced-secret-ref-picker";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import type { WriteMode } from "@/lib/write/types";
import { fakeWriter } from "../../../../test/fakes/writer";
import { createChange } from "./draft";
import { platformFixtures, twoMinorTable } from "./fixtures";
import { CREATE_EFFECT } from "./model";
import { PlatformCreate } from "./platform-create";
import type { PlatformSources } from "./sources";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));
vi.mock("@/lib/k8s/read", () => ({ list: vi.fn(async () => []) }));
const router = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("next/navigation", () => ({ useRouter: () => router }));
vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
  useResources: () => {
    throw new Error("the test reads Secrets through the static reader");
  },
  useWatchesFresh: () => [false],
}));

beforeEach(() => {
  fakeWriter.reset();
  router.push.mockClear();
});
afterEach(cleanup);

// camunda-system has no camunda-license, so a copy of broken names a
// Secret that does not exist.
const SECRETS = staticSecretRefs({
  "camunda-system": [
    { name: "oidc-credentials", keys: ["client-secret", "identity-secret"] },
    { name: "license", keys: ["license-key"] },
  ],
});

function setup(opts: { mode?: WriteMode } = {}) {
  const mode = "mode" in opts ? opts.mode : fakeWriter.mode;
  const view = (sources: PlatformSources) => (
    <GitPushesProvider>
      <SecretRefsReaderContext.Provider value={SECRETS}>
        <PlatformCreate
          sources={{ ...sources, errors: [], stale: [] }}
          imagesTable={twoMinorTable}
          mode={mode}
          actions={{
            previewChange: previewPlatformConfigChange,
            applyChange: applyPlatformConfigChange,
          }}
        />
      </SecretRefsReaderContext.Provider>
    </GitPushesProvider>
  );
  const { rerender } = render(view(platformFixtures()));
  return { rerender: (sources: PlatformSources) => rerender(view(sources)) };
}

const nameInput = () =>
  screen.getByLabelText("Name", { exact: true }) as HTMLInputElement;
const copyFrom = () =>
  screen.getByLabelText("Copy from", { exact: true }) as HTMLSelectElement;
const secretOf = (group: string) =>
  within(screen.getByRole("group", { name: group })).queryByLabelText(
    "Secret",
  ) as HTMLSelectElement | null;

// pick picks a Secret key in the first picker named group.
function pick(group: string, namespace: string, secret: string, key: string) {
  const picker = within(screen.getAllByRole("group", { name: group })[0]);
  fireEvent.change(picker.getByLabelText("Namespace"), {
    target: { value: namespace },
  });
  fireEvent.change(picker.getByLabelText("Secret"), {
    target: { value: secret },
  });
  fireEvent.change(picker.getByLabelText("Key"), { target: { value: key } });
}

// create opens the review and confirms it. The dry run of the fake writer
// answers with the object of the change, so the confirm turns on.
async function create() {
  fakeWriter.previews = [
    {
      objects: [
        {
          ref: { kind: "CamundaPlatformConfig", name: nameInput().value },
          diff: [{ op: "added", text: `name: ${nameInput().value}` }],
        } as never,
      ],
      errors: [],
    },
  ];
  await userEvent.click(screen.getByRole("button", { name: "Review" }));
  const review = screen.getByRole("region", {
    name: "Review the platform config",
  });
  const confirm = within(review).getByRole("button", { name: "Create" });
  await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
  await userEvent.click(confirm);
  return review;
}

const withConfig = (name: string): PlatformSources => {
  const sources = platformFixtures();
  const config: CamundaPlatformConfig = {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaPlatformConfig",
    metadata: { name },
    spec: {},
  };
  return { ...sources, platformConfigs: [...sources.platformConfigs, config] };
};

describe("PlatformCreate", () => {
  it("creates a basic-auth config by name with the license Secret", async () => {
    const { rerender } = setup();

    pick("License key", "camunda-system", "license", "license-key");
    await userEvent.type(nameInput(), "basic-two");
    const review = await create();

    expect(within(review).getByText(CREATE_EFFECT)).toBeDefined();
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    expect(fakeWriter.applied).toEqual([
      createChange("basic-two", {
        auth: { method: "basic" },
        licenseSecretRef: {
          namespace: "camunda-system",
          name: "license",
          key: "license-key",
        },
      }),
    ]);
    expect(
      await screen.findByText("The platform config basic-two is created."),
    ).toBeDefined();
    expect(router.push).not.toHaveBeenCalled();

    rerender(withConfig("basic-two"));
    await waitFor(() =>
      expect(router.push).toHaveBeenCalledWith("/setup/platform/basic-two"),
    );
  });

  it("creates an OIDC config with the client secret and a management plane client", async () => {
    setup();

    await userEvent.type(nameInput(), "oidc-two");
    await userEvent.click(screen.getByRole("radio", { name: "OIDC" }));
    await userEvent.type(
      screen.getByLabelText("Issuer URL"),
      "https://login.example.com/realms/camunda",
    );
    await userEvent.type(
      screen.getAllByLabelText("Client ID")[0],
      "orchestration",
    );
    pick(
      "Client secret",
      "camunda-system",
      "oidc-credentials",
      "client-secret",
    );
    await userEvent.type(
      within(screen.getByRole("group", { name: "Console" })).getByLabelText(
        "Client ID",
      ),
      "console",
    );
    await create();

    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    expect(fakeWriter.applied[0].create).toEqual([
      "CamundaPlatformConfig//oidc-two",
    ]);
    expect(fakeWriter.applied[0].apply[0].spec).toEqual({
      auth: {
        method: "oidc",
        oidc: {
          issuerUrl: "https://login.example.com/realms/camunda",
          clientId: "orchestration",
          clientSecretRef: {
            namespace: "camunda-system",
            name: "oidc-credentials",
            key: "client-secret",
          },
          management: { clients: { console: { clientId: "console" } } },
        },
      },
    });
  });

  it("creates a copy with its missing license Secret", async () => {
    setup();

    await userEvent.selectOptions(copyFrom(), "broken");

    expect(
      screen.getByText(
        "The Secret camunda-license does not exist in namespace camunda-system.",
      ),
    ).toBeDefined();
    const secret = secretOf("License key");
    expect(secret?.selectedOptions[0]?.textContent).toBe(
      "camunda-license (not found)",
    );

    await userEvent.type(nameInput(), "broken-copy");
    await create();

    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    expect(fakeWriter.applied[0].apply[0]).toEqual({
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaPlatformConfig",
      metadata: { name: "broken-copy" },
      spec: {
        auth: { method: "basic" },
        licenseSecretRef: {
          namespace: "camunda-system",
          name: "camunda-license",
          key: "license-key",
        },
      },
    });
    expect(fakeWriter.applied[0].preconditions).toBeUndefined();
  });

  it("keeps the typed name and replaces the fields when the copy source changes", async () => {
    setup();

    await userEvent.type(nameInput(), "next-config");
    await userEvent.selectOptions(copyFrom(), "oidc-prod");
    expect(
      (screen.getByLabelText("Issuer URL") as HTMLInputElement).value,
    ).toBe("https://login.example.com/realms/camunda");
    expect(secretOf("License key")?.value).toBe("camunda-license");

    await userEvent.selectOptions(copyFrom(), "basic-lab");

    expect(nameInput().value).toBe("next-config");
    expect(screen.getByRole("radio", { name: "Basic" })).toHaveProperty(
      "checked",
      true,
    );
    expect(screen.queryByLabelText("Issuer URL")).toBeNull();
    expect(secretOf("License key")).toBeNull();
  });

  it("writes no oidc block when a copy of an OIDC config switches to basic", async () => {
    setup();

    await userEvent.selectOptions(copyFrom(), "oidc-prod");
    await userEvent.type(nameInput(), "basic-copy");
    await userEvent.click(screen.getByRole("radio", { name: "Basic" }));
    await create();

    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    expect(fakeWriter.applied[0].apply[0].spec?.auth).toEqual({
      method: "basic",
    });
  });

  it("blocks a create with a name that exists or that a cluster cannot name", async () => {
    setup();

    const bar = () => screen.getByRole("region", { name: "Unsaved changes" });
    await userEvent.type(nameInput(), "oidc-prod");

    expect(
      within(bar()).getByText("A platform config with this name exists."),
    ).toBeDefined();
    expect(
      within(bar()).getByRole("button", { name: "Review" }),
    ).toHaveProperty("disabled", true);

    await userEvent.clear(nameInput());
    await userEvent.type(nameInput(), "team.a");

    expect(
      within(bar()).getByText(
        "The name has only lowercase letters, digits, and hyphens, and at most 253 characters.",
      ),
    ).toBeDefined();
  });

  it("stays on the page when the create fails", async () => {
    setup();
    fakeWriter.results = [
      {
        ok: false,
        errors: [
          {
            code: "Stale",
            message: "The CamundaPlatformConfig basic-two exists.",
          },
        ],
      },
    ];

    await userEvent.type(nameInput(), "basic-two");
    const review = await create();

    expect(
      await within(review).findByText(
        "The CamundaPlatformConfig basic-two exists.",
        { exact: false },
      ),
    ).toBeDefined();
    await userEvent.click(within(review).getByRole("button", { name: "Back" }));
    expect(router.push).not.toHaveBeenCalled();
    expect(nameInput().closest("fieldset:disabled")).toBeNull();
    expect(nameInput().value).toBe("basic-two");
  });

  it("disables the form while changes are off", () => {
    setup({ mode: undefined });

    expect(screen.getByText(CHANGES_OFF)).toBeDefined();
    expect(nameInput().closest("fieldset:disabled")).not.toBeNull();
    expect(copyFrom().closest("fieldset:disabled")).not.toBeNull();
    expect(screen.queryByRole("button", { name: "Review" })).toBeNull();
  });
});
