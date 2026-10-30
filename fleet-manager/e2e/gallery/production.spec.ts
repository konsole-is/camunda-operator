import { existsSync } from "node:fs";
import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import { expect, test } from "@playwright/test";
import { OBSERVE_FILE_VARIABLE } from "../../src/lib/observe/observe-variable";

test("a production build serves the app and returns 404 for the gallery", async ({
  request,
}) => {
  expect((await request.get("/fleet")).status()).toBe(200);
  for (const path of [
    "/dev/gallery",
    "/dev/gallery/statuschip/ready",
    "/dev/watch-probe",
  ]) {
    expect((await request.get(path)).status(), path).toBe(404);
  }
});

test("a production build holds none of the test readers", async () => {
  const dirs = [".next/server", ".next/static", ".next/standalone"].filter(
    (dir) => existsSync(dir),
  );
  expect(dirs.length, "make fleet-build builds .next").toBeGreaterThan(0);
  const holding: string[] = [];
  for (const dir of dirs) {
    for (const entry of await readdir(dir, {
      recursive: true,
      withFileTypes: true,
    })) {
      if (!entry.isFile()) continue;
      const file = path.join(entry.parentPath, entry.name);
      if ((await readFile(file, "latin1")).includes(OBSERVE_FILE_VARIABLE)) {
        holding.push(file);
      }
    }
  }
  expect(holding).toEqual([]);
});
