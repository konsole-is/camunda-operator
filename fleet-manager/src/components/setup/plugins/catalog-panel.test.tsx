// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { CatalogChart } from "@/startup/bundled-data";
import { CatalogPanel } from "./catalog-panel";
import { registrationSources } from "./fixtures";

afterEach(cleanup);

const chart = (name: string, sections = ["encryption-key"]): CatalogChart => ({
  name,
  description: `The ${name} chart.`,
  sections,
  chart: `oci://example/${name}`,
  version: "0.1.0",
  repository: "https://example.com/charts",
  installCommand: `helm install ${name} oci://example/${name} --version 0.1.0`,
});

const COMMAND = "helm install aws-kms oci://example/aws-kms --version 0.1.0";

function stubClipboard(writeText: (text: string) => Promise<void>) {
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText },
    configurable: true,
  });
}

describe("CatalogPanel", () => {
  it("renders nothing for no chart", () => {
    const { container } = render(<CatalogPanel charts={[]} sections={[]} />);
    expect(container.innerHTML).toBe("");
  });

  it("names the sections of a chart by their labels and shows its command", () => {
    render(
      <CatalogPanel
        charts={[chart("aws-kms")]}
        sections={registrationSources().sections}
      />,
    );
    const panel = screen.getByRole("region", {
      name: "Charts you can install",
    });
    expect(within(panel).getByText("aws-kms")).toBeTruthy();
    expect(within(panel).getByText("The aws-kms chart.")).toBeTruthy();
    expect(within(panel).getByText("Encryption key")).toBeTruthy();
    expect(within(panel).getByText(COMMAND)).toBeTruthy();
  });

  it("names a section that does not exist by its name", () => {
    render(
      <CatalogPanel charts={[chart("vault", ["secrets"])]} sections={[]} />,
    );
    expect(screen.getByText("secrets")).toBeTruthy();
  });

  it("copies the install command", async () => {
    // userEvent.setup puts its own clipboard on navigator.
    const user = userEvent.setup();
    const writeText = vi.fn().mockResolvedValue(undefined);
    stubClipboard(writeText);
    render(<CatalogPanel charts={[chart("aws-kms")]} sections={[]} />);
    await user.click(screen.getByRole("button", { name: "Copy the command" }));
    expect(writeText).toHaveBeenCalledWith(COMMAND);
    expect(await screen.findByRole("button", { name: "Copied" })).toBeTruthy();
  });

  it("asks to copy the command by hand when the clipboard refuses it", async () => {
    const user = userEvent.setup();
    stubClipboard(vi.fn().mockRejectedValue(new Error("denied")));
    render(<CatalogPanel charts={[chart("aws-kms")]} sections={[]} />);
    await user.click(screen.getByRole("button", { name: "Copy the command" }));
    expect(
      await screen.findByText("Select the command and copy it."),
    ).toBeTruthy();
  });
});
