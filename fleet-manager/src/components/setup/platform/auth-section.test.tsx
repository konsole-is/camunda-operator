// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  SecretRefsReaderContext,
  staticSecretRefs,
} from "@/components/fleet/namespaced-secret-ref-picker";
import type { ValidationError } from "@/lib/schema";
import { AuthSection } from "./auth-section";
import { draftOf, type PlatformDraft } from "./draft";
import { platformFixtures } from "./fixtures";

afterEach(cleanup);

const SECRETS = staticSecretRefs({
  "camunda-system": [
    {
      name: "oidc-credentials",
      keys: ["client-secret", "identity-secret"],
    },
    { name: "camunda-license", keys: ["license-key"] },
  ],
});

const draftNamed = (name: string) =>
  draftOf(
    platformFixtures().platformConfigs.find((c) => c.metadata.name === name),
  );

function setup(draft: PlatformDraft, errors: ValidationError[] = []) {
  const onChange = vi.fn<(draft: PlatformDraft) => void>();
  render(
    <SecretRefsReaderContext.Provider value={SECRETS}>
      <AuthSection
        draft={draft}
        namespaces={["camunda-system", "team-a"]}
        errors={errors}
        onChange={onChange}
      />
    </SecretRefsReaderContext.Provider>,
  );
  return { onChange };
}

describe("AuthSection", () => {
  it("shows the OIDC fields and the management plane clients in OIDC mode", () => {
    setup(draftNamed("oidc-prod"));

    expect(
      (screen.getByLabelText("Issuer URL") as HTMLInputElement).value,
    ).toBe("https://login.example.com/realms/camunda");
    expect(
      (screen.getAllByLabelText("Client ID")[0] as HTMLInputElement).value,
    ).toBe("orchestration");
    for (const label of [
      "Provider type",
      "Audience",
      "Username claim",
      "Client ID claim",
      "JWKS URL",
      "Token URL",
      "Authorization URL",
    ])
      expect(screen.getAllByLabelText(label).length).toBeGreaterThan(0);
    for (const client of [
      "Management Identity",
      "Optimize",
      "Web Modeler",
      "Web Modeler API",
      "Console",
    ])
      expect(screen.getByRole("group", { name: client })).toBeDefined();
    expect(screen.getByLabelText("Public API audience")).toBeDefined();
    expect(screen.getByRole("group", { name: "License key" })).toBeDefined();
  });

  it("hides the OIDC fields and the clients in basic mode", () => {
    setup(draftNamed("basic-lab"));

    expect(screen.queryByLabelText("Issuer URL")).toBeNull();
    expect(screen.queryByText("Management plane clients")).toBeNull();
    expect(screen.getByRole("radio", { name: "Basic" })).toHaveProperty(
      "checked",
      true,
    );
    expect(screen.getByRole("group", { name: "License key" })).toBeDefined();
  });

  it("switches the method with the radio buttons", () => {
    const { onChange } = setup(draftNamed("oidc-prod"));

    fireEvent.click(screen.getByRole("radio", { name: "Basic" }));

    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        auth: expect.objectContaining({ method: "basic" }),
      }),
    );
  });

  it("edits an OIDC field", () => {
    const { onChange } = setup(draftNamed("oidc-prod"));

    fireEvent.change(screen.getByLabelText("Token URL"), {
      target: { value: "https://login.example.com/token" },
    });

    expect(onChange.mock.lastCall?.[0].auth?.oidc?.tokenUrl).toBe(
      "https://login.example.com/token",
    );
  });

  it("shows the error of a field next to it", () => {
    setup(draftNamed("oidc-prod"), [
      {
        path: "spec.auth.oidc.issuerUrl",
        message: "The issuer URL is an http or https URL.",
      },
    ]);

    const input = screen.getByLabelText("Issuer URL");
    expect(input.getAttribute("aria-invalid")).toBe("true");
    expect(
      document.getElementById(
        input.getAttribute("aria-describedby")!.split(" ").at(-1)!,
      )?.textContent,
    ).toBe("The issuer URL is an http or https URL.");
  });
});
