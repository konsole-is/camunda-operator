// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { ClusterWaiting } from "./cluster-waiting";

afterEach(cleanup);

describe("ClusterWaiting", () => {
  it("names each request that the claim waits for", () => {
    render(
      <ClusterWaiting
        unread={[]}
        waits={[
          {
            section: "secondary-storage",
            label: "Secondary storage",
            request: "orders-secondary-storage",
          },
          { section: "access", label: "Access" },
        ]}
      />,
    );
    const block = screen.getByRole("region", {
      name: "The cluster does not exist yet",
    });
    expect(
      within(block).getByText("The claim waits for the outputs of these requests:"),
    ).toBeTruthy();
    const list = within(block).getByRole("list", {
      name: "Requests the claim waits for",
    });
    expect(
      within(list)
        .getAllByRole("listitem")
        .map((li) => li.textContent),
    ).toEqual(["Secondary storage: orders-secondary-storage", "Access: Not made yet"]);
  });

  it("names the kinds it could not read, never a list of waits", () => {
    render(
      <ClusterWaiting
        unread={["PluginRequest"]}
        waits={[{ section: "access", label: "Access" }]}
      />,
    );
    expect(
      screen.getByText(
        "Not known: the fleet manager has no current list of PluginRequest.",
      ),
    ).toBeTruthy();
    expect(screen.queryByRole("list")).toBeNull();
  });

  it("says that the claim is being deleted after its cluster went", () => {
    render(
      <ClusterWaiting
        unread={["PluginRequest"]}
        waits={[{ section: "access", label: "Access" }]}
        deleting
      />,
    );
    const block = screen.getByRole("region", {
      name: "The cluster no longer exists",
    });
    expect(block.textContent).toBe(
      "The cluster no longer existsThe claim is being deleted.",
    );
    expect(screen.queryByText("The cluster does not exist yet")).toBeNull();
  });

  it("shows no list while the claim waits for no output", () => {
    render(<ClusterWaiting unread={[]} waits={[]} />);
    expect(
      screen.getByRole("region", { name: "The cluster does not exist yet" }),
    ).toBeTruthy();
    expect(screen.queryByRole("list")).toBeNull();
  });

  it("shows no list while the waits are not known", () => {
    render(<ClusterWaiting unread={[]} />);
    expect(screen.queryByRole("list")).toBeNull();
  });
});
