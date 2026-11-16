// @vitest-environment jsdom
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { GrantPreview } from "@/lib/claims/templates";
import type { Change, Preview, WriteResult } from "@/lib/write/types";
import {
  type ClusterReviewSummary,
  ClusterReviewStep,
} from "./cluster-review-step";

afterEach(cleanup);

const namespace = {
  apiVersion: "v1",
  kind: "Namespace",
  metadata: {
    name: "team-new",
    labels: { "fleet.camunda.io/environment": "prod", team: "orders" },
  },
};
const claim = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  metadata: { name: "orders", namespace: "team-new" },
  spec: {
    presetRef: "small",
    releaseRef: "camunda-8-9",
    plugins: [{ section: "backup-storage", registration: "s3" }],
  },
};
const change: Change = {
  intent: "Create the cluster orders in team-new",
  apply: [namespace, claim],
  create: ["Namespace//team-new", "CamundaClusterClaim/team-new/orders"],
};

const summary: ClusterReviewSummary = [
  {
    label: "Environment and name",
    terms: [
      { label: "Environment", value: "Production" },
      { label: "Namespace", value: "team-new", note: "New" },
    ],
  },
  {
    label: "Storage and access",
    terms: [{ label: "Backup storage", value: "S3 bucket" }],
  },
];

const effects = [
  "Creates the namespace team-new.",
  "Creates the claim orders.",
  "Creates the CamundaCluster orders.",
];

const grant: GrantPreview = {
  section: "backup-storage",
  sectionLabel: "Backup storage",
  registration: "s3",
  optionLabel: "S3 bucket",
  template: "read",
  result: { value: { Resource: "<bucket ARN from Backup storage>" } },
};

function step(over: Partial<Parameters<typeof ClusterReviewStep>[0]> = {}) {
  const props = {
    change,
    summary,
    effects,
    grants: [grant],
    mode: "direct" as const,
    preview: vi.fn(async (): Promise<Preview> => ({ objects: [], errors: [] })),
    create: vi.fn(async (): Promise<WriteResult> => ({
      ok: true,
      mode: "direct",
    })),
    onWritten: vi.fn(),
    onCancel: vi.fn(),
    ...over,
  };
  render(<ClusterReviewStep {...props} />);
  return props;
}

const create = () =>
  screen.getByRole("button", { name: "Create" }) as HTMLButtonElement;

describe("ClusterReviewStep", () => {
  it("shows the claim, the new namespace, the step summaries, and what the create makes", async () => {
    step();
    const review = screen.getByRole("region", { name: "Review" });
    expect(
      within(review).getByRole("figure", { name: "The claim" }).textContent,
    ).toContain("releaseRef: camunda-8-9");
    expect(
      within(review).getByRole("figure", { name: "The namespace" }).textContent,
    ).toContain("team: orders");
    const environment = within(review).getByRole("region", {
      name: "Environment and name",
    });
    expect(environment.textContent).toContain("Production");
    expect(environment.textContent).toContain("New");
    expect(
      within(review).getByRole("region", { name: "Storage and access" })
        .textContent,
    ).toContain("S3 bucket");
    expect(
      within(review).getByRole("region", {
        name: "Permissions the options ask for",
      }).textContent,
    ).toContain("<bucket ARN from Backup storage>");
    expect(
      within(screen.getByRole("list", { name: "What this change does" }))
        .getAllByRole("listitem")
        .map((li) => li.textContent),
    ).toEqual(effects);
    await waitFor(() => expect(create().disabled).toBe(false));
  });

  it("shows no namespace for a claim in an existing namespace", () => {
    step({
      change: { ...change, apply: [claim], create: [change.create![1]] },
    });
    expect(screen.queryByRole("figure", { name: "The namespace" })).toBeNull();
  });

  it("shows each refusal of the dry run and keeps Create off", async () => {
    const props = step({
      preview: vi.fn(async () => ({
        objects: [],
        errors: [
          {
            code: "Forbidden" as const,
            message: "The app may not create Namespaces.",
          },
          { code: "Invalid" as const, message: "spec.presetRef: Required" },
        ],
      })),
    });
    const alert = await screen.findByRole("alert");
    expect(
      within(alert)
        .getAllByRole("listitem")
        .map((li) => li.textContent),
    ).toEqual([
      "The app may not create Namespaces.",
      "spec.presetRef: Required",
    ]);
    expect(create().disabled).toBe(true);
    await userEvent.click(create());
    expect(props.create).not.toHaveBeenCalled();
  });

  it("shows the error of one grant template and keeps Create", async () => {
    step({
      grants: [
        grant,
        {
          ...grant,
          template: "tags",
          result: { error: 'parameter "tags" has no value', path: "Tags" },
        },
      ],
    });
    expect(screen.getByRole("alert").textContent).toBe(
      'Tags: parameter "tags" has no value',
    );
    await waitFor(() => expect(create().disabled).toBe(false));
  });

  it("names a list that could not be read and keeps Create off", async () => {
    const props = step({ unread: ["PluginRegistration"] });
    await waitFor(() => expect(props.preview).toHaveBeenCalled());
    expect(screen.getAllByRole("alert").map((a) => a.textContent)).toEqual([
      "The fleet manager could not read the PluginRegistration objects, so the permissions are not known.",
      "The fleet manager could not read the PluginRegistration objects. The review can miss what the create makes, so Create stays off.",
    ]);
    expect(create().disabled).toBe(true);
  });

  it("writes the change once on Create and gives the result to onWritten", async () => {
    const props = step();
    await waitFor(() => expect(create().disabled).toBe(false));
    await userEvent.click(create());
    expect(props.create).toHaveBeenCalledTimes(1);
    expect(props.create).toHaveBeenCalledWith(change);
    expect(props.onWritten).toHaveBeenCalledWith({ ok: true, mode: "direct" });
  });

  it("keeps Create off while blocked", async () => {
    const props = step({ blocked: true });
    await waitFor(() => expect(props.preview).toHaveBeenCalled());
    expect(create().disabled).toBe(true);
  });

  it("puts Back beside Create and calls onCancel", async () => {
    const props = step();
    await userEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(props.onCancel).toHaveBeenCalled();
  });

  it("reads Push to Git in Git mode", () => {
    step({ mode: "git" });
    expect(screen.getByRole("button", { name: "Push to Git" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Create" })).toBeNull();
  });
});
