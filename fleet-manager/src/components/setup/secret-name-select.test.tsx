// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SecretNameSelect } from "./secret-name-select";

afterEach(cleanup);

const options = () => screen.getAllByRole("option").map((o) => o.textContent);

describe("SecretNameSelect", () => {
  it("lists None and the Secret names, and nothing else of a Secret", () => {
    render(
      <SecretNameSelect
        label="Credentials Secret"
        value=""
        names={["git", "prom"]}
        namespace="fleet"
        onChange={() => {}}
      />,
    );
    expect(options()).toEqual(["None", "git", "prom"]);
    expect(
      (screen.getByLabelText("Credentials Secret") as HTMLSelectElement).value,
    ).toBe("");
  });

  it("keeps a name that does not exist and says so", () => {
    render(
      <SecretNameSelect
        label="Credentials Secret"
        value="git"
        names={["prom"]}
        namespace="fleet"
        onChange={() => {}}
      />,
    );
    expect(options()).toEqual(["None", "git (not found)", "prom"]);
    expect(
      (screen.getByLabelText("Credentials Secret") as HTMLSelectElement).value,
    ).toBe("git");
    expect(
      screen.getByText("The Secret git does not exist in namespace fleet."),
    ).toBeTruthy();
  });

  it("keeps the value and does not call it missing when the list is not known", () => {
    render(
      <SecretNameSelect
        label="Credentials Secret"
        value="git"
        namespace="fleet"
        onChange={() => {}}
      />,
    );
    expect(options()).toEqual(["None", "git"]);
    expect(
      (screen.getByLabelText("Credentials Secret") as HTMLSelectElement).value,
    ).toBe("git");
    expect(screen.queryByText(/does not exist/)).toBeNull();
  });

  it("gives the picked name to onChange", () => {
    const onChange = vi.fn();
    render(
      <SecretNameSelect
        label="Credentials Secret"
        value=""
        names={["git", "prom"]}
        namespace="fleet"
        onChange={onChange}
      />,
    );
    fireEvent.change(screen.getByLabelText("Credentials Secret"), {
      target: { value: "prom" },
    });
    expect(onChange).toHaveBeenCalledWith("prom");
  });
});
