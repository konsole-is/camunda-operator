// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyPlaneClaimChange,
  previewPlaneClaimChange,
} from "@/app/fleet/planes/[namespace]/[name]/actions";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "@/components/fleet/namespaced-secret-ref-picker";
import { IDENTITY_INITIAL_CLAIM_ANNOTATION } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import {
  CLAIM_LABEL,
  ENVIRONMENT_LABEL,
  REVISION_ANNOTATION,
} from "@/lib/labels";
import {
  CALLBACKS_MOVE,
  CONSOLE_OFF,
  WEB_MODELER_OFF,
} from "@/lib/planes/impacts";
import type { PlaneClaim } from "@/lib/session";
import type { WriteMode } from "@/lib/write/types";
import { FakeEventSource } from "../../../../test/fakes/event-source";
import { fakeWriter } from "../../../../test/fakes/writer";
import type { PlanePageData } from "../plane-view";
import type { PlaneClaimTabData } from "./data";
import { RECORDED_PAIR } from "./identity-panel";
import {
  changesWaitText,
  DIRECT_MODE_FIELDS,
  FIRST_SYNC,
  KEPT_FIELDS_NOTE,
  NOTHING_CHANGES,
  PlaneClaimEditProvider,
  planeClaimContext,
} from "./plane-claim-edit";
import { PlaneClaimEditor } from "./plane-claim-editor";
import { EXPRESSIONS_READ_ONLY, NO_SELECTOR } from "./served-panel";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));

const actions = {
  preview: previewPlaneClaimChange,
  apply: applyPlaneClaimChange,
};
const NOW = "2026-11-16T12:00:00Z";

const basicConfig = {
  metadata: { name: "basic" },
  spec: { auth: { method: "basic" } },
} as Resource<"CamundaPlatformConfig">;
const ssoConfig = {
  metadata: { name: "sso" },
  spec: {
    auth: { method: "oidc", oidc: { issuerUrl: "https://sso.example.com" } },
  },
} as Resource<"CamundaPlatformConfig">;

function claimOf(spec: Partial<PlaneClaim["spec"]> = {}): PlaneClaim {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { name: "mgmt", namespace: "mgmt-ns", resourceVersion: "7" },
    spec: {
      environments: ["production"],
      clusterSelector: {},
      management: {
        identityProvider: {
          externalKeycloak: {
            url: "https://kc.example.com",
            adminCredentialsSecretRef: { name: "kc-admin" },
          },
        },
        identity: { admin: { username: "admin" } },
        console: {},
      },
      versions: { identity: "8.9.14", console: "8.9.133" },
      ...spec,
    },
  } as PlaneClaim;
}

const oidcClaim = (spec: Partial<PlaneClaim["spec"]> = {}) =>
  claimOf({
    platformConfigRef: "sso",
    management: {
      identityProvider: { oidc: {} },
      identity: { admin: { claimName: "sub", claimValue: "42" } },
      console: {},
    },
    ...spec,
  });

function planeOf(annotations: Record<string, string> = {}) {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: {
      name: "mgmt",
      namespace: "mgmt-ns",
      labels: { [CLAIM_LABEL]: "mgmt" },
      annotations,
    },
    spec: {
      identity: { externalUrl: "https://mgmt.example.com/identity" },
      console: { externalUrl: "https://mgmt.example.com/console" },
    },
  } as unknown as Resource<"CamundaManagementCluster">;
}

const cluster = (namespace: string, name: string) =>
  ({
    metadata: { namespace, name },
    spec: { platformConfigRef: "basic" },
    status: { gateway: { grpc: `${name}:26500` } },
  }) as unknown as Resource<"CamundaCluster">;

function dataOf(over: Partial<PlaneClaimTabData> = {}): PlanePageData {
  return {
    namespace: "mgmt-ns",
    name: "mgmt",
    now: NOW,
    namespaces: [],
    environments: [],
    optimizes: [],
    managementAuthConfigs: [],
    pods: [],
    platformConfigs: [],
    registrations: [],
    claimTab: {
      clusters: [cluster("team-orders", "orders"), cluster("team-qa", "qa")],
      platformConfigs: [basicConfig, ssoConfig],
      sections: [],
      registrations: [],
      requests: [],
      readErrors: [],
      ...over,
    },
  };
}

const namespaces = [
  {
    metadata: { name: "mgmt-ns", labels: { [ENVIRONMENT_LABEL]: "platform" } },
  },
  {
    metadata: {
      name: "team-orders",
      labels: { [ENVIRONMENT_LABEL]: "production" },
    },
  },
  { metadata: { name: "team-qa", labels: { [ENVIRONMENT_LABEL]: "staging" } } },
] as Resource<"Namespace">[];
const environments = [
  { metadata: { name: "platform" }, spec: { platformConfigRef: "basic" } },
  { metadata: { name: "production" }, spec: {} },
  { metadata: { name: "staging" }, spec: {} },
] as Resource<"Environment">[];

type Setup = {
  claim?: PlaneClaim;
  plane?: Resource<"CamundaManagementCluster"> | null;
  data?: PlanePageData;
  writeMode?: WriteMode;
  pageUnread?: string[];
};

