// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { AboutTab } from "./about-tab";
import { pluginRegistration, registrationSources } from "./fixtures";

afterEach(cleanup);

const sources = registrationSources();

// termOf returns the value of the term label of the panel Details.
const termOf = (label: string) =>
  within(screen.getByRole("region", { name: "Details" })).getByText(label, {
    selector: "dt",
  }).nextElementSibling as HTMLElement;
const es = sources.registrations.find(
  (r) => r.metadata.name === "elasticsearch",
)!;

describe("AboutTab", () => {
  it("shows the limits set at install with the note", () => {
    render(
      <AboutTab
        registration={pluginRegistration("aws-kms", {
          limits: [{ label: "Regions", value: "eu-west-1, eu-central-1" }],
        })}
        registrations={[]}
        unread={[]}
      />,
    );
    const limits = screen.getByRole("region", {
      name: "Limits set at install",
    });
    expect(within(limits).getByText("eu-west-1, eu-central-1")).toBeTruthy();
    expect(
      within(limits).getByText(
        "No environment and no claim can change these limits.",
      ),
    ).toBeTruthy();
  });

  it("says when the option sets no limits at install", () => {
    render(<AboutTab registration={es} registrations={[]} unread={[]} />);
    const limits = screen.getByRole("region", {
      name: "Limits set at install",
    });
    expect(
      within(limits).getByText("This option sets no limits at install."),
    ).toBeTruthy();
  });

  it("shows the description, the documentation, the controller, the ServiceAccount, and the mode", () => {
    render(
      <AboutTab
        registration={pluginRegistration("aws-kms", {
          description: "Makes a KMS key for the cluster.",
          docsUrl: "https://example.com/docs/aws-kms",
          controller: { name: "aws-kms", version: "0.4.1" },
          serviceAccount: { name: "aws-kms", namespace: "plugins" },
          mode: "Default",
        })}
        registrations={[]}
        unread={[]}
      />,
    );
    const details = screen.getByRole("region", { name: "Details" });
    expect(
      within(details).getByText("Makes a KMS key for the cluster."),
    ).toBeTruthy();
    const docs = within(details).getByRole("link", {
      name: "https://example.com/docs/aws-kms",
    });
    expect(docs.getAttribute("href")).toBe("https://example.com/docs/aws-kms");
    expect(docs.getAttribute("target")).toBe("_blank");
    expect(docs.getAttribute("rel")).toBe("noreferrer");
    expect(within(details).getByText("aws-kms 0.4.1")).toBeTruthy();
    expect(within(details).getByText("plugins/aws-kms")).toBeTruthy();
    expect(within(details).getByText("Default")).toBeTruthy();
  });

  it("links the other options of the same controller", () => {
    render(
      <AboutTab
        registration={es}
        registrations={sources.registrations}
        unread={[]}
      />,
    );
    expect(
      screen
        .getByRole("link", { name: "database-server" })
        .getAttribute("href"),
    ).toBe("/setup/plugins/database-server");
    expect(screen.queryByRole("link", { name: "elasticsearch" })).toBeNull();
  });

  it("says None when no other option has the controller", () => {
    const own = pluginRegistration("aws-kms", {
      controller: { name: "aws-kms", version: "0.4.1" },
    });
    render(
      <AboutTab registration={own} registrations={[own, es]} unread={[]} />,
    );
    expect(termOf("Other options of this controller").textContent).toBe("None");
  });

  it("does not say None while PluginRegistration is not read", () => {
    const own = pluginRegistration("aws-kms", {
      controller: { name: "aws-kms", version: "0.4.1" },
    });
    render(
      <AboutTab
        registration={own}
        registrations={[own]}
        unread={["PluginRegistration"]}
      />,
    );
    expect(termOf("Other options of this controller").textContent).toBe(
      "Not known: the fleet manager has no current list of PluginRegistration.",
    );
  });

  it("shows an error for limits that are not a list", () => {
    render(
      <AboutTab
        registration={pluginRegistration("bad", {
          limits: { label: "Regions" } as never,
        })}
        registrations={[]}
        unread={[]}
      />,
    );
    const limits = screen.getByRole("region", {
      name: "Limits set at install",
    });
    expect(within(limits).getByRole("alert").textContent).toBe(
      "The limits are not a list.",
    );
  });

  it("shows an error for a limit without a label or a value", () => {
    render(
      <AboutTab
        registration={pluginRegistration("bad", {
          limits: [{ label: "Regions", value: 3 }] as never,
        })}
        registrations={[]}
        unread={[]}
      />,
    );
    const limits = screen.getByRole("region", {
      name: "Limits set at install",
    });
    expect(within(limits).getByRole("alert").textContent).toBe(
      "Limit 1 is not a valid limit.",
    );
  });
});
