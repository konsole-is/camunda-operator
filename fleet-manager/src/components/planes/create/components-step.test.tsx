// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import { useCreateFlow } from "@/components/create/use-create-flow";
import { EMPTY_ADMIN } from "@/lib/planes/components";
import {
  EMPTY_PLANE_DRAFT,
  type PlaneContext,
  type PlaneDraft,
  componentsErrors,
} from "@/lib/planes/draft";
import { KEYCLOAK_REALM_WARNING } from "@/lib/planes/versions";
import { ComponentsStep } from "./components-step";

afterEach(cleanup);

const ctx: PlaneContext = {
  environments: [
    {
      metadata: { name: "production" },
      spec: { releases: { default: "camunda-8-9-23" } },
    } as Environment,
  ],
  platformConfigs: [],
  takenNames: [],
  namespaces: [],
  releases: [
    {
      metadata: { name: "camunda-8-9-23" },
      spec: { version: "8.9.23" },
    } as CamundaRelease,
  ],
  images: {
    schemaVersion: 1,
    minors: {
      "8.9": {
        identity: { version: "8.9.10" },
        console: { version: "8.9.122" },
        "web-modeler-restapi": { version: "8.9.11" },
        keycloak: { tagPrefix: "quay-optimized-", version: "26.6.4" },
      },
    },
  },
};

const keycloak: PlaneDraft = {
  ...EMPTY_PLANE_DRAFT,
  environment: "production",
  name: "plane-a",
  namespace: { kind: "existing", name: "platform" },
  mode: "keycloak",
  admin: { ...EMPTY_ADMIN, username: "admin" },
};

function Harness({ initial }: { initial: PlaneDraft }) {
  const flow = useCreateFlow<PlaneDraft>({
    steps: [{ id: "components", label: "Components and versions" }],
    initial,
  });
  return (
    <>
      <ComponentsStep
        draft={flow.draft}
        set={flow.set}
        ctx={ctx}
        secrets={[{ name: "smtp", keys: ["username", "password"] }]}
      />
      <output data-testid="draft">{JSON.stringify(flow.draft)}</output>
    </>
  );
}

const draftOf = () =>
  JSON.parse(screen.getByTestId("draft").textContent ?? "{}") as PlaneDraft;
const field = (label: string) =>
  screen.getByLabelText(label) as HTMLInputElement;

describe("ComponentsStep", () => {
  it("in OIDC mode asks for the claim name and value and no username", () => {
    render(<Harness initial={{ ...keycloak, mode: "oidc" }} />);
    expect(field("Claim name")).toBeTruthy();
    expect(field("Claim value")).toBeTruthy();
    expect(screen.queryByLabelText("Username")).toBeNull();
    expect(screen.queryByRole("group", { name: "Password Secret" })).toBeNull();
  });

  it("in Keycloak mode asks for the username, the email, and the password Secret", async () => {
    render(<Harness initial={{ ...keycloak, admin: EMPTY_ADMIN }} />);
    await userEvent.type(field("Username"), "root");
    await userEvent.type(field("Email"), "root@example.com");
    expect(screen.getByRole("group", { name: "Password Secret" })).toBeTruthy();
    expect(screen.getByText("Generated if empty.")).toBeTruthy();
    expect(draftOf().admin).toMatchObject({
      username: "root",
      email: "root@example.com",
    });
    expect(screen.queryByLabelText("Claim name")).toBeNull();
  });

  it("prefills each version from the images table with the note Tested with release 8.9", async () => {
    render(<Harness initial={keycloak} />);
    expect(field("Management Identity version").value).toBe("8.9.10");
    expect(field("Keycloak version").value).toBe("26.6.4");
    expect(screen.queryByLabelText("Console version")).toBeNull();
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Run Console" }),
    );
    expect(field("Console version").value).toBe("8.9.122");
    expect(screen.getByText("Tested with release 8.9")).toBeTruthy();
  });

  it("keeps a typed version in the draft", async () => {
    render(<Harness initial={keycloak} />);
    await userEvent.clear(field("Management Identity version"));
    await userEvent.type(field("Management Identity version"), "8.9.12");
    expect(draftOf().versions).toEqual({ identity: "8.9.12" });
  });

  it("shows the Keycloak warning for 26.7.0 with Identity 8.9 and keeps the step open", async () => {
    render(<Harness initial={keycloak} />);
    await userEvent.clear(field("Keycloak version"));
    await userEvent.type(field("Keycloak version"), "26.7.0");
    expect(screen.getByRole("note").textContent).toContain(
      KEYCLOAK_REALM_WARNING,
    );
    expect(componentsErrors(draftOf(), ctx)).toEqual([]);
  });

  it("shows the SMTP fields only while Web Modeler is on", async () => {
    render(<Harness initial={keycloak} />);
    expect(screen.queryByLabelText("SMTP host")).toBeNull();
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Run Web Modeler" }),
    );
    expect(field("SMTP port").value).toBe("587");
    expect(
      (
        screen.getByRole("checkbox", { name: "STARTTLS" }) as HTMLElement
      ).getAttribute("aria-checked"),
    ).toBe("true");
    await userEvent.type(field("SMTP host"), "smtp.example.com");
    await userEvent.type(field("From address"), "modeler@example.com");
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Credentials Secret" }),
      "smtp",
    );
    expect(draftOf().mail).toMatchObject({
      smtpHost: "smtp.example.com",
      fromAddress: "modeler@example.com",
      credentialsSecret: "smtp",
    });
    expect(field("Web Modeler version").value).toBe("8.9.11");
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Run Web Modeler" }),
    );
    expect(screen.queryByLabelText("SMTP host")).toBeNull();
  });
});