function tree(setup: Setup) {
  const plane = setup.plane === null ? undefined : (setup.plane ?? planeOf());
  return (
    <QueryClientProvider client={client}>
      <GitPushesProvider>
        <SecretRefsReaderContext.Provider
          value={staticSecretRefs({
            "mgmt-ns": [
              { name: "kc-admin", keys: ["username", "password"] },
              { name: "admin-password", keys: ["password"] },
            ],
          })}
        >
          <PlaneClaimEditProvider
            data={setup.data ?? dataOf()}
            plane={plane}
            claim={"claim" in setup ? setup.claim : claimOf()}
            namespaces={namespaces}
            environments={environments}
            pageUnread={setup.pageUnread ?? []}
            writeMode={"writeMode" in setup ? setup.writeMode : "direct"}
            actions={actions}
          >
            <PlaneClaimEditor />
          </PlaneClaimEditProvider>
        </SecretRefsReaderContext.Provider>
      </GitPushesProvider>
    </QueryClientProvider>
  );
}

let client: QueryClient;
const editor = (setup: Setup = {}) => render(tree(setup));
const region = (name: string) => screen.getByRole("region", { name });
const open = (panel: string) =>
  fireEvent.click(screen.getByRole("button", { name: `Edit ${panel}` }));
const reviewButton = () =>
  within(region("Unsaved changes")).getByRole<HTMLButtonElement>("button", {
    name: "Review",
  });
const dialog = () => screen.getByRole("dialog", { name: "Review the change" });
const effects = () =>
  within(dialog()).getByRole("list", { name: "What this change does" });

