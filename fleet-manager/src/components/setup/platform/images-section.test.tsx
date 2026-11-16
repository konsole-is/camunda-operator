// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { defaultRepositories } from "@/lib/merge";
import type { ValidationError } from "@/lib/schema";
import { twoMinorTable } from "./fixtures";
import { ImagesSection } from "./images-section";
import type { PlatformDraft } from "./draft";

afterEach(cleanup);

function setup(
  minors: string[],
  images: PlatformDraft["images"] = {},
  errors: ValidationError[] = [],
  usersKnown = true,
) {
  const onChange = vi.fn<(images: PlatformDraft["images"]) => void>();
  render(
    <ImagesSection
      images={images}
      defaults={defaultRepositories(twoMinorTable, minors)}
      errors={errors}
      usersKnown={usersKnown}
      onChange={onChange}
    />,
  );
  return { onChange };
}

const defaultsOf = (label: string) =>
  Array.from(
    screen
      .getByRole("list", { name: `Defaults of ${label}` })
      .querySelectorAll("li"),
    (li) => li.textContent,
  );

describe("ImagesSection", () => {
  it("shows each image field with its repository and one default for one minor", () => {
    setup(["8.9"], { camunda: "registry.example.com/camunda/camunda" });

    expect(
      (screen.getByLabelText("Orchestration cluster") as HTMLInputElement)
        .value,
    ).toBe("registry.example.com/camunda/camunda");
    expect(defaultsOf("Web Modeler REST API")).toEqual([
      "camunda/web-modeler-restapi",
    ]);
    expect(
      (screen.getByLabelText("Optimize") as HTMLInputElement).placeholder,
    ).toBe("camunda/optimize");
  });

  it("shows each default with its version range when users run two minors", () => {
    setup(["8.9", "8.10"]);

    expect(defaultsOf("Web Modeler REST API")).toEqual([
      "camunda/web-modeler-restapi for Camunda 8.9",
      "camunda/hub for Camunda 8.10",
    ]);
    expect(defaultsOf("Orchestration cluster")).toEqual([
      "camunda/camunda for Camunda 8.9 to 8.10",
    ]);
    expect(
      (screen.getByLabelText("Web Modeler REST API") as HTMLInputElement)
        .placeholder,
    ).toBe("");
  });

  it("names the minors without defaults", () => {
    setup(["8.7", "8.8", "8.9"]);

    expect(
      screen.getByText(
        "The images table has no defaults for Camunda 8.7, 8.8.",
      ),
    ).toBeDefined();
  });

  it("says which minor stands in when no cluster or management plane uses the config", () => {
    setup([]);

    expect(
      screen.getByText(
        "No cluster or management plane uses this config. The defaults are those of Camunda 8.10, the newest in the images table.",
      ),
    ).toBeDefined();
  });

  it("does not call the config unused when its users are not known", () => {
    setup([], {}, [], false);

    expect(screen.queryByText(/^No cluster or management plane uses/)).toBeNull();
    expect(
      screen.getByText(
        "The users of this config are not known. The defaults are those of Camunda 8.10, the newest in the images table.",
      ),
    ).toBeDefined();
  });

  it("says when a field has no default", () => {
    setup(["8.8"]);

    expect(defaultsOf("Console")).toEqual(["No default in the images table."]);
  });

  it("never lists elasticsearch", () => {
    setup(["8.9"]);

    expect(screen.queryByText(/elasticsearch/i)).toBeNull();
    expect(screen.getAllByRole("textbox")).toHaveLength(9);
  });

  it("edits one repository and keeps the others", () => {
    const { onChange } = setup(["8.9"], {
      camunda: "registry.example.com/camunda/camunda",
    });

    fireEvent.change(screen.getByLabelText("Keycloak"), {
      target: { value: "mirror.example.com/keycloak" },
    });

    expect(onChange).toHaveBeenLastCalledWith({
      camunda: "registry.example.com/camunda/camunda",
      keycloak: "mirror.example.com/keycloak",
    });
  });

  it("shows the error of a field next to it", () => {
    setup(["8.9"], { optimize: "Bad" }, [
      { path: "spec.images.optimize", message: "A repository is lowercase." },
    ]);

    expect(screen.getByLabelText("Optimize").getAttribute("aria-invalid")).toBe(
      "true",
    );
    expect(screen.getByText("A repository is lowercase.")).toBeDefined();
  });
});
