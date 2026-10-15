// @vitest-environment jsdom
import { useQueryClient } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { QueryProvider } from "./query-provider";

afterEach(cleanup);

function UsesClient() {
  useQueryClient();
  return <p>has a client</p>;
}

describe("QueryProvider", () => {
  it("gives its children a query client", () => {
    render(
      <QueryProvider>
        <UsesClient />
      </QueryProvider>,
    );
    expect(screen.getByText("has a client")).toBeTruthy();
  });
});