beforeEach(() => {
  client = new QueryClient();
  fakeWriter.reset();
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("PlaneClaimEditor read view", () => {
  it("shows the name and namespace as set at creation, and the platform config from the environment", () => {
    editor();
    const basics = region("Basics");
    expect(within(basics).getAllByText("Set at creation")).toHaveLength(2);
    expect(within(basics).getByText("basic")).toBeTruthy();
    expect(within(basics).getByText("From the environment")).toBeTruthy();
  });

  it("shows the OIDC claim pair as fixed after Identity started", () => {
    editor({
      claim: oidcClaim(),
      plane: planeOf({ [IDENTITY_INITIAL_CLAIM_ANNOTATION]: "sub=42" }),
    });
    expect(
      within(region("Identity")).getAllByText("Fixed after Identity started"),
    ).toHaveLength(2);
  });

  it("shows the Databases and Access sections as chosen at creation, with the host and the URLs", () => {
    editor({
      claim: claimOf({
        plugins: [
          { section: "databases", registration: "postgres" },
          { section: "access", registration: "gateway" },
        ],
      }),
    });
    expect(
      within(region("Databases")).getByText("Chosen at creation"),
    ).toBeTruthy();
    const access = region("Access");
    expect(within(access).getByText("Chosen at creation")).toBeTruthy();
    expect(within(access).getByText("mgmt.example.com")).toBeTruthy();
    expect(
      within(access).getByText("https://mgmt.example.com/identity"),
    ).toBeTruthy();
    expect(
      within(access).getByText("https://mgmt.example.com/console"),
    ).toBeTruthy();
  });

  it("Show YAML shows the full claim", () => {
    editor({ claim: claimOf({ deletionPolicy: "Orphan" }) });
    fireEvent.click(screen.getByRole("button", { name: "Show YAML" }));
    const yaml = screen.getByRole("region", { name: "Claim YAML" });
    expect(yaml.textContent).toContain("kind: CamundaManagementClusterClaim");
    expect(yaml.textContent).toContain("deletionPolicy: Orphan");
  });

  it("a plane without a write mode offers no edit", () => {
    editor({ writeMode: undefined });
    expect(screen.queryByRole("button", { name: /^Edit/ })).toBeNull();
  });

  it("a claim-only plane opens the editor", () => {
    editor({ plane: null });
    expect(screen.getByRole("button", { name: "Edit Identity" })).toBeTruthy();
  });

  it("offers no edit while the page could not read the plane, and names it", () => {
    editor({ pageUnread: ["CamundaManagementCluster"] });
    expect(screen.queryByRole("button", { name: /^Edit/ })).toBeNull();
    expect(
      screen.getByText(changesWaitText(["CamundaManagementCluster"])),
    ).toBeTruthy();
  });
});

describe("PlaneClaimEditor identity", () => {
  it("a move of the identity mode lists the callback move and needs the plane name typed", async () => {
    editor({ claim: claimOf({ platformConfigRef: "sso" }) });
    open("Identity");
    fireEvent.click(
      screen.getByRole("radio", { name: /OIDC from the platform config/ }),
    );
    fireEvent.change(screen.getByLabelText("Claim name"), {
      target: { value: "sub" },
    });
    fireEvent.change(screen.getByLabelText("Claim value"), {
      target: { value: "42" },
    });
    fireEvent.click(reviewButton());
    expect(
      within(effects()).getByText(
        "Sign-in moves from External Keycloak to OIDC.",
      ),
    ).toBeTruthy();
    expect(within(effects()).getByText(CALLBACKS_MOVE)).toBeTruthy();
    const save = within(dialog()).getByRole<HTMLButtonElement>("button", {
      name: "Save",
    });
    expect(save.disabled).toBe(true);
    fireEvent.change(within(dialog()).getByLabelText(/to confirm/), {
      target: { value: "mgmt" },
    });
    await waitFor(() => expect(save.disabled).toBe(false));
    fireEvent.click(save);
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    const written = fakeWriter.applied[0].apply[0] as unknown as PlaneClaim;
    expect(written.spec.management.identityProvider).toEqual({ oidc: {} });
    expect(written.spec.management.identity.admin).toEqual({
      claimName: "sub",
      claimValue: "42",
    });
  });

  it("a move from Keycloak to OIDC asks for the claim pair and removes the password Secret", () => {
    editor({
      claim: claimOf({
        platformConfigRef: "sso",
        management: {
          identityProvider: { keycloak: {} },
          identity: {
            admin: {
              username: "admin",
              passwordSecretRef: { name: "admin-password", key: "password" },
            },
          },
        },
        versions: { identity: "8.9.14", keycloak: "26.6.4" },
      }),
    });
    open("Identity");
    fireEvent.click(
      screen.getByRole("radio", { name: /OIDC from the platform config/ }),
    );
    expect(
      (screen.getByLabelText("Claim name") as HTMLInputElement).required,
    ).toBe(true);
    expect(screen.queryByRole("group", { name: "Password Secret" })).toBeNull();
    expect(
      screen.getByText(
        "The first administrator stays. Management Identity keeps the one in its database.",
      ),
    ).toBeTruthy();
    expect(
      within(region("Unsaved changes")).getByText(
        /Fix \d+ fields? before review/,
      ),
    ).toBeTruthy();
  });

  it("a move from OIDC to a Keycloak mode removes the locked claim pair, and a move back restores it", () => {
    editor({
      claim: oidcClaim(),
      plane: planeOf({ [IDENTITY_INITIAL_CLAIM_ANNOTATION]: "sub=42" }),
    });
    open("Identity");
    expect(screen.queryByRole("textbox", { name: "Claim name" })).toBeNull();
    fireEvent.click(screen.getByRole("radio", { name: /External Keycloak/ }));
    expect((screen.getByLabelText("Username") as HTMLInputElement).value).toBe(
      "",
    );
    expect(
      within(region("Unsaved changes")).getByText(/unsaved change/),
    ).toBeTruthy();
    fireEvent.click(
      screen.getByRole("radio", { name: /OIDC from the platform config/ }),
    );
    expect(
      screen.queryByRole("region", { name: "Unsaved changes" }),
    ).toBeNull();
    expect(screen.getAllByText("Fixed after Identity started")).toHaveLength(2);
  });

  it("a move back to OIDC of a plane that started in OIDC mode takes the recorded claim pair", () => {
    editor({
      claim: claimOf({
        platformConfigRef: "sso",
        management: {
          identityProvider: { keycloak: {} },
          identity: { admin: { username: "admin" } },
        },
        versions: { identity: "8.9.14", keycloak: "26.6.4" },
      }),
      plane: planeOf({ [IDENTITY_INITIAL_CLAIM_ANNOTATION]: "sub=42" }),
    });
    open("Identity");
    fireEvent.click(
      screen.getByRole("radio", { name: /OIDC from the platform config/ }),
    );
    expect(screen.getByText(RECORDED_PAIR)).toBeTruthy();
    expect(screen.getByText("sub")).toBeTruthy();
    expect(screen.getByText("42")).toBeTruthy();
    expect(reviewButton().disabled).toBe(false);
    expect(within(region("Unsaved changes")).queryByText(/Fix/)).toBeNull();
  });

  it("a new URL of the External Keycloak needs the plane name typed", () => {
    editor();
    open("Identity");
    fireEvent.change(screen.getByLabelText("Keycloak URL"), {
      target: { value: "https://kc2.example.com" },
    });
    fireEvent.click(reviewButton());
    expect(within(dialog()).getByLabelText(/to confirm/)).toBeTruthy();
  });

  it("opening Identity with a password Secret that the namespace does not hold changes nothing", () => {
    editor({
      claim: claimOf({
        management: {
          identityProvider: { keycloak: {} },
          identity: {
            admin: {
              username: "admin",
              passwordSecretRef: { name: "gone", key: "password" },
            },
          },
        },
        versions: { identity: "8.9.14", keycloak: "26.6.4" },
      }),
    });
    open("Identity");
    expect(
      screen.queryByRole("region", { name: "Unsaved changes" }),
    ).toBeNull();
  });

  it("an untouched password picker follows a new reference in the live claim", () => {
    const keycloak = (key: string) =>
      claimOf({
        management: {
          identityProvider: { keycloak: {} },
          identity: {
            admin: {
              username: "admin",
              passwordSecretRef: { name: "kc-admin", key },
            },
          },
        },
        versions: { identity: "8.9.14", keycloak: "26.6.4" },
      });
    const { rerender } = editor({ claim: keycloak("username") });
    open("Identity");
    const picker = () =>
      within(screen.getByRole("group", { name: "Password Secret" }));
    expect((picker().getByLabelText("Key") as HTMLSelectElement).value).toBe(
      "username",
    );
    const updated = keycloak("password");
    updated.metadata.resourceVersion = "8";
    rerender(tree({ claim: updated }));
    expect((picker().getByLabelText("Key") as HTMLSelectElement).value).toBe(
      "password",
    );
  });

  it("a new administrator Secret of the External Keycloak drops the custom keys of the old one", async () => {
    const claim = claimOf();
    (
      claim.spec.management.identityProvider.externalKeycloak as {
        adminCredentialsSecretRef: object;
      }
    ).adminCredentialsSecretRef = {
      name: "old-admin",
      usernameKey: "user",
      passwordKey: "pass",
    };
    editor({ claim });
    open("Identity");
    fireEvent.change(
      screen.getByLabelText("Administrator credentials Secret"),
      {
        target: { value: "kc-admin" },
      },
    );
    fireEvent.click(reviewButton());
    await waitFor(() => expect(fakeWriter.previewed).toHaveLength(1));
    const sent = fakeWriter.previewed[0].apply[0] as unknown as PlaneClaim;
    expect(
      sent.spec.management.identityProvider.externalKeycloak
        ?.adminCredentialsSecretRef,
    ).toEqual({ name: "kc-admin" });
  });

  it("OIDC is not offered while the platform config uses basic authentication", () => {
    editor();
    open("Identity");
    expect(
      (
        screen.getByRole("radio", {
          name: /OIDC from the platform config/,
        }) as HTMLInputElement
      ).disabled,
    ).toBe(true);
    expect(screen.getByText(/basic uses basic authentication/)).toBeTruthy();
  });
});

describe("PlaneClaimEditor served clusters, components, and versions", () => {
  it("an environment change lists each cluster that attaches and each that detaches", () => {
    editor({ claim: claimOf({ platformConfigRef: "sso" }) });
    open("Environments served");
    fireEvent.click(screen.getByRole("checkbox", { name: "staging" }));
    expect(
      screen.getByText(
        "team-qa/qa attaches and restarts its pods once, because it gets the Console settings.",
      ),
    ).toBeTruthy();
    fireEvent.click(screen.getByRole("checkbox", { name: "production" }));
    expect(
      screen.getByText(
        "team-orders/orders detaches. It loses its Console settings and restarts once.",
      ),
    ).toBeTruthy();
  });

  it("a change of the labels keeps the match expressions", async () => {
    const expressions = [{ key: "tier", operator: "In", values: ["gold"] }];
    editor({
      claim: claimOf({ clusterSelector: { matchExpressions: expressions } }),
    });
    open("Environments served");
    expect(screen.getByText(EXPRESSIONS_READ_ONLY)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: /Add/ }));
    fireEvent.change(screen.getAllByLabelText(/^Label/)[0], {
      target: { value: "team" },
    });
    fireEvent.change(screen.getAllByLabelText(/^Value/)[0], {
      target: { value: "orders" },
    });
    fireEvent.click(reviewButton());
    await waitFor(() => expect(fakeWriter.previewed).toHaveLength(1));
    const sent = fakeWriter.previewed[0].apply[0] as unknown as PlaneClaim;
    expect(sent.spec.clusterSelector).toEqual({
      matchExpressions: expressions,
      matchLabels: { team: "orders" },
    });
  });

  it("the label editor follows a new selector in the live claim", () => {
    const { rerender } = editor();
    open("Environments served");
    expect(screen.queryAllByLabelText(/^Label/)).toHaveLength(0);
    const updated = claimOf({
      clusterSelector: { matchLabels: { team: "a" } },
    });
    updated.metadata.resourceVersion = "8";
    rerender(tree({ claim: updated }));
    expect(
      (screen.getAllByLabelText(/^Label/)[0] as HTMLInputElement).value,
    ).toBe("team");
  });

  it("an invalid label stops blocking Review once its editor closes", () => {
    editor();
    open("Environments served");
    fireEvent.click(screen.getByRole("button", { name: /Add/ }));
    fireEvent.change(screen.getAllByLabelText(/^Label/)[0], {
      target: { value: "not a label!" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Done" }));
    open("When the plane is deleted");
    fireEvent.click(screen.getByRole("radio", { name: /Keep the resources/ }));
    expect(reviewButton().disabled).toBe(false);
  });

  it("a blank label row keeps a claim without a cluster selector unchanged", () => {
    const claim = claimOf();
    delete (claim.spec as { clusterSelector?: unknown }).clusterSelector;
    editor({ claim });
    open("Environments served");
    fireEvent.click(screen.getByRole("button", { name: /Add/ }));
    expect(
      screen.queryByRole("region", { name: "Unsaved changes" }),
    ).toBeNull();
  });

  it("says that a claim without a cluster selector serves no cluster", () => {
    const claim = claimOf();
    delete (claim.spec as { clusterSelector?: unknown }).clusterSelector;
    editor({ claim });
    open("Environments served");
    expect(screen.getByText(NO_SELECTOR)).toBeTruthy();
  });

  it("Run Console turns on a Console whose block is null", () => {
    const claim = claimOf();
    (claim.spec.management as { console: unknown }).console = null;
    editor({ claim });
    open("Components");
    const box = screen.getByRole("checkbox", { name: "Run Console" });
    expect(box.getAttribute("aria-checked")).toBe("false");
    fireEvent.click(box);
    expect(
      screen
        .getByRole("checkbox", { name: "Run Console" })
        .getAttribute("aria-checked"),
    ).toBe("true");
  });

  it("a change of the SMTP host keeps the keys of the mail credentials", async () => {
    const claim = claimOf();
    claim.spec.management.webModeler = {
      mail: {
        smtpHost: "smtp",
        smtpPort: 587,
        fromAddress: "a@example.com",
        credentialsSecretRef: {
          name: "kc-admin",
          usernameKey: "username",
          passwordKey: "password",
        },
      },
    };
    claim.spec.management.identity.admin.email = "a@example.com";
    claim.spec.versions.webModeler = "8.9.15";
    editor({ claim });
    open("Components");
    fireEvent.change(screen.getByLabelText("SMTP host"), {
      target: { value: "smtp2" },
    });
    fireEvent.click(reviewButton());
    await waitFor(() => expect(fakeWriter.previewed).toHaveLength(1));
    const sent = fakeWriter.previewed[0].apply[0] as unknown as PlaneClaim;
    expect(sent.spec.management.webModeler?.mail).toEqual({
      ...claim.spec.management.webModeler.mail,
      smtpHost: "smtp2",
    });
  });

  it("a new mail credentials Secret drops the custom keys of the old one", async () => {
    const claim = claimOf();
    claim.spec.management.webModeler = {
      mail: {
        smtpHost: "smtp",
        smtpPort: 587,
        fromAddress: "a@example.com",
        credentialsSecretRef: {
          name: "old-smtp",
          usernameKey: "user",
          passwordKey: "pass",
        },
      },
    };
    claim.spec.management.identity.admin.email = "a@example.com";
    claim.spec.versions.webModeler = "8.9.15";
    editor({ claim });
    open("Components");
    fireEvent.change(screen.getByLabelText(/credentials/i), {
      target: { value: "kc-admin" },
    });
    fireEvent.click(reviewButton());
    await waitFor(() => expect(fakeWriter.previewed).toHaveLength(1));
    const sent = fakeWriter.previewed[0].apply[0] as unknown as PlaneClaim;
    expect(sent.spec.management.webModeler?.mail.credentialsSecretRef).toEqual({
      name: "kc-admin",
    });
  });

  it("Console off says that served clusters lose their Console settings and restart once", () => {
    editor();
    open("Components");
    fireEvent.click(screen.getByRole("checkbox", { name: "Run Console" }));
    expect(screen.getByText(CONSOLE_OFF)).toBeTruthy();
  });

  it("Web Modeler off says that its database stays", () => {
    const claim = claimOf();
    claim.spec.management.webModeler = {
      mail: { smtpHost: "smtp", smtpPort: 587, fromAddress: "a@example.com" },
    };
    claim.spec.management.identity.admin.email = "a@example.com";
    claim.spec.versions.webModeler = "8.9.15";
    editor({ claim });
    open("Components");
    fireEvent.click(screen.getByRole("checkbox", { name: "Run Web Modeler" }));
    expect(screen.getByText(WEB_MODELER_OFF)).toBeTruthy();
  });

  it("a version outside the Keycloak range shows its error and blocks Review", () => {
    editor({
      claim: claimOf({
        management: {
          identityProvider: { keycloak: {} },
          identity: { admin: { username: "admin" } },
        },
        versions: { identity: "8.9.14", keycloak: "26.6.4" },
      }),
    });
    open("Versions");
    fireEvent.change(screen.getByLabelText("Keycloak version"), {
      target: { value: "27.0.0" },
    });
    expect(
      screen.getByText(
        "Keycloak needs a version from 26.0.0 up to, but not including, 27.0.0.",
      ),
    ).toBeTruthy();
    expect(reviewButton().disabled).toBe(true);
  });

  it("a version that moves down shows a warning and does not block Review", () => {
    editor();
    open("Versions");
    fireEvent.change(screen.getByLabelText("Management Identity version"), {
      target: { value: "8.9.10" },
    });
    expect(
      screen.getByText(
        "Management Identity moves down from 8.9.14 to 8.9.10. Camunda documents no downgrade.",
      ),
    ).toBeTruthy();
    expect(reviewButton().disabled).toBe(false);
  });
});

describe("PlaneClaimEditor sections, deletion, and the platform config", () => {
  const registration = {
    metadata: { name: "postgres" },
    spec: {
      deletionPolicies: ["Delete", "Orphan"],
      parameters: {
        type: "object",
        properties: { size: { type: "string" } },
      },
    },
  } as unknown as Resource<"PluginRegistration">;

  it("a claim-editable parameter of a locked section stays editable", () => {
    const claim = claimOf({
      plugins: [
        {
          section: "databases",
          registration: "postgres",
          parameters: { size: "small" },
        },
      ],
    });
    (claim as { status?: unknown }).status = {
      resolvedPolicy: {
        parameters: { postgres: { size: { claimEditable: true } } },
      },
    };
    editor({ claim, data: dataOf({ registrations: [registration] }) });
    open("Databases");
    expect(screen.getByText("Chosen at creation")).toBeTruthy();
    const size = screen.getByDisplayValue("small");
    fireEvent.change(size, { target: { value: "large" } });
    expect(
      within(region("Unsaved changes")).getByText("1 unsaved change"),
    ).toBeTruthy();
  });

  it("a plane without access from outside edits the URL of each app", () => {
    editor();
    open("Access");
    fireEvent.change(screen.getByLabelText("Console URL"), {
      target: { value: "https://other.example.com/console" },
    });
    expect(
      within(region("Unsaved changes")).getByText("1 unsaved change"),
    ).toBeTruthy();
  });

  it("a move to the keycloak mode asks for the Keycloak URL of a plane without access", () => {
    editor();
    open("Identity");
    fireEvent.click(
      screen.getByRole("radio", { name: /Keycloak, run by this plane/ }),
    );
    expect(
      (screen.getByLabelText("Keycloak URL") as HTMLInputElement).required,
    ).toBe(true);
    expect(screen.getByText("Enter the Keycloak URL.")).toBeTruthy();
  });

  it("the deletion panel offers the deletion policy of a management plane", () => {
    editor();
    open("When the plane is deleted");
    expect(
      screen.getByRole("group", {
        name: "When the management plane is deleted",
      }),
    ).toBeTruthy();
  });

  describe("a selected registration that does not exist", () => {
    const recorded = () => {
      const claim = claimOf({
        plugins: [{ section: "databases", registration: "postgres" }],
      });
      (claim as { status?: unknown }).status = { resolvedPolicy: {} };
      return claim;
    };

    it("the deletion panel turns Keep off and names the option", () => {
      editor({ claim: recorded() });
      open("When the plane is deleted");
      expect(screen.getByText("Registration does not exist")).toBeTruthy();
      expect(
        screen.getByRole<HTMLInputElement>("radio", {
          name: /Keep the resources/,
        }).disabled,
      ).toBe(true);
    });

    it("the deletion panel does not say so while PluginRegistration is not read", () => {
      editor({
        claim: recorded(),
        data: dataOf({
          readErrors: [{ kind: "PluginRegistration", message: "forbidden" }],
        }),
      });
      open("When the plane is deleted");
      expect(screen.queryByText("Registration does not exist")).toBeNull();
    });

    it("planeClaimContext does not count a registration as read while PluginRegistration is unread", () => {
      const sources = {
        unread: ["PluginRegistration"],
        environments: [],
        platformConfigs: [],
        sections: [],
        registrations: [],
        requests: [],
      } as unknown as Parameters<typeof planeClaimContext>[0];
      expect(planeClaimContext(sources).registrationsRead).toBe(false);
      expect(
        planeClaimContext({ ...sources, unread: [] }).registrationsRead,
      ).toBe(true);
    });
  });

  it("a move of the platform config shows in the review with what restarts", () => {
    editor({ claim: claimOf({ platformConfigRef: "basic" }) });
    open("Basics");
    fireEvent.change(screen.getByLabelText("Platform config"), {
      target: { value: "sso" },
    });
    fireEvent.click(reviewButton());
    expect(
      within(effects()).getByText(
        "The platform config moves from basic to sso.",
      ),
    ).toBeTruthy();
    expect(
      within(effects()).getByText("Management Identity and Console restart."),
    ).toBeTruthy();
  });
});

describe("PlaneClaimEditor writes", () => {
  it("the save sends one claim change with the resourceVersion precondition, and needs no typed name", async () => {
    editor();
    open("When the plane is deleted");
    fireEvent.click(screen.getByRole("radio", { name: /Keep the resources/ }));
    fireEvent.click(reviewButton());
    expect(within(dialog()).queryByLabelText(/to confirm/)).toBeNull();
    const save = within(dialog()).getByRole<HTMLButtonElement>("button", {
      name: "Save",
    });
    await waitFor(() => expect(save.disabled).toBe(false));
    fireEvent.click(save);
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    const change = fakeWriter.applied[0];
    expect(change.apply).toHaveLength(1);
    expect(change.apply[0]).toMatchObject({
      kind: "CamundaManagementClusterClaim",
      metadata: { name: "mgmt", namespace: "mgmt-ns" },
      spec: { deletionPolicy: "Orphan" },
    });
    expect(change.delete ?? []).toEqual([]);
    expect(change.create ?? []).toEqual([]);
    expect(change.preconditions).toEqual({
      "CamundaManagementClusterClaim/mgmt-ns/mgmt": "7",
    });
  });

  it("a stale save writes nothing more and says that the claim changed", async () => {
    fakeWriter.results = [
      { ok: false, errors: [{ code: "Stale", message: "conflict" }] },
    ];
    editor();
    open("When the plane is deleted");
    fireEvent.click(screen.getByRole("radio", { name: /Keep the resources/ }));
    fireEvent.click(reviewButton());
    const save = within(dialog()).getByRole<HTMLButtonElement>("button", {
      name: "Save",
    });
    await waitFor(() => expect(save.disabled).toBe(false));
    fireEvent.click(save);
    await waitFor(() =>
      expect(
        within(dialog()).getByText(
          "The claim changed since you opened it. Reload to see the new version.",
        ),
      ).toBeTruthy(),
    );
    expect(save.disabled).toBe(true);
    fireEvent.click(save);
    expect(fakeWriter.applied).toHaveLength(1);
  });

  it("the review says when another field manager keeps a field that the change clears", async () => {
    const claim = claimOf({ platformConfigRef: "basic" });
    fakeWriter.previews = [
      {
        objects: [
          {
            ref: {
              kind: "CamundaManagementClusterClaim",
              namespace: "mgmt-ns",
              name: "mgmt",
            },
            before: claim,
            after: claim,
          } as never,
        ],
        errors: [],
        // The direct writer names the field of the other tool.
        ownership: [
          {
            ref: {
              apiVersion: "fleet.camunda.io/v1alpha1",
              kind: "CamundaManagementClusterClaim",
              namespace: "mgmt-ns",
              name: "mgmt",
            },
            changed: [],
            cleared: [
              {
                path: "spec.platformConfigRef",
                steps: [{ field: "spec" }, { field: "platformConfigRef" }],
                managers: [
                  {
                    name: "kubectl",
                    operation: "Apply",
                    kind: "other",
                    known: true,
                  },
                ],
              },
            ],
          },
        ],
      },
    ];
    editor({ claim });
    open("Basics");
    fireEvent.change(screen.getByLabelText("Platform config"), {
      target: { value: "" },
    });
    fireEvent.click(reviewButton());
    await waitFor(() =>
      expect(
        within(dialog()).getByRole("group", {
          name: "Fields that another tool set",
        }).textContent,
      ).toContain(
        "spec.platformConfigRef: kubectl set it. It stays unless you take it over.",
      ),
    );
  });

  it("keeps an open panel while the read of the plane failed, and offers no other Edit", () => {
    const { rerender } = editor();
    open("Identity");
    const radio = screen.getByRole("radio", { name: /External Keycloak/ });
    rerender(tree({ pageUnread: ["CamundaManagementCluster"] }));
    expect(screen.getByRole("radio", { name: /External Keycloak/ })).toBe(
      radio,
    );
    expect(screen.queryByRole("button", { name: /^Edit/ })).toBeNull();
    rerender(tree({}));
    expect(screen.getByRole("radio", { name: /External Keycloak/ })).toBe(
      radio,
    );
    expect(screen.getByRole("button", { name: "Edit Basics" })).toBeTruthy();
  });

  it("the review offers to take over a field that kubectl set, and Save stays off until it is chosen", async () => {
    const claim = oidcClaim();
    const ref = {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaManagementClusterClaim",
      namespace: "mgmt-ns",
      name: "mgmt",
    };
    const kubectl = {
      name: "kubectl",
      operation: "Apply",
      kind: "other" as const,
      known: true,
    };
    const fieldAt = (...names: string[]) => ({
      path: names.join("."),
      steps: names.map((field) => ({ field })),
      managers: [kubectl],
    });
    fakeWriter.previews = [
      {
        objects: [],
        errors: [
          {
            code: "Conflict",
            message: 'conflict with "kubectl"',
            path: "spec.management.identity.admin.username",
            manager: "kubectl",
            ref,
          },
        ],
        ownership: [
          {
            ref,
            changed: [
              fieldAt("spec", "management", "identity", "admin", "username"),
            ],
            cleared: [
              fieldAt("spec", "management", "identityProvider", "oidc"),
            ],
          },
        ],
      },
      { objects: [], errors: [] },
    ];
    editor({ claim });
    open("Identity");
    fireEvent.click(screen.getByRole("radio", { name: /External Keycloak/ }));
    fireEvent.change(screen.getByLabelText("Keycloak URL"), {
      target: { value: "https://kc.example.com" },
    });
    fireEvent.change(
      screen.getByLabelText("Administrator credentials Secret"),
      { target: { value: "kc-admin" } },
    );
    fireEvent.change(screen.getByLabelText("Username"), {
      target: { value: "admin" },
    });
    fireEvent.click(reviewButton());
    const group = await within(dialog()).findByRole("group", {
      name: "Fields that another tool set",
    });
    expect(group.textContent).toContain(
      "spec.management.identity.admin.username: kubectl set it. The save changes it.",
    );
    expect(group.textContent).toContain(
      "spec.management.identityProvider.oidc: kubectl set it. It stays unless you take it over.",
    );
    // The panel names the fields, so no other note lists them.
    expect(within(dialog()).queryByText(KEPT_FIELDS_NOTE)).toBeNull();
    const save = within(dialog()).getByRole<HTMLButtonElement>("button", {
      name: "Save",
    });
    expect(save.disabled).toBe(true);

    fireEvent.click(
      within(group).getByRole("checkbox", { name: "Take over these fields" }),
    );
    // A move of the sign-in asks for the name of the plane.
    fireEvent.change(within(dialog()).getByLabelText(/to confirm/), {
      target: { value: "mgmt" },
    });
    await waitFor(() => expect(save.disabled).toBe(false));
    fireEvent.click(save);
    await waitFor(() => expect(fakeWriter.applied).toHaveLength(1));
    const key = "CamundaManagementClusterClaim/mgmt-ns/mgmt";
    expect(fakeWriter.applied[0].takeOver).toEqual({
      [key]: ["spec.management.identity.admin.username"],
    });
    expect(fakeWriter.applied[0].remove).toEqual({
      [key]: ["spec.management.identityProvider.oidc"],
    });
  });

  it("says that a save changes nothing while kubectl keeps the list item, until the take-over", async () => {
    const claim = claimOf({ environments: ["production", "staging"] });
    (claim.metadata as { managedFields?: unknown }).managedFields = [
      {
        manager: "kubectl",
        operation: "Apply",
        fieldsV1: {
          "f:spec": {
            "f:environments": { 'v:"production"': {}, 'v:"staging"': {} },
          },
        },
      },
      {
        manager: "fleet-manager",
        operation: "Apply",
        fieldsV1: {
          "f:spec": {
            "f:environments": { 'v:"production"': {}, 'v:"staging"': {} },
          },
        },
      },
    ];
    const ref = {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "CamundaManagementClusterClaim",
      namespace: "mgmt-ns",
      name: "mgmt",
    };
    const ownership = [
      {
        ref,
        changed: [],
        cleared: [
          {
            path: 'spec.environments[="staging"]',
            steps: [
              { field: "spec" },
              { field: "environments" },
              { value: "staging" },
            ],
            managers: [
              {
                name: "kubectl",
                operation: "Apply",
                kind: "other" as const,
                known: true,
              },
            ],
          },
        ],
      },
    ];
    fakeWriter.previews = [
      { objects: [], errors: [], ownership },
      { objects: [], errors: [], ownership },
    ];
    editor({ claim });
    open("Environments served");
    fireEvent.click(screen.getByRole("checkbox", { name: "staging" }));
    fireEvent.click(reviewButton());
    const group = await within(dialog()).findByRole("group", {
      name: "Fields that another tool set",
    });
    const caution = within(dialog()).getByRole("note", { name: "Caution" });
    expect(caution.textContent).toBe(NOTHING_CHANGES);
    expect(within(dialog()).queryByText(/detaches/)).toBeNull();

    fireEvent.click(
      within(group).getByRole("checkbox", { name: "Take over these fields" }),
    );
    await waitFor(() =>
      expect(
        within(dialog()).queryByRole("note", { name: "Caution" }),
      ).toBeNull(),
    );
    expect(
      within(dialog()).getByText(
        "team-qa/qa detaches. It loses its Console settings and restarts once.",
      ),
    ).toBeTruthy();
  });

  it("in Git mode lets the GitOps tool remove a list item that only it set", () => {
    const claim = claimOf({
      platformConfigRef: "sso",
      environments: ["production", "staging"],
    });
    (claim.metadata as { managedFields?: unknown }).managedFields = [
      {
        manager: "argocd-controller",
        operation: "Apply",
        fieldsV1: {
          "f:metadata": {
            "f:annotations": { [`f:${REVISION_ANNOTATION}`]: {} },
          },
          "f:spec": {
            "f:environments": { 'v:"production"': {}, 'v:"staging"': {} },
          },
        },
      },
    ];
    editor({ claim, writeMode: "git" });
    open("Environments served");
    fireEvent.click(screen.getByRole("checkbox", { name: "staging" }));
    expect(
      screen.getByText(
        "team-qa/qa detaches. It loses its Console settings and restarts once.",
      ),
    ).toBeTruthy();
  });

  it("in Git mode before the first sync names the fields of the fleet manager and says what the sync can do", () => {
    const claim = claimOf({ environments: ["production", "staging"] });
    (claim.metadata as { managedFields?: unknown }).managedFields = [
      {
        manager: "fleet-manager",
        operation: "Apply",
        fieldsV1: {
          "f:spec": {
            "f:environments": { 'v:"production"': {}, 'v:"staging"': {} },
          },
        },
      },
    ];
    editor({ claim, writeMode: "git" });
    open("Environments served");
    fireEvent.click(screen.getByRole("checkbox", { name: "staging" }));
    fireEvent.click(reviewButton());
    const caution = within(dialog()).getByRole("note", { name: "Caution" });
    expect(caution.textContent).toContain(
      'spec.environments[="staging"] (fleet-manager)',
    );
    expect(caution.textContent).toContain(DIRECT_MODE_FIELDS);
    expect(caution.textContent).toContain(FIRST_SYNC);
  });

  it("closes an open review and turns Review off while a list that the checks read is unread", () => {
    const { rerender } = editor();
    open("When the plane is deleted");
    fireEvent.click(screen.getByRole("radio", { name: /Keep the resources/ }));
    fireEvent.click(reviewButton());
    expect(dialog()).toBeTruthy();
    rerender(
      tree({
        data: dataOf({
          readErrors: [{ kind: "PluginSection", message: "forbidden" }],
        }),
      }),
    );
    expect(
      screen.queryByRole("dialog", { name: "Review the change" }),
    ).toBeNull();
    expect(reviewButton().disabled).toBe(true);
    expect(
      within(region("Unsaved changes")).getByText(
        changesWaitText(["PluginSection"]),
      ),
    ).toBeTruthy();
  });
});
