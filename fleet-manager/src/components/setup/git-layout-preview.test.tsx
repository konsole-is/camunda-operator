// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { KindMap } from "@/lib/k8s/kinds";
import { GitLayoutPreview } from "./git-layout-preview";

afterEach(cleanup);

const kinds = {
  namespaced: KindMap.CamundaClusterClaim,
  clusterScoped: KindMap.Environment,
};

describe("GitLayoutPreview", () => {
  it("shows the file of a namespaced and of a cluster-scoped kind under git.path", () => {
    const { container } = render(
      <GitLayoutPreview path="clusters/prod" {...kinds} />,
    );
    expect(container.textContent).toContain(
      "clusters/prod/team-a/camundaclusterclaims/orders.yaml",
    );
    expect(container.textContent).toContain(
      "clusters/prod/_cluster/environments/production.yaml",
    );
  });

  it("puts the files at the root of the repository without git.path", () => {
    const { container } = render(<GitLayoutPreview path="" {...kinds} />);
    expect(container.textContent).toContain(
      "team-a/camundaclusterclaims/orders.yaml",
    );
    expect(container.textContent).not.toContain("/team-a/");
  });

  it("shows the error and no file for a path that names no folder", () => {
    const { container } = render(<GitLayoutPreview path="a/../b" {...kinds} />);
    expect(screen.getByRole("alert").textContent).toBe(
      "git.path must name a folder in the repository.",
    );
    expect(container.textContent).not.toContain(".yaml");
  });
});
