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
import { CLAIM_LABEL, ENVIRONMENT_LABEL } from "@/lib/labels";
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
import { changesWaitText, PlaneClaimEditProvider } from "./plane-claim-edit";
import { PlaneClaimEditor } from "./plane-claim-editor";
import { EXPRESSIONS_READ_ONLY } from "./served-panel";

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
        within(dialog()).getByText(
          "Another field manager keeps spec.platformConfigRef, so the reset does not remove it.",
        ),
      ).toBeTruthy(),
    );
  });

  it("keeps an open panel while the plane is not current, and offers no other Edit", () => {
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
