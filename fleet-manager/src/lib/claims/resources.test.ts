import { describe, expect, it } from "vitest";
import { resourceFate, type ResourceRow } from "./resources";

const row = (onOrphan: "Kept" | "Deleted"): ResourceRow => ({
  kind: "S3Bucket",
  name: "orders-backups",
  onOrphan,
});

describe("resourceFate", () => {
  it.each([
    ["Kept", "Delete", "Deleted"],
    ["Deleted", "Delete", "Deleted"],
    ["Kept", "Orphan", "Kept"],
    ["Deleted", "Orphan", "Deleted"],
  ] as const)(
    "a row with onOrphan %s under the policy %s is %s",
    (onOrphan, policy, fate) => {
      expect(resourceFate(row(onOrphan), policy)).toBe(fate);
    },
  );
});
