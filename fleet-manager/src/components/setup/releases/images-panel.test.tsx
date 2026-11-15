// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { imagesFixture } from "./fixtures";
import { ImagesPanel } from "./images-panel";
import { releaseImages } from "./images";

afterEach(cleanup);

describe("ImagesPanel", () => {
  it("shows the default for the version with the release version as the tag", () => {
    render(
      <ImagesPanel
        rows={releaseImages(
          { version: "8.9.0", connectors: { version: "8.9.7" } },
          imagesFixture,
        )}
        minor="8.9"
        onPin={vi.fn()}
      />,
    );
    for (const image of [
      "camunda/camunda:8.9.0",
      "camunda/optimize:8.9.0",
      "camunda/connectors-bundle:8.9.7",
      "ghcr.io/cloudnative-pg/postgresql:17",
    ])
      expect(screen.getByText(image)).toBeTruthy();
    expect(screen.getByText("9.2.8")).toBeTruthy();
    expect(screen.getByText("Picked by its operator")).toBeTruthy();
    expect(
      screen.getByText(
        "The repository is the default of the images table. The platform config of each cluster sets the repository it pulls from.",
      ),
    ).toBeTruthy();
  });

  it("says that each cluster sets its connectors version when the release sets none", () => {
    render(
      <ImagesPanel
        rows={releaseImages({ version: "8.9.0" }, imagesFixture)}
        minor="8.9"
        onPin={vi.fn()}
      />,
    );
    expect(
      screen.getByText("Each cluster sets its own connectors version"),
    ).toBeTruthy();
  });

  it("edits only the camunda and connectors images", () => {
    const onPin = vi.fn();
    render(
      <ImagesPanel
        rows={releaseImages(
          { version: "8.9.0", images: { camunda: "reg/c:1" } },
          imagesFixture,
        )}
        minor="8.9"
        onPin={onPin}
      />,
    );
    expect(
      screen
        .getAllByRole("textbox")
        .map((box) => box.getAttribute("aria-label")),
    ).toEqual(["Pinned Camunda image", "Pinned Connectors image"]);
    expect(
      (
        screen.getByRole("textbox", {
          name: "Pinned Camunda image",
        }) as HTMLInputElement
      ).value,
    ).toBe("reg/c:1");
    expect(screen.getByText("The release pins no Optimize image")).toBeTruthy();
    fireEvent.change(
      screen.getByRole("textbox", { name: "Pinned Connectors image" }),
      { target: { value: "reg/k:2" } },
    );
    expect(onPin).toHaveBeenCalledWith("connectors", "reg/k:2");
  });

  it("shows the empty state for a minor that the table does not hold", () => {
    const onPin = vi.fn();
    render(<ImagesPanel rows={undefined} minor="8.10" onPin={onPin} />);
    expect(
      screen.getByText(
        "The images table has no entry for Camunda 8.10. Pin the images here, or set them in the platform config.",
      ),
    ).toBeTruthy();
    fireEvent.change(
      screen.getByRole("textbox", { name: "Pinned Camunda image" }),
      { target: { value: "reg/c:9" } },
    );
    expect(onPin).toHaveBeenCalledWith("camunda", "reg/c:9");
    expect(
      screen.getByRole("textbox", { name: "Pinned Connectors image" }),
    ).toBeTruthy();
  });
});
