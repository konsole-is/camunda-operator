// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { PlaneField } from "@/lib/fields";
import { PlaneSettings } from "./plane-settings";

afterEach(cleanup);

const field = (over: Partial<PlaneField> & { key: string }): PlaneField => ({
  group: "identity",
  label: over.key,
  value: undefined,
  claimPath: over.key,
  claimSteps: [],
  ...over,
});

// valueOf returns the description of the term of that name.
function valueOf(term: string): HTMLElement {
  const dt = screen.getByText(term, { selector: "dt" });
  return dt.nextElementSibling as HTMLElement;
}

describe("PlaneSettings", () => {
  it("shows each field with its value and the lock label", () => {
    render(
      <PlaneSettings
        fields={[
          field({
            key: "admin.claimName",
            label: "Administrator claim name",
            value: "email",
            lock: { reason: "immutableAfterStart" },
          }),
        ]}
      />,
    );
    const value = valueOf("Administrator claim name");
    expect(value.textContent).toContain("email");
    expect(value.textContent).toContain("Fixed after Identity started");
  });

  it("shows the note of a field without a lock", () => {
    render(
      <PlaneSettings
        fields={[
          field({
            key: "admin.username",
            label: "Administrator username",
            value: "admin",
            note: "A new username creates a second user.",
          }),
        ]}
      />,
    );
    expect(valueOf("Administrator username").textContent).toContain(
      "A new username creates a second user.",
    );
  });

  it("shows On and Off for a component switch, the mode by name, and each key of an object value", () => {
    render(
      <PlaneSettings
        fields={[
          field({ key: "console", label: "Console", value: true }),
          field({ key: "webModeler", label: "Web Modeler", value: false }),
          field({
            key: "identityProvider",
            label: "Identity provider",
            value: "oidc",
          }),
          field({
            key: "webModeler.mail",
            label: "Web Modeler mail",
            value: { fromAddress: "modeler@example.com", smtpHost: "smtp" },
          }),
        ]}
      />,
    );
    expect(valueOf("Console").textContent).toBe("On");
    expect(valueOf("Web Modeler").textContent).toBe("Off");
    expect(valueOf("Identity provider").textContent).toBe("OIDC");
    const mail = valueOf("Web Modeler mail").textContent;
    expect(mail).toContain("fromAddress: modeler@example.com");
    expect(mail).toContain("smtpHost: smtp");
  });

  it("shows a dash for a field without a value, and says when no field applies", () => {
    const { rerender } = render(
      <PlaneSettings
        fields={[field({ key: "url.console", label: "Console URL" })]}
      />,
    );
    expect(valueOf("Console URL").textContent).toBe("-");
    rerender(<PlaneSettings fields={[]} />);
    expect(screen.getByText("No settings.")).toBeTruthy();
  });
});
