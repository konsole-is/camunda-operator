import { describe, expect, it } from "vitest";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { Environment } from "@/generated/crds/environment";
import type { BackupLike } from "./backups";
import { releaseMove } from "./release";

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
