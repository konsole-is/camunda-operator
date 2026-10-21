import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  ACCESS_SECTION,
  BACKUP_SCHEDULE_LABEL,
  CLAIM_LABEL,
  CLUSTER_LABEL,
  COMPONENT_LABEL,
  ENVIRONMENT_LABEL,
  REVISION_ANNOTATION,
  SECTION_LABEL,
  isLabelKey,
  isLabelValue,
  isManaged,
} from ".";

// goConstant reads the value of a Go string constant from a file of the
// repository. \b keeps ClusterKey from matching ManagementClusterKey.
function goConstant(file: string, name: string): string | undefined {
  const source = readFileSync(
    new URL(`../../../../${file}`, import.meta.url),
    "utf8",
  );
  return new RegExp(`\\b${name}\\s*=\\s*"([^"]*)"`).exec(source)?.[1];
}

describe("isLabelKey", () => {
  it.each(["team", "app.kubernetes.io/name", "a", "a_b.c-d"])(
    "accepts %j",
    (key) => {
      expect(isLabelKey(key)).toBe(true);
    },
  );
  it.each([
    "",
    "-team",
    "team-",
    "a/b/c",
    "UPPER.example.com/x",
    "x".repeat(64),
    "/name",
  ])("refuses %j", (key) => {
    expect(isLabelKey(key)).toBe(false);
  });
});

describe("isLabelValue", () => {
  it.each(["", "payments", "a.b_c-d", "x".repeat(63)])(
    "accepts %j",
    (value) => {
      expect(isLabelValue(value)).toBe(true);
    },
  );
  it.each(["-x", "x-", "a b", "x".repeat(64)])("refuses %j", (value) => {
    expect(isLabelValue(value)).toBe(false);
  });
});

describe("ENVIRONMENT_LABEL", () => {
  // The Go constant is the source. A change on one side alone fails here.
  it("matches LabelEnvironment of api/fleet/v1alpha1", () => {
    const goSource = readFileSync(
      new URL(
        "../../../../api/fleet/v1alpha1/shared_types.go",
        import.meta.url,
      ),
      "utf8",
    );
    expect(ENVIRONMENT_LABEL).toBe(
      /LabelEnvironment\s*=\s*"([^"]*)"/.exec(goSource)?.[1],
    );
  });
});

// The Go constants are the source. A change on one side alone fails here.
it.each([
  [CLAIM_LABEL, "api/fleet/v1alpha1/shared_types.go", "LabelClaim"],
  [
    REVISION_ANNOTATION,
    "api/fleet/v1alpha1/shared_types.go",
    "AnnotationRevision",
  ],
  [SECTION_LABEL, "api/plugins/v1alpha1/shared_types.go", "LabelSection"],
  [ACCESS_SECTION, "internal/pluginstartup/names.go", "SectionAccess"],
  [CLUSTER_LABEL, "pkg/labels/labels.go", "ClusterKey"],
  [COMPONENT_LABEL, "pkg/labels/labels.go", "ComponentKey"],
  [BACKUP_SCHEDULE_LABEL, "pkg/labels/labels.go", "BackupScheduleKey"],
])("%s matches %s %s", (value, file, name) => {
  expect(value).toBe(goConstant(file, name));
});

describe("isManaged", () => {
  it("is true for an object with the claim label", () => {
    expect(
      isManaged({ metadata: { labels: { [CLAIM_LABEL]: "orders" } } }),
    ).toBe(true);
  });

  it.each([
    ["no metadata", {}],
    ["no labels", { metadata: {} }],
    ["other labels", { metadata: { labels: { team: "a" } } }],
    ["an empty claim label", { metadata: { labels: { [CLAIM_LABEL]: "" } } }],
  ])("is false with %s", (_, object) => {
    expect(isManaged(object)).toBe(false);
  });
});
