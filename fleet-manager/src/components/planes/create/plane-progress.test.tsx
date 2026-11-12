// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { CamundaManagementClusterClaim } from "@/generated/crds/camundamanagementclusterclaim";
import { PlaneProgress } from "./plane-progress";

const read = vi.hoisted(() => ({ error: undefined as string | undefined }));
vi.mock("@/lib/k8s/hooks", () => ({
  useResource: () => (read.error ? { error: read.error } : {}),
}));

afterEach(() => {
  cleanup();
  read.error = undefined;
});

const claim = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaManagementClusterClaim",
  metadata: { name: "plane-a", namespace: "platform" },
  spec: { environments: ["platform"], management: {}, versions: {} },
} as unknown as CamundaManagementClusterClaim;

describe("PlaneProgress", () => {
  it("shows the error of a failed read of the claim above the rows", () => {
    read.error = "camundamanagementclusterclaims is forbidden";
    render(<PlaneProgress claim={claim} sections={[]} />);
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read CamundaManagementClusterClaim: camundamanagementclusterclaims is forbidden",
    );
    expect(
      screen.getByRole("heading", {
        name: "Creating the management plane plane-a",
      }),
    ).toBeTruthy();
  });

  it("shows no alert while the read works", () => {
    render(<PlaneProgress claim={claim} sections={[]} />);
    expect(screen.queryByRole("alert")).toBeNull();
  });
});
