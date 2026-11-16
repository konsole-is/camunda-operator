import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { startEnvtest } from "./envtest";

const dirs: string[] = [];

afterEach(() => {
  for (const dir of dirs.splice(0)) {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

// fakeAssets returns a directory with an etcd that counts its starts in the
// file runs, writes message to stderr, and exits with status 1.
function fakeAssets(message: string): string {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "fake-envtest-"));
  dirs.push(dir);
  fs.writeFileSync(
    path.join(dir, "etcd"),
    [
      "#!/bin/sh",
      `echo run >> "${path.join(dir, "runs")}"`,
      `echo ${JSON.stringify(message)} >&2`,
      "exit 1",
      "",
    ].join("\n"),
    { mode: 0o755 },
  );
  return dir;
}

function runs(dir: string): number {
  return fs
    .readFileSync(path.join(dir, "runs"), "utf8")
    .split("\n")
    .filter(Boolean).length;
}

describe("startEnvtest", () => {
  it("starts again on new ports when a port is taken, three times in all", async () => {
    const dir = fakeAssets(
      "listen tcp 127.0.0.1:40123: bind: address already in use",
    );

    await expect(startEnvtest({ assetsDir: dir })).rejects.toThrow(
      /etcd exited during the start of envtest:\n.*bind: address already in use/,
    );
    expect(runs(dir)).toBe(3);
  });

  it("fails at once with its message on any other failure", async () => {
    const dir = fakeAssets("the data directory is not writable");

    await expect(startEnvtest({ assetsDir: dir })).rejects.toThrow(
      /etcd exited during the start of envtest:\nthe data directory is not writable/,
    );
    expect(runs(dir)).toBe(1);
  });
});
