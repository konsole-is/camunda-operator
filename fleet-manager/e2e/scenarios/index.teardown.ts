import { test } from "@playwright/test";
import { PAGES_DIR, writeShotIndex } from "../shot";

// A teardown runs after the page tests also when one failed, so the
// artifact has an index of every screenshot that was taken.
test("write the page index", async () => {
  await writeShotIndex(PAGES_DIR, "Fleet manager UI pages");
});
