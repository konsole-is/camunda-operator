// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { PermissionsTable } from "./permissions-table";

afterEach(cleanup);

const permissions = {
  rules: [
    {
      apiGroups: [""],
      resources: ["configmaps"],
      resourceNames: ["fleet-manager-settings"],
      verbs: ["get", "update"],
    },
    {
      apiGroups: ["fleet.camunda.io"],
      resources: ["environments", "camundaclusterclaims"],
      resourceNames: [],
      verbs: ["get", "list", "watch"],
    },
  ],
  incomplete: false,
};

describe("PermissionsTable", () => {
  it("shows one row per rule with its group, resources, names, and verbs", () => {
    render(<PermissionsTable permissions={permissions} />);
    const core = screen.getByRole("row", { name: /configmaps/ });
    expect(
      within(core)
        .getAllByRole("cell")
        .map((c) => c.textContent),
    ).toEqual(["core", "configmaps", "fleet-manager-settings", "get, update"]);
    const fleet = screen.getByRole("row", { name: /environments/ });
    expect(
      within(fleet)
        .getAllByRole("cell")
        .map((c) => c.textContent),
    ).toEqual([
      "fleet.camunda.io",
      "environments, camundaclusterclaims",
      "any",
      "get, list, watch",
    ]);
  });

  it("has no control, so the page cannot change the rules", () => {
    render(<PermissionsTable permissions={permissions} />);
    const table = screen.getByRole("table");
    expect(within(table).queryByRole("button")).toBeNull();
    expect(within(table).queryByRole("textbox")).toBeNull();
    expect(within(table).queryByRole("combobox")).toBeNull();
    expect(within(table).queryByRole("checkbox")).toBeNull();
  });

  it("says when the API server could not list every rule", () => {
    render(
      <PermissionsTable permissions={{ ...permissions, incomplete: true }} />,
    );
    expect(
      screen.getByText("The API server could not list every rule."),
    ).toBeTruthy();
  });

  it("shows the error of a read that failed", () => {
    render(<PermissionsTable permissions={{ error: "forbidden" }} />);
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read the permissions: forbidden",
    );
    expect(screen.queryByRole("table")).toBeNull();
  });
});
