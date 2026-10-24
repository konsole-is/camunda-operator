import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { test } from "@playwright/test";
import { type JourneyShot, renderJourneyIndex } from "../journey-index";
import { OUT } from "../shot";

// A teardown runs after the journeys also when a journey failed, so the
// artifact has an index of every screenshot that was taken.
test("write the journey index", async () => {
  await mkdir(OUT, { recursive: true });
  const lines = await readFile(path.join(OUT, "shots.jsonl"), "utf8").catch(
    (err: NodeJS.ErrnoException) => {
      if (err.code === "ENOENT") return "";
      throw err;
    },
  );
  const shots = lines
    .split("\n")
    .filter((line) => line !== "")
    .map((line) => JSON.parse(line) as JourneyShot);
  await writeFile(path.join(OUT, "index.html"), renderJourneyIndex(shots));
});
