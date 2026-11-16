// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { releaseFixtures } from "./fixtures";
import { ReleaseEnvironments } from "./release-environments";

afterEach(cleanup);

describe("ReleaseEnvironments", () => {
  it("shows Default, Allowed, or Not allowed for each environment with a link to its page", () => {
    const { environments } = releaseFixtures();
    render(
      <ReleaseEnvironments
        release="camunda-8-9-0"
        environments={environments}
      />,
    );
    const card = within(screen.getByRole("region", { name: "Environments" }));
    const production = card.getByRole("link", { name: "Production" });
    expect(production.getAttribute("href")).toBe(
      "/setup/environments/production",
    );
    expect(
      within(production.closest("li") as HTMLElement).getByText("Default"),
    ).toBeTruthy();
    const staging = card.getByRole("link", { name: "Staging" });
    expect(
      within(staging.closest("li") as HTMLElement).getByText("Allowed"),
    ).toBeTruthy();
    cleanup();
    render(
      <ReleaseEnvironments
        release="camunda-8-9-1"
        environments={environments}
      />,
    );
    expect(screen.getAllByText("Not allowed")).toHaveLength(2);
  });

  it("says that no environment exists", () => {
    render(<ReleaseEnvironments release="camunda-8-9-0" environments={[]} />);
    expect(screen.getByText("No environment exists yet.")).toBeTruthy();
    expect(
      screen
        .getByRole("link", { name: /Environments page/ })
        .getAttribute("href"),
    ).toBe("/setup/environments");
  });
});
