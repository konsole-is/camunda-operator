// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { PlaneClusters, PlaneName, PlaneServes } from "./cells";
import type { Plane } from "./plane";

afterEach(cleanup);

const plane = (fields: Partial<Plane> = {}): Plane => ({
  id: "platform/plane-a",
  namespace: "platform",
  name: "plane-a",
  managed: true,
  components: ["Identity"],
  serves: { environments: ["production", "staging"] },
  servedEnvironments: ["production", "staging"],
  attached: 2,
  notAttached: [],
  optimize: 0,
  ...fields,
});

describe("PlaneName", () => {
  it("shows the name and the namespace of a managed plane", () => {
    render(<PlaneName plane={plane()} />);
    expect(screen.getByText("plane-a")).toBeTruthy();
    expect(screen.getByText("platform")).toBeTruthy();
    expect(screen.queryByText("Not managed by the fleet manager")).toBeNull();
  });

  it("says when the fleet manager does not manage the plane", () => {
    render(<PlaneName plane={plane({ managed: false })} />);
    expect(screen.getByText("plane-a")).toBeTruthy();
    expect(screen.getByText("Not managed by the fleet manager")).toBeTruthy();
  });
});

describe("PlaneServes", () => {
  it("lists the environments of the claim", () => {
    render(<PlaneServes plane={plane()} />);
    expect(screen.getByText("production, staging")).toBeTruthy();
  });

  it("shows the raw selector of a plane without a claim", () => {
    render(
      <PlaneServes
        plane={plane({
          managed: false,
          serves: { selector: "team in (a,b),tier=prod" },
        })}
      />,
    );
    expect(screen.getByText("team in (a,b),tier=prod")).toBeTruthy();
  });
});

describe("PlaneClusters", () => {
  it("counts the attached clusters", () => {
    render(<PlaneClusters plane={plane()} />);
    expect(screen.getByText("2 attached")).toBeTruthy();
  });

  it("counts the clusters that are not attached too", () => {
    render(
      <PlaneClusters
        plane={plane({
          notAttached: [{ namespace: "team-a", name: "orders" }],
        })}
      />,
    );
    expect(screen.getByText("2 attached · 1 not attached")).toBeTruthy();
  });
});
