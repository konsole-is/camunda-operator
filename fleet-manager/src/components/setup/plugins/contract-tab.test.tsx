// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { contractOf } from "./contract";
import { ContractTab } from "./contract-tab";
import { pluginRegistration, registrationSources } from "./fixtures";

afterEach(cleanup);

const sources = registrationSources();
const es = sources.registrations.find(
  (r) => r.metadata.name === "elasticsearch",
)!;

describe("ContractTab", () => {
  it("names Delete and Orphan and no other deletion policy", () => {
    render(
      <ContractTab
        contract={contractOf(
          pluginRegistration("x", { deletionPolicies: ["Delete"] }),
          [],
          [],
        )}
      />,
    );
    const deletion = screen.getByRole("region", { name: "Deletion" });
    expect(deletion.textContent).toMatch(/Delete: supported/);
    expect(deletion.textContent).toMatch(/Orphan: not supported/);
    expect(deletion.textContent).not.toMatch(/Retain/);
  });

  it("shows the sections and the fields they fill", () => {
    render(<ContractTab contract={contractOf(es, sources.sections, [])} />);
    const sections = screen.getByRole("region", { name: "Sections" });
    expect(within(sections).getByText("Secondary storage")).toBeTruthy();
    expect(
      within(sections).getByText("Fills CamundaCluster spec.storageRef"),
    ).toBeTruthy();
  });

  it("says when a section does not exist", () => {
    render(
      <ContractTab
        contract={contractOf(
          pluginRegistration("x", { sections: ["gone"] }),
          [],
          [],
        )}
      />,
    );
    const sections = screen.getByRole("region", { name: "Sections" });
    expect(
      within(sections).getByText("Not a section on this cluster"),
    ).toBeTruthy();
  });

  it("does not say that a section is missing while PluginSection is not read", () => {
    render(
      <ContractTab
        contract={contractOf(
          pluginRegistration("x", { sections: ["gone"] }),
          [],
          ["PluginSection"],
        )}
      />,
    );
    const sections = screen.getByRole("region", { name: "Sections" });
    expect(
      within(sections).queryByText("Not a section on this cluster"),
    ).toBeNull();
    expect(
      within(sections).getByText("Not known: PluginSection is not read."),
    ).toBeTruthy();
  });

  it("shows the inputs with their source, formats, and whether they are required", () => {
    render(
      <ContractTab
        contract={contractOf(
          pluginRegistration("aws-s3", {
            inputs: [
              {
                name: "kmsKeyArn",
                from: "encryption-key/keyArn",
                formats: ["aws-kms-arn"],
                required: true,
              },
              { name: "principal", from: "cloud-identity/roleArn" },
            ],
          }),
          sources.sections,
          [],
        )}
      />,
    );
    const inputs = screen.getByRole("region", { name: "Inputs" });
    const rows = within(inputs).getAllByRole("row");
    expect(rows.map((r) => r.textContent)).toEqual([
      "InputFromFormatsRequired",
      "kmsKeyArnEncryption key / keyArnencryption-key/keyArnaws-kms-arnRequired",
      "principalCloud identity / roleArncloud-identity/roleArnAnyOptional",
    ]);
  });

  it("shows the outputs with their description and format", () => {
    render(<ContractTab contract={contractOf(es, sources.sections, [])} />);
    const outputs = screen.getByRole("region", { name: "Outputs" });
    expect(within(outputs).getByText("secondaryStorageConfig")).toBeTruthy();
    expect(
      within(outputs).getByText(
        "The name of the SecondaryStorageConfig that the cluster reads.",
      ),
    ).toBeTruthy();
  });

  it("says when the option takes no input and gives no output", () => {
    render(
      <ContractTab
        contract={contractOf(pluginRegistration("x", {}), [], [])}
      />,
    );
    expect(screen.getByText("This option takes no input.")).toBeTruthy();
    expect(screen.getByText("This option gives no output.")).toBeTruthy();
  });

  it("shows the error of an input that is not valid", () => {
    render(
      <ContractTab
        contract={contractOf(
          pluginRegistration("x", {
            inputs: [{ name: "a", from: 3 }] as never,
          }),
          [],
          [],
        )}
      />,
    );
    const inputs = screen.getByRole("region", { name: "Inputs" });
    expect(within(inputs).getByRole("alert").textContent).toBe(
      "Input 1 is not a valid input.",
    );
  });
});
