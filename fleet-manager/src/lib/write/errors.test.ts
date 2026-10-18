import { describe, expect, it } from "vitest";
import { KubernetesRequestError, type KubeStatus } from "@/lib/k8s/client";
import { apiErrorsOf } from "./errors";

const claim = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  namespace: "team-a",
  name: "orders",
};
const failure = (statusCode: number, status: KubeStatus) =>
  new KubernetesRequestError(statusCode, status.message ?? "", status);

describe("apiErrorsOf", () => {
  it("puts a message that names spec.cluster.version on that field path", () => {
    const message =
      'CamundaClusterClaim.fleet.camunda.io "orders" is invalid: spec.cluster.version: Invalid value: "8.7": must be at least 8.9';
    expect(
      apiErrorsOf(failure(422, { reason: "Invalid", message }), claim, false),
    ).toEqual([
      { code: "Invalid", message, path: "spec.cluster.version", ref: claim },
    ]);
  });

  it("puts each cause with a field on its own path", () => {
    const errors = apiErrorsOf(
      failure(422, {
        reason: "Invalid",
        message: "two errors",
        details: {
          causes: [
            {
              reason: "FieldValueInvalid",
              field: "spec.cluster.version",
              message: "Invalid value: must be at least 8.9",
            },
            {
              reason: "FieldValueRequired",
              field: "spec.presetRef",
              message: "Required value",
            },
          ],
        },
      }),
      claim,
      false,
    );
    expect(errors).toEqual([
      {
        code: "Invalid",
        path: "spec.cluster.version",
        message: "Invalid value: must be at least 8.9",
        ref: claim,
      },
      {
        code: "Invalid",
        path: "spec.presetRef",
        message: "Required value",
        ref: claim,
      },
    ]);
  });

  it("keeps a 400 that names no field as one Invalid without a path", () => {
    expect(
      apiErrorsOf(
        failure(400, { message: "the body is not YAML" }),
        claim,
        false,
      ),
    ).toEqual([
      { code: "Invalid", message: "the body is not YAML", ref: claim },
    ]);
  });

  it("names the manager of a field manager conflict, also with a precondition", () => {
    const status: KubeStatus = {
      reason: "Conflict",
      message:
        'Apply failed with 1 conflict: conflict with "kubectl-edit" using fleet.camunda.io/v1alpha1: .spec.displayName',
      details: {
        causes: [
          {
            reason: "FieldManagerConflict",
            field: ".spec.displayName",
            message:
              'conflict with "kubectl-edit" using fleet.camunda.io/v1alpha1',
          },
        ],
      },
    };
    expect(apiErrorsOf(failure(409, status), claim, true)).toEqual([
      {
        code: "Conflict",
        manager: "kubectl-edit",
        path: "spec.displayName",
        message: 'conflict with "kubectl-edit" using fleet.camunda.io/v1alpha1',
        ref: claim,
      },
    ]);
  });

  it("maps another 409 on an object with a precondition to Stale", () => {
    const status = {
      reason: "Conflict",
      message:
        'Operation cannot be fulfilled on camundaclusterclaims.fleet.camunda.io "orders": the object has been modified',
    };
    expect(apiErrorsOf(failure(409, status), claim, true)).toEqual([
      {
        code: "Stale",
        message: "The CamundaClusterClaim orders changed since you opened it.",
        ref: claim,
      },
    ]);
  });

  it("maps a 404 on an object with a precondition to Stale", () => {
    expect(
      apiErrorsOf(
        failure(404, { reason: "NotFound", message: "gone" }),
        claim,
        true,
      ),
    ).toEqual([
      {
        code: "Stale",
        message: "The CamundaClusterClaim orders changed since you opened it.",
        ref: claim,
      },
    ]);
  });

  it("keeps another 409 without a precondition as a Conflict with no manager", () => {
    const message = 'camundaclusterclaims "orders" already exists';
    expect(
      apiErrorsOf(
        failure(409, { reason: "AlreadyExists", message }),
        claim,
        false,
      ),
    ).toEqual([{ code: "Conflict", message, ref: claim }]);
  });

  it("maps a 404 without a precondition to Unavailable for a kind the API server does not serve", () => {
    expect(
      apiErrorsOf(
        failure(404, { reason: "NotFound", message: "x" }),
        claim,
        false,
      ),
    ).toEqual([
      {
        code: "Unavailable",
        message:
          "The API server does not serve CamundaClusterClaim (fleet.camunda.io/v1alpha1).",
        ref: claim,
      },
    ]);
  });

  it("maps a 401 and a 403 to Forbidden", () => {
    for (const code of [401, 403]) {
      expect(
        apiErrorsOf(failure(code, { message: "forbidden" }), claim, false),
      ).toEqual([{ code: "Forbidden", message: "forbidden", ref: claim }]);
    }
  });

  it("maps another status and a lost connection to Unavailable", () => {
    expect(
      apiErrorsOf(failure(500, { message: "etcd timeout" }), claim, false),
    ).toEqual([{ code: "Unavailable", message: "etcd timeout", ref: claim }]);
    expect(
      apiErrorsOf(new Error("connect ECONNREFUSED 127.0.0.1:1"), claim, false),
    ).toEqual([
      {
        code: "Unavailable",
        message: "connect ECONNREFUSED 127.0.0.1:1",
        ref: claim,
      },
    ]);
  });
});
