// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import { ExternalKeycloakFields, IdentityCards } from "./identity-cards";

afterEach(cleanup);

const config = (spec: object): CamundaPlatformConfig =>
  ({ metadata: { name: "pc" }, spec }) as CamundaPlatformConfig;
const basicConfig = config({ auth: { method: "basic" } });
const oidcConfig = config({
  auth: { method: "oidc", oidc: { issuerUrl: "https://idp" } },
});

const radio = (name: RegExp) =>
  screen.getByRole("radio", { name }) as HTMLInputElement;

describe("IdentityCards", () => {
  it("offers the three modes in a group named for the sign-in", () => {
    render(<IdentityCards platformConfig={oidcConfig} onChange={vi.fn()} />);
    expect(
      screen.getByRole("radiogroup", { name: "Where users sign in" }),
    ).toBeTruthy();
    expect(
      screen.getAllByRole("radio").map((r) => r.closest("label")?.textContent),
    ).toEqual([
      expect.stringContaining("Keycloak, run by this plane"),
      expect.stringContaining("External Keycloak"),
      expect.stringContaining("OIDC from the platform config"),
    ]);
  });

  it("disables the OIDC card for a basic-auth platform config and says why", () => {
    render(<IdentityCards platformConfig={basicConfig} onChange={vi.fn()} />);
    expect(radio(/OIDC from the platform config/).disabled).toBe(true);
    expect(screen.getByText(/pc uses basic authentication/)).toBeTruthy();
  });

  it("disables the OIDC card for a platform config without an auth block", () => {
    render(<IdentityCards platformConfig={config({})} onChange={vi.fn()} />);
    expect(radio(/OIDC from the platform config/).disabled).toBe(true);
  });

  it("enables the OIDC card for an oidc platform config", () => {
    render(<IdentityCards platformConfig={oidcConfig} onChange={vi.fn()} />);
    expect(radio(/OIDC from the platform config/).disabled).toBe(false);
    expect(screen.queryByText(/basic authentication/)).toBeNull();
  });

  it("reports the picked mode", async () => {
    const onChange = vi.fn();
    render(<IdentityCards platformConfig={basicConfig} onChange={onChange} />);
    await userEvent.click(radio(/External Keycloak/));
    expect(onChange).toHaveBeenCalledWith("externalKeycloak");
  });

  it("checks the card of value", () => {
    render(
      <IdentityCards
        value="keycloak"
        platformConfig={basicConfig}
        onChange={vi.fn()}
      />,
    );
    expect(radio(/Keycloak, run by this plane/).checked).toBe(true);
  });
});

describe("ExternalKeycloakFields", () => {
  const value = { url: "", realm: "camunda-platform", adminSecret: "" };

  it("reports each field as the admin types", async () => {
    const onChange = vi.fn();
    render(
      <ExternalKeycloakFields
        value={value}
        secrets={[]}
        namespaceIsNew={false}
        onChange={onChange}
      />,
    );
    await userEvent.type(screen.getByLabelText("Keycloak URL"), "h");
    expect(onChange).toHaveBeenLastCalledWith({ ...value, url: "h" });
    await userEvent.type(
      screen.getByLabelText("Administrator credentials Secret"),
      "k",
    );
    expect(onChange).toHaveBeenLastCalledWith({ ...value, adminSecret: "k" });
  });

  it("prefills the realm camunda-platform", () => {
    render(
      <ExternalKeycloakFields
        value={value}
        secrets={[]}
        namespaceIsNew={false}
        onChange={vi.fn()}
      />,
    );
    expect((screen.getByLabelText("Realm") as HTMLInputElement).value).toBe(
      "camunda-platform",
    );
  });

  it("suggests the Secrets of the namespace for the credentials", () => {
    render(
      <ExternalKeycloakFields
        value={value}
        secrets={[{ name: "kc-admin", keys: ["username", "password"] }]}
        namespaceIsNew={false}
        onChange={vi.fn()}
      />,
    );
    const input = screen.getByLabelText("Administrator credentials Secret");
    const list = document.getElementById(input.getAttribute("list") ?? "");
    expect(
      [...(list?.querySelectorAll("option") ?? [])].map((o) => o.value),
    ).toEqual(["kc-admin"]);
  });

  it("says that a new namespace gets its CA bundle later", () => {
    render(
      <ExternalKeycloakFields
        value={value}
        secrets={[]}
        namespaceIsNew
        onChange={vi.fn()}
      />,
    );
    expect(screen.getByText(/No Secrets in this namespace/)).toBeTruthy();
    expect(screen.getByText(/Claim tab/)).toBeTruthy();
  });
});
