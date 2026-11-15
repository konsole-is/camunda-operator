// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { platformFixtures } from "./fixtures";
import { usersOf } from "./model";
import { UsersSection } from "./users-section";

afterEach(cleanup);

const renderUsers = (name: string) =>
  render(<UsersSection users={usersOf(name, platformFixtures())} />);

const listText = (name: string) =>
  within(screen.getByRole("list", { name }))
    .getAllByRole("listitem")
    .map((li) => li.textContent);

describe("UsersSection", () => {
  it("lists database servers and environments as users, with the preset of a server", () => {
    renderUsers("oidc-prod");

    expect(listText("Database servers")).toEqual([
      "team-a/orders-db from the preset pg-standard",
    ]);
    expect(listText("Environments that default to it")).toEqual(["production"]);
    expect(listText("Management planes")).toEqual(["platform/main"]);
    expect(
      listText("Claims without a cluster or management plane yet"),
    ).toEqual(["team-b/billing"]);
  });

  it("links clusters to their page and environments to their editor", () => {
    renderUsers("oidc-prod");

    expect(
      screen.getByRole("link", { name: "team-a/orders" }).getAttribute("href"),
    ).toBe("/fleet/clusters/team-a/orders");
    expect(
      screen.getByRole("link", { name: "production" }).getAttribute("href"),
    ).toBe("/setup/environments/production");
  });

  it("shows only the groups that have users", () => {
    renderUsers("broken");

    expect(screen.queryByRole("list", { name: "Clusters" })).toBeNull();
    expect(listText("Environments that default to it")).toEqual(["staging"]);
  });

  it("says when nothing uses the config", () => {
    renderUsers("basic-lab");

    expect(screen.getByText("Nothing uses this config.")).toBeDefined();
    expect(screen.queryByRole("list")).toBeNull();
  });
});
