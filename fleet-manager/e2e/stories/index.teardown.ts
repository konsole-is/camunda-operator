import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { test } from "@playwright/test";
import { type Shot, renderIndex } from "../gallery/gallery-index";

const OUT = path.resolve("ui-stories");

// A teardown runs after the stories project also when a story failed, so the
// artifact has an index of every screenshot that was taken.
test("write the stories index", async () => {
  await mkdir(OUT, { recursive: true });
  const manifests = (await readdir(OUT)).filter((file) =>
    /^manifest-\d+(-light)?\.json$/.test(file),
  );
  const shots: Shot[] = [];
  for (const file of manifests) {
    shots.push(
      ...(JSON.parse(await readFile(path.join(OUT, file), "utf8")) as Shot[]),
    );
  }
  await writeFile(
    path.join(OUT, "index.html"),
    renderIndex(shots, "Fleet manager stories"),
  );
});
