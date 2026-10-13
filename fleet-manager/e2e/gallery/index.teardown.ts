import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { test } from "@playwright/test";
import { type Shot, renderIndex } from "./gallery-index";

const OUT = path.resolve("ui-gallery");

// A teardown runs after the gallery project also when a story failed, so the
// artifact has an index of every screenshot that was taken.
test("write the gallery index", async () => {
  await mkdir(OUT, { recursive: true });
  const manifests = (await readdir(OUT)).filter((file) =>
    /^manifest-\d+\.json$/.test(file),
  );
  const shots: Shot[] = [];
  for (const file of manifests) {
    shots.push(
      ...(JSON.parse(await readFile(path.join(OUT, file), "utf8")) as Shot[]),
    );
  }
  await writeFile(path.join(OUT, "index.html"), renderIndex(shots));
});
