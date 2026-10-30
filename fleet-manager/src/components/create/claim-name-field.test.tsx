// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ClaimNameField } from "./claim-name-field";

afterEach(cleanup);

describe("ClaimNameField", () => {
  it("shows a host line for each rendered host", () => {
    render(
      <ClaimNameField
        value="orders"
        errors={[]}
        hosts={[
          { parameter: "hostnamePattern", host: "orders.team-a.example.com" },
          { parameter: "grpcHostnamePattern", host: "grpc-orders.example.com" },
        ]}
      />,
    );
    expect(screen.getByText("orders.team-a.example.com")).toBeTruthy();
    expect(screen.getByText("grpc-orders.example.com")).toBeTruthy();
  });

  it("shows no host line when there is no host", () => {
    render(<ClaimNameField value="orders" errors={[]} hosts={[]} />);
    expect(screen.queryByText(/hosts/)).toBeNull();
  });

  it("shows the errors after the first edit only", async () => {
    const onChange = vi.fn();
    render(
      <ClaimNameField
        value=""
        errors={["Enter a name"]}
        hosts={[]}
        onChange={onChange}
      />,
    );
    const input = screen.getByRole("textbox", { name: "Name" });
    expect(screen.queryByText("Enter a name")).toBeNull();
    expect(input.getAttribute("aria-invalid")).toBeNull();
    await userEvent.type(input, "x");
    expect(onChange).toHaveBeenCalledWith("x");
    expect(input.getAttribute("aria-invalid")).toBe("true");
    const described = input.getAttribute("aria-describedby")!.split(" ");
    expect(
      described.map((id) => document.getElementById(id)?.textContent).join(),
    ).toContain("Enter a name");
  });
});
