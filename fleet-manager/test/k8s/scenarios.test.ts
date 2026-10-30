import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { type Envtest, startEnvtest } from "../envtest/envtest";
import { kubeRequest } from "../scenarios/kube";

const USER = "fleet-manager-scenarios";

// This file loads whole scenarios, whose objects would collide with the
// objects of the other files, so it starts an envtest of its own.
let envtest: Envtest;

beforeAll(async () => {
  envtest = await startEnvtest({ users: [{ name: USER }] });
}, 120_000);

afterAll(async () => {
  await envtest?.stop();
});

describe("the users option of startEnvtest", () => {
  it("authenticates a user that is not an admin", async () => {
    const user = envtest.users[USER];
    const review = await kubeRequest<{
      status: { userInfo: { username: string; groups: string[] } };
    }>(
      user.kubeConfig,
      "POST",
      "/apis/authentication.k8s.io/v1/selfsubjectreviews",
      { apiVersion: "authentication.k8s.io/v1", kind: "SelfSubjectReview" },
    );
    expect(review.status.userInfo.username).toBe(USER);
    expect(review.status.userInfo.groups).not.toContain("system:masters");
    await expect(
      kubeRequest(user.kubeConfig, "GET", "/api/v1/namespaces/default/secrets"),
    ).rejects.toMatchObject({ status: 403 });
  });

  it("refuses a user named envtest-admin before anything starts", async () => {
    await expect(
      startEnvtest({ users: [{ name: "envtest-admin" }] }),
    ).rejects.toThrow(/envtest-admin/);
  });

  it("refuses a user name that is given twice", async () => {
    await expect(
      startEnvtest({ users: [{ name: "a" }, { name: "a" }] }),
    ).rejects.toThrow(/a.*twice/);
  });
});
