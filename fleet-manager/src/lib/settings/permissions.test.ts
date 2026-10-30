import { beforeEach, describe, expect, it, vi } from "vitest";
import { KubernetesRequestError, requestJSON } from "@/lib/k8s/client";
import { appPermissions } from "./permissions";

vi.mock("@/lib/k8s/client", async (real) => ({
  ...(await real<typeof import("@/lib/k8s/client")>()),
  kubeConfig: vi.fn(),
  requestJSON: vi.fn(),
}));

const review = (status: Record<string, unknown>) => ({ status });

beforeEach(() => {
  vi.resetAllMocks();
  process.env.FLEET_NAMESPACE = "fleet";
});

describe("appPermissions", () => {
  it("asks for the rules of the app in FLEET_NAMESPACE", async () => {
    vi.mocked(requestJSON).mockResolvedValue(
      review({ resourceRules: [], incomplete: false }),
    );

    await appPermissions();

    expect(vi.mocked(requestJSON).mock.calls[0][1]).toEqual(
      expect.objectContaining({
        method: "POST",
        path: "/apis/authorization.k8s.io/v1/selfsubjectrulesreviews",
        body: expect.objectContaining({
          kind: "SelfSubjectRulesReview",
          spec: { namespace: "fleet" },
        }),
      }),
    );
  });

  it("drops the discovery rows and the non-resource rules", async () => {
    vi.mocked(requestJSON).mockResolvedValue(
      review({
        resourceRules: [
          {
            verbs: ["create"],
            apiGroups: ["authorization.k8s.io"],
            resources: ["selfsubjectaccessreviews", "selfsubjectrulesreviews"],
          },
          {
            verbs: ["create"],
            apiGroups: ["authentication.k8s.io"],
            resources: ["selfsubjectreviews"],
          },
          {
            verbs: ["get", "update", "patch"],
            apiGroups: [""],
            resources: ["configmaps"],
            resourceNames: ["fleet-manager-settings"],
          },
        ],
        nonResourceRules: [{ verbs: ["get"], nonResourceURLs: ["/healthz"] }],
        incomplete: false,
      }),
    );

    expect(await appPermissions()).toEqual({
      rules: [
        {
          apiGroups: [""],
          resources: ["configmaps"],
          resourceNames: ["fleet-manager-settings"],
          verbs: ["get", "update", "patch"],
        },
      ],
      incomplete: false,
    });
  });

  it("keeps a rule that names a selfsubject resource beside another resource", async () => {
    const rule = {
      verbs: ["create"],
      apiGroups: ["authorization.k8s.io"],
      resources: ["selfsubjectaccessreviews", "subjectaccessreviews"],
    };
    vi.mocked(requestJSON).mockResolvedValue(
      review({ resourceRules: [rule], incomplete: false }),
    );

    expect((await appPermissions()).rules).toEqual([
      { ...rule, resourceNames: [] },
    ]);
  });

  it("keeps a selfsubject rule of another API group", async () => {
    const rule = {
      verbs: ["*"],
      apiGroups: ["*"],
      resources: ["selfsubjectreviews"],
    };
    vi.mocked(requestJSON).mockResolvedValue(
      review({ resourceRules: [rule], incomplete: false }),
    );

    expect((await appPermissions()).rules).toEqual([
      { ...rule, resourceNames: [] },
    ]);
  });

  it("sorts the rules by API group, then by resource", async () => {
    const rule = (apiGroups: string[], resources: string[]) => ({
      verbs: ["get"],
      apiGroups,
      resources,
    });
    vi.mocked(requestJSON).mockResolvedValue(
      review({
        resourceRules: [
          rule(["fleet.camunda.io"], ["environments"]),
          rule([""], ["secrets"]),
          rule(["apps"], ["deployments"]),
          rule([""], ["configmaps"]),
        ],
        incomplete: false,
      }),
    );

    expect(
      (await appPermissions()).rules.map((r) => [
        r.apiGroups.join(","),
        r.resources.join(","),
      ]),
    ).toEqual([
      ["", "configmaps"],
      ["", "secrets"],
      ["apps", "deployments"],
      ["fleet.camunda.io", "environments"],
    ]);
  });

  it("lists a rule that two roles grant once", async () => {
    const rule = {
      verbs: ["get"],
      apiGroups: [""],
      resources: ["configmaps"],
      resourceNames: ["fleet-manager-settings"],
    };
    vi.mocked(requestJSON).mockResolvedValue(
      review({ resourceRules: [rule, { ...rule }], incomplete: false }),
    );

    expect((await appPermissions()).rules).toEqual([rule]);
  });

  it("passes incomplete on", async () => {
    vi.mocked(requestJSON).mockResolvedValue(
      review({ resourceRules: [], incomplete: true }),
    );

    expect((await appPermissions()).incomplete).toBe(true);
  });

  it("gives no rules for a review without status rules", async () => {
    vi.mocked(requestJSON).mockResolvedValue(review({}));

    expect(await appPermissions()).toEqual({ rules: [], incomplete: false });
  });

  it("rejects when the API server refuses the review", async () => {
    vi.mocked(requestJSON).mockRejectedValue(
      new KubernetesRequestError(403, "forbidden"),
    );

    await expect(appPermissions()).rejects.toThrow("forbidden");
  });
});
