import { describe, expect, it } from "vitest";
import {
  NO_MANAGEMENT_AUTH_CONFIG,
  OPTIMIZE_NEEDS_ELASTICSEARCH,
  SCHEDULE_NEEDS_BACKUP_STORAGE,
  optimizeBlocked,
  scheduleBlocked,
} from "./attachments";

describe("optimizeBlocked", () => {
  it("refuses Optimize without Elasticsearch, also when the type is not known", () => {
    expect(optimizeBlocked("rdbms", ["central"])).toBe(
      OPTIMIZE_NEEDS_ELASTICSEARCH,
    );
    expect(optimizeBlocked(undefined, ["central"])).toBe(
      OPTIMIZE_NEEDS_ELASTICSEARCH,
    );
  });

  it("refuses Optimize without a ManagementAuthConfig", () => {
    expect(optimizeBlocked("elasticsearch", [])).toBe(
      NO_MANAGEMENT_AUTH_CONFIG,
    );
  });

  it("allows Optimize with Elasticsearch and a ManagementAuthConfig", () => {
    expect(optimizeBlocked("elasticsearch", ["central"])).toBeUndefined();
  });
});

describe("scheduleBlocked", () => {
  it("needs a backup storage option", () => {
    expect(scheduleBlocked(false)).toBe(SCHEDULE_NEEDS_BACKUP_STORAGE);
    expect(scheduleBlocked(true)).toBeUndefined();
  });
});
