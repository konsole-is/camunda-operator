// @vitest-environment jsdom
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import {
  EnvironmentEditor,
  type EnvironmentActions,
} from "./environment-editor";
import { sectionFixtures } from "./fixtures";
import type { EnvironmentSources } from "./sources";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
// The editor gets its writes as props. The page module of the server
// actions loads the writer, which a component test does not need.
vi.mock("@/app/setup/environments/actions", () => ({
  previewEnvironmentChange: vi.fn(),
  applyEnvironmentChange: vi.fn(),
  previewNamespaceLabel: vi.fn(),
  applyNamespaceLabel: vi.fn(),
}));
vi.mock("@/lib/k8s/hooks", () => ({
  useGitSync: () => "waiting",
  useGitGone: () => "waiting",
}));

function renderEditor(name: string, sources: EnvironmentSources) {
  const actions = {
    previewChange: vi.fn<EnvironmentActions["previewChange"]>(async () => ({
      objects: [],
      errors: [],
    })),
    applyChange: vi.fn<EnvironmentActions["applyChange"]>(async () => ({
      ok: true as const,
      mode: "direct" as const,
    })),
    previewLabel: vi.fn(async () => ({ objects: [], errors: [] })),
    applyLabel: vi.fn(async () => ({
      ok: true as const,
      mode: "direct" as const,
    })),
  };
  render(
    <GitPushesProvider>
      <EnvironmentEditor
        sources={{ ...sources, errors: [] }}
        name={name}
        mode="direct"
        actions={actions}
      />
    </GitPushesProvider>,
  );
  return actions;
}

// group returns the settings of a parameter in the settings of an option.
const group = (option: string, parameter: string) =>
  within(screen.getByRole("region", { name: option })).getByRole("group", {
    name: parameter,
  });

async function save(actions: ReturnType<typeof renderEditor>) {
  fireEvent.click(screen.getByRole("button", { name: "Review" }));
  const confirm = screen.getByRole("button", { name: "Save" });
  await waitFor(() => expect(confirm).toHaveProperty("disabled", false));
  fireEvent.click(confirm);
  await waitFor(() => expect(actions.applyChange).toHaveBeenCalledTimes(1));
  return actions.applyChange.mock.calls[0][0].apply[0].spec;
}

afterEach(cleanup);

function withoutChoices() {
  const sources = sectionFixtures();
  delete sources.environments[0].spec.parameters!["aws-connectors-role"]
    .extraPolicyArns;
  return sources;
}

const extra = () => group("Role for each cluster", "extraPolicyArns");

describe("AllowedEditor", () => {
  it("is shown only while A claim can change it is on", () => {
    renderEditor("production", withoutChoices());
    expect(
      within(extra()).queryByRole("button", { name: "Add a choice" }),
    ).toBeNull();
    fireEvent.click(
      within(extra()).getByRole("checkbox", { name: "A claim can change it" }),
    );
    expect(
      within(extra()).getByRole("button", { name: "Add a choice" }),
    ).toBeTruthy();
  });

  it("saves name, label, and value", async () => {
    const actions = renderEditor("production", withoutChoices());
    fireEvent.click(
      within(extra()).getByRole("checkbox", { name: "A claim can change it" }),
    );
    fireEvent.click(within(extra()).getByRole("button", { name: "Add a choice" }));
    const choice = within(extra()).getByRole("group", { name: "Choice 1" });
    fireEvent.change(within(choice).getByLabelText("Name"), {
      target: { value: "sqs" },
    });
    fireEvent.change(within(choice).getByLabelText("Label"), {
      target: { value: "SQS access" },
    });
    fireEvent.change(within(choice).getByLabelText("Value"), {
      target: { value: "arn:aws:iam::aws:policy/AmazonSQSFullAccess" },
    });
    const spec = await save(actions);
    expect(spec.parameters["aws-connectors-role"].extraPolicyArns).toEqual({
      claimEditable: true,
      allowed: [
        {
          name: "sqs",
          label: "SQS access",
          value: "arn:aws:iam::aws:policy/AmazonSQSFullAccess",
        },
      ],
    });
  });

  it("refuses a duplicate name or label and blocks Review", () => {
    const { previewChange } = renderEditor("production", sectionFixtures());
    fireEvent.click(within(extra()).getByRole("button", { name: "Add a choice" }));
    const second = within(extra()).getByRole("group", { name: "Choice 2" });
    fireEvent.change(within(second).getByLabelText("Name"), {
      target: { value: "sqs" },
    });
    fireEvent.change(within(second).getByLabelText("Label"), {
      target: { value: "SQS access" },
    });
    expect(within(second).getByText("Each name is used once.")).toBeTruthy();
    expect(within(second).getByText("Each label is used once.")).toBeTruthy();
    const review = screen.getByRole("button", { name: "Review" });
    expect(review).toHaveProperty("disabled", true);
    fireEvent.click(review);
    expect(previewChange).not.toHaveBeenCalled();
  });

  it("removes a choice", async () => {
    const actions = renderEditor("production", sectionFixtures());
    const first = within(extra()).getByRole("group", { name: "Choice 1" });
    fireEvent.click(within(first).getByRole("button", { name: "Remove choice 1" }));
    const spec = await save(actions);
    expect(spec.parameters["aws-connectors-role"].extraPolicyArns).toEqual({
      claimEditable: true,
    });
  });
});
