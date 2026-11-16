import { describe, expect, it } from "vitest";
import type { CamundaClusterPresetSpec } from "@/generated/crds/camundaclusterpreset";
import { fingerprintMoved, renderedPreset } from "./fingerprint";

// The cases and values of TestPresetFingerprint* in
// pkg/components/camundacluster/confighash_test.go.
const base: CamundaClusterPresetSpec = {
  cluster: {
    version: "8.9.9",
    auth: { basic: { passwordRotation: "2026-08" } },
  },
};
const none: CamundaClusterPresetSpec = { cluster: { version: "8.9.9" } };

function withBasic(
  spec: CamundaClusterPresetSpec,
  basic: Record<string, string>,
): CamundaClusterPresetSpec {
  return {
    cluster: {
      ...spec.cluster,
      auth: { basic: { ...spec.cluster.auth?.basic, ...basic } },
    },
  };
}

describe("fingerprintMoved", () => {
  it("ignores a new password rotation", () => {
    expect(
      fingerprintMoved(base, withBasic(base, { passwordRotation: "2026-09" })),
    ).toBe(false);
  });

  it("moves for every other change", () => {
    expect(
      fingerprintMoved(base, {
        cluster: { ...base.cluster, version: "8.9.8" },
      }),
    ).toBe(true);
  });

  it("ignores the first rotation, which adds the blocks that carry it", () => {
    expect(fingerprintMoved(none, base)).toBe(false);
  });

  it("ignores the admin email", () => {
    expect(
      fingerprintMoved(
        base,
        withBasic(base, { adminEmail: "team@example.com" }),
      ),
    ).toBe(false);
  });

  it("ignores the first connectors service account", () => {
    const annotated: CamundaClusterPresetSpec = {
      cluster: {
        version: "8.9.9",
        connectors: {
          serviceAccount: {
            annotations: {
              "eks.amazonaws.com/role-arn":
                "arn:aws:iam::123456789012:role/connectors",
            },
          },
        },
      },
    };
    expect(fingerprintMoved(none, annotated)).toBe(false);
  });

  it("moves for another connectors change", () => {
    const enabledOnly: CamundaClusterPresetSpec = {
      cluster: { version: "8.9.9", connectors: { enabled: true } },
    };
    const enabled: CamundaClusterPresetSpec = {
      cluster: {
        version: "8.9.9",
        connectors: {
          enabled: true,
          serviceAccount: { annotations: { a: "b" } },
        },
      },
    };
    expect(fingerprintMoved(enabledOnly, enabled)).toBe(false);
    expect(fingerprintMoved(none, enabledOnly)).toBe(true);
  });

  it("keeps an auth field that renders, such as the client id", () => {
    const client: CamundaClusterPresetSpec = {
      cluster: {
        version: "8.9.9",
        auth: { clientId: "camunda", basic: { passwordRotation: "2026-08" } },
      },
    };
    expect(fingerprintMoved(base, client)).toBe(true);
  });
});

describe("renderedPreset", () => {
  it("does not change its input", () => {
    const spec = withBasic(base, { adminEmail: "team@example.com" });
    const copy = structuredClone(spec);
    renderedPreset(spec);
    expect(spec).toEqual(copy);
  });

  it("drops the auth and connectors blocks that are left empty", () => {
    const spec: CamundaClusterPresetSpec = {
      cluster: {
        version: "8.9.9",
        auth: { basic: { adminEmail: "team@example.com" } },
        connectors: { serviceAccount: { create: true } },
      },
    };
    expect(renderedPreset(spec)).toEqual({ cluster: { version: "8.9.9" } });
  });
});
