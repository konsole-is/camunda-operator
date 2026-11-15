// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import { slotContext } from "../../../test/fakes/slot-context";
import { DeleteAction } from "./delete-action";

afterEach(cleanup);

it("links a cluster with a claim to its delete page", () => {
  render(<DeleteAction context={slotContext()} />);
  expect(
    screen.getByRole("link", { name: "Delete" }).getAttribute("href"),
  ).toBe("/fleet/clusters/team-a/orders/delete");
});

it("shows no delete for a cluster without a claim", () => {
  const { container } = render(
    <DeleteAction context={slotContext({ claim: undefined })} />,
  );
  expect(container.textContent).toBe("");
});
