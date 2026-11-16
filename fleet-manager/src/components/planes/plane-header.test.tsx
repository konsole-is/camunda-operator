// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { PlaneHeader } from "./plane-header";

afterEach(cleanup);

const header = (props: Partial<ComponentProps<typeof PlaneHeader>> = {}) => (
  <PlaneHeader
    name="plane-a"
    namespace="platform"
    ready={{ status: "False", reason: "InvalidReference" }}
    mode="OIDC"
    platformConfig="oidc-platform"
    serves={{ environments: ["platform", "staging"] }}
    {...props}
  />
);

// fact returns the description of the term of that name.
function fact(term: string): string | null {
  const dt = screen.getByText(term, { selector: "dt" });
  return dt.nextElementSibling?.textContent ?? null;
}

describe("PlaneHeader", () => {
  it("shows the name, the Ready chip with its reason, the mode, the namespace, the platform config, and the served environments", () => {
    render(header());
    expect(
      screen.getByRole("heading", { level: 1, name: "plane-a" }),
    ).toBeTruthy();
    expect(screen.getByText("Not ready · InvalidReference")).toBeTruthy();
    expect(fact("Identity provider")).toBe("OIDC");
    expect(fact("Namespace")).toBe("platform");
    expect(fact("Platform config")).toBe("oidc-platform");
    expect(fact("Serves")).toBe("platform, staging");
  });

  it("shows the selector of a plane without a claim", () => {
    render(header({ serves: { selector: "team=a, !legacy" } }));
    expect(fact("Serves")).toBe("team=a, !legacy");
  });

  it("says that a plane without a claim and a cluster selector serves no cluster", () => {
    render(header({ serves: { selector: "team=a", noClusterSelector: true } }));
    expect(fact("Serves")).toBe(
      "No cluster, because it has no cluster selector",
    );
  });

  it("shows a dash for a mode and a platform config that are not known", () => {
    render(header({ mode: undefined, platformConfig: undefined }));
    expect(fact("Identity provider")).toBe("-");
    expect(fact("Platform config")).toBe("-");
  });

  it("shows Suspended for Ready with reason Suspended", () => {
    render(header({ ready: { status: "True", reason: "Suspended" } }));
    expect(screen.getByText("Ready · Suspended")).toBeTruthy();
  });

  it("renders actions", () => {
    render(header({ actions: <button type="button">Suspend</button> }));
    expect(screen.getByRole("button", { name: "Suspend" })).toBeTruthy();
  });
});
