// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import type { Environment } from "@/generated/crds/environment";
import {
  EnvironmentEditor,
  type EnvironmentActions,
} from "./environment-editor";
import { environmentFixtures, sectionFixtures } from "./fixtures";
import type { EnvironmentSources } from "./sources";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
// The editor gets its writes as props. The page module of the server
// actions loads the writer, which a component test does not need.
vi.mock("@/app/setup/environments/actions", () => ({
  previewEnvironmentChange: vi.fn(),
  applyEnvironmentChange: vi.fn(),
  previewNamespaceLabel: vi.fn(),
  applyNamespaceLabel: vi.fn(),
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

function renderEditor(name: string, sources: EnvironmentSources) {
  const actions = {
    previewChange: vi.fn<EnvironmentActions["previewChange"]>(async () => ({
      objects: [],
      errors: [],
    })),
    applyChange: vi.fn<EnvironmentActions["applyChange"]>(async () => ({
      ok: true as const,
      mode: "direct" as const,
    })),
    previewLabel: vi.fn(async () => ({ objects: [], errors: [] })),
    applyLabel: vi.fn(async () => ({
      ok: true as const,
      mode: "direct" as const,
    })),
  };
  render(
    <GitPushesProvider>
      <EnvironmentEditor
        sources={{ ...sources, errors: [] }}
        name={name}
        mode="direct"
        actions={actions}
      />
    </GitPushesProvider>,
  );
  return actions;
}

// row returns the row of the section label in the group of the claim kind.
const row = (group: "Clusters" | "Management planes", label: string) =>
  within(screen.getByRole("region", { name: group })).getByRole("group", {
    name: label,
  });

async function save(actions: ReturnType<typeof renderEditor>) {
  fireEvent.click(screen.getByRole("button", { name: "Review" }));
  const confirm = screen.getByRole("button", { name: "Save" });
  await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
  fireEvent.click(confirm);
  await waitFor(() => expect(actions.applyChange).toHaveBeenCalledTimes(1));
  return actions.applyChange.mock.calls[0][0].apply[0]
    .spec as Environment["spec"];
}

afterEach(cleanup);

describe("SectionsTable", () => {
  it("shows Required, Default, and Allowed options for each row", () => {
    renderEditor("platform", environmentFixtures());
    const database = row("Management planes", "Management database");
    expect(
      within(database).getByRole("checkbox", { name: "Required" }),
    ).toHaveProperty("checked", true);
    expect(
      (within(database).getByLabelText("Default option") as HTMLSelectElement)
        .value,
    ).toBe("management-database-server");
    expect(
      within(database).getByRole("checkbox", {
        name: "PostgreSQL in the cluster",
      }),
    ).toHaveProperty("checked", true);
  });

  it("shows a section of both claim kinds in both groups", () => {
    renderEditor("platform", environmentFixtures());
    expect(row("Clusters", "Access")).toBeTruthy();
    expect(row("Management planes", "Access")).toBeTruthy();
  });

  it("checking Required hides the empty option and saves required", async () => {
    const actions = renderEditor("staging", environmentFixtures());
    const backups = row("Clusters", "Backup storage");
    expect(within(backups).getByText(/No backups/)).toBeTruthy();
    fireEvent.click(within(backups).getByRole("checkbox", { name: "RustFS" }));
    fireEvent.click(
      within(backups).getByRole("checkbox", { name: "Required" }),
    );
    expect(within(backups).queryByText(/No backups/)).toBeNull();
    expect(await save(actions)).toMatchObject({
      sections: { "backup-storage": { required: true, allowed: ["rustfs"] } },
    });
  });

  it("saves the default option", async () => {
    const actions = renderEditor("staging", environmentFixtures());
    const access = row("Clusters", "Access");
    fireEvent.click(
      within(access).getByRole("checkbox", { name: "Public gateway" }),
    );
    fireEvent.change(within(access).getByLabelText("Default option"), {
      target: { value: "gateway-public" },
    });
    expect(await save(actions)).toMatchObject({
      sections: {
        access: { allowed: ["gateway-public"], default: "gateway-public" },
      },
    });
  });

  it("a default that is not allowed blocks Review with its error", () => {
    const { previewChange } = renderEditor("platform", environmentFixtures());
    const access = row("Clusters", "Access");
    fireEvent.click(
      within(access).getByRole("checkbox", { name: "Internal gateway" }),
    );
    expect(
      within(access)
        .getByLabelText("Default option")
        .getAttribute("aria-invalid"),
    ).toBe("true");
    expect(
      within(access).getByText(
        "The default option must be one of the allowed options.",
      ),
    ).toBeTruthy();
    const review = screen.getByRole("button", { name: "Review" });
    expect(review).toHaveProperty("disabled", true);
    fireEvent.click(review);
    expect(previewChange).not.toHaveBeenCalled();
  });

  it("shows the Ready note and the input note at the option", () => {
    renderEditor("production", sectionFixtures());
    const key = within(row("Clusters", "Encryption key")).getByRole(
      "checkbox",
      { name: "AWS KMS key" },
    );
    expect(key.closest("li")?.textContent).toContain(
      "Not ready: AccessDenied on the key.",
    );
    const volumes = within(row("Clusters", "Volume encryption")).getByRole(
      "checkbox",
      { name: "Encrypted EBS volumes" },
    );
    expect(volumes.closest("li")?.textContent).toContain(
      "Needs an option of Encryption key. Set a default in Encryption key.",
    );
  });

  it("does not offer a registration that is not accepted", () => {
    renderEditor("production", sectionFixtures());
    expect(
      within(row("Clusters", "Encryption key")).queryByRole("checkbox", {
        name: "Vault transit key",
      }),
    ).toBeNull();
  });

  it("shows an allowed name without an offered option as Not offered and lets the admin clear it", async () => {
    const sources = environmentFixtures();
    sources.environments[1].spec.sections = {
      "encryption-key": { allowed: ["vault-transit"] },
    };
    const actions = renderEditor("staging", sources);
    const missing = within(row("Clusters", "Encryption key")).getByRole(
      "checkbox",
      { name: "vault-transit" },
    );
    expect(missing).toHaveProperty("checked", true);
    expect(missing.closest("li")?.textContent).toContain("Not offered");
    fireEvent.click(missing);
    const spec = await save(actions);
    expect(spec.sections).toBeUndefined();
  });

  it("shows at the row that a required section needs an allowed option, and blocks Review", () => {
    const { previewChange } = renderEditor("staging", environmentFixtures());
    const backups = row("Clusters", "Backup storage");
    fireEvent.click(
      within(backups).getByRole("checkbox", { name: "Required" }),
    );
    expect(
      within(backups).getByText(
        "A required section needs at least one allowed option.",
      ),
    ).toBeTruthy();
    const review = screen.getByRole("button", { name: "Review" });
    expect(review).toHaveProperty("disabled", true);
    fireEvent.click(review);
    expect(previewChange).not.toHaveBeenCalled();
  });

  it("names a section that does not exist and removes it", async () => {
    const sources = environmentFixtures();
    sources.environments[1].spec.sections = {
      gone: { allowed: ["rustfs"] },
    };
    const actions = renderEditor("staging", sources);
    const note = screen.getByRole("group", {
      name: "Sections that do not exist",
    });
    fireEvent.click(within(note).getByRole("button", { name: "Remove gone" }));
    const spec = await save(actions);
    expect(spec.sections).toBeUndefined();
  });
});
