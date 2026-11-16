import { describe, expect, it } from "vitest";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import type { BackupLike } from "./backups";
import {
  moveStatus,
  parseVersion,
  releaseMove,
  versionCheckWait,
} from "./release";

const cluster = (version?: string, specVersion?: string) => ({
  metadata: { namespace: "team-a", name: "orders" },
  spec: { version: specVersion },
  status: version ? { management: { version } } : {},
});
const release = (name: string, version: string) =>
  ({ metadata: { name }, spec: { version } }) as CamundaRelease;
const env = (releases?: Environment["spec"]["releases"]) =>
  ({ metadata: { name: "dev" }, spec: { releases } }) as Environment;
const running = (name: string): BackupLike => ({
  kind: "LogicalBackupRDBMS",
  metadata: { namespace: "team-a", name },
  spec: { clusterRef: { name: "orders" } },
  status: { phase: "Running" },
});
const pending = (name: string): BackupLike => ({
  ...running(name),
  status: { phase: "Pending" },
});

describe("releaseMove", () => {
  it("reports a downgrade against status.management.version even when the old release names a lower version", () => {
    expect(
      releaseMove(
        cluster("8.9.20", "8.9.5"),
        release("8.9.10", "8.9.10"),
        undefined,
        [],
      ),
    ).toMatchObject({ current: "8.9.20", target: "8.9.10", downgrade: true });
  });

  it("reports a skipped minor with the minor to move to first", () => {
    expect(
      releaseMove(
        cluster("8.9.20"),
        release("8.11.0", "8.11.0"),
        undefined,
        [],
      ),
    ).toMatchObject({ downgrade: false, skippedMinor: "8.10" });
  });

  it.each([
    ["the next minor", "8.10.0"],
    ["a change of major", "9.1.0"],
    ["a patch", "8.9.21"],
  ])("reports neither a downgrade nor a skip for %s", (_, version) => {
    const move = releaseMove(
      cluster("8.9.20"),
      release(version, version),
      undefined,
      [],
    );

    expect(move.downgrade).toBe(false);
    expect(move).not.toHaveProperty("skippedMinor");
  });

  it("has no current version, no downgrade, and no skip for a cluster without status.management", () => {
    const move = releaseMove(
      cluster(undefined, "8.9.20"),
      release("8.8.0", "8.8.0"),
      undefined,
      [],
    );

    expect(move).toEqual({ allowed: true, target: "8.8.0", downgrade: false });
  });

  it("counts a version that is not major.minor.patch as no version", () => {
    const move = releaseMove(
      cluster("8.9"),
      release("8.7.0", "8.7.0"),
      undefined,
      [],
    );

    expect(move).toMatchObject({ current: "8.9", downgrade: false });
    expect(move).not.toHaveProperty("skippedMinor");
  });

  it("allows every release without an environment", () => {
    expect(
      releaseMove(cluster("8.9.20"), release("any", "8.9.21"), undefined, [])
        .allowed,
    ).toBe(true);
  });

  it("allows a release that the environment lists", () => {
    expect(
      releaseMove(
        cluster("8.9.20"),
        release("8.9.21", "8.9.21"),
        env({ allowed: ["8.9.21"] }),
        [],
      ).allowed,
    ).toBe(true);
  });

  it("does not allow a release that the environment does not list", () => {
    expect(
      releaseMove(
        cluster("8.9.20"),
        release("8.9.21", "8.9.21"),
        env({ allowed: ["8.9.20"] }),
        [],
      ).allowed,
    ).toBe(false);
  });

  it("does not allow any release when the environment has no releases", () => {
    expect(
      releaseMove(cluster("8.9.20"), release("8.9.21", "8.9.21"), env(), [])
        .allowed,
    ).toBe(false);
  });

  it("reports the running backup of the cluster", () => {
    expect(
      releaseMove(cluster("8.9.20"), release("8.9.21", "8.9.21"), undefined, [
        running("nightly-1"),
      ]).backup,
    ).toEqual({
      kind: "LogicalBackupRDBMS",
      namespace: "team-a",
      name: "nightly-1",
    });
  });
});

describe("parseVersion", () => {
  it("parseVersion reads major.minor.patch and refuses another form", () => {
    expect(parseVersion("8.10.2")).toEqual([8, 10, 2]);
    for (const version of ["8.10", "8.10.2-rc1", "v8.10.2", "", undefined]) {
      expect(parseVersion(version)).toBeUndefined();
    }
  });
});

describe("moveStatus", () => {
  const status = (
    c: ReturnType<typeof cluster> & { spec: { suspend?: boolean } },
    target: string,
    environment?: Environment,
    backups: BackupLike[] = [],
  ) =>
    moveStatus(
      releaseMove(c, release(target, target), environment, backups),
      c,
    );

  it("can move to a newer patch that the environment allows", () => {
    expect(status(cluster("8.9.0"), "8.9.1")).toEqual({
      kind: "canMove",
      text: "Can move.",
    });
  });

  it("puts a release that the environment does not allow first", () => {
    expect(
      status(cluster("8.9.0"), "8.11.0", env({ allowed: ["other"] }), [
        running("b1"),
      ]),
    ).toEqual({
      kind: "notAllowed",
      text: "The environment does not allow this release.",
    });
  });

  it("names a pending backup and blocks on it", () => {
    expect(
      status(cluster("8.9.0"), "8.9.1", undefined, [pending("nightly")]),
    ).toEqual({
      kind: "backupRunning",
      backup: { kind: "LogicalBackupRDBMS", namespace: "team-a", name: "nightly" },
      text: "Backup nightly is running. The move waits until it ends.",
    });
  });

  it("puts a running backup before a skip", () => {
    expect(
      status(cluster("8.9.0"), "8.11.0", undefined, [running("nightly")]).kind,
    ).toBe("backupRunning");
  });

  it("waits for a suspended cluster to resume, also on a skip", () => {
    const c = { ...cluster("8.9.0"), spec: { suspend: true } };
    expect(status(c, "8.11.0")).toEqual({
      kind: "waitsForResume",
      text: "The version checks run when the cluster resumes.",
    });
  });

  it("waits for a cluster that reports no version", () => {
    expect(status(cluster(), "8.9.1")).toEqual({
      kind: "waitsForVersion",
      text: "The version checks run when the cluster reports its version.",
    });
  });

  it("refuses a skip with the minor to move to first", () => {
    expect(status(cluster("8.9.0"), "8.11.0")).toEqual({
      kind: "skipsMinor",
      minor: "8.10",
      text: "Skips 8.10. Move to an 8.10 release first.",
    });
  });

  it("gives a downgrade both versions", () => {
    expect(status(cluster("8.9.3"), "8.9.1")).toEqual({
      kind: "downgrade",
      from: "8.9.3",
      to: "8.9.1",
      text: "8.9.3 to 8.9.1 is a downgrade.",
    });
  });

  it("reads the running version, not spec.version", () => {
    expect(status(cluster("8.8.3", "8.9.0"), "8.10.0").kind).toBe(
      "skipsMinor",
    );
  });
});

describe("versionCheckWait", () => {
  it("has no wait for a running cluster that reports its version", () => {
    expect(versionCheckWait(cluster("8.9.0"))).toBeUndefined();
  });

  it("waits for a version that is not major.minor.patch", () => {
    expect(versionCheckWait(cluster("8.9"))).toBe(
      "The version checks run when the cluster reports its version.",
    );
  });

  it("waits for a suspended cluster before one without a version", () => {
    expect(versionCheckWait({ ...cluster(), spec: { suspend: true } })).toBe(
      "The version checks run when the cluster resumes.",
    );
  });
});
