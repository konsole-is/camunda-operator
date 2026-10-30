import { test } from "@playwright/test";
import { JOURNEYS_DIR, writeShotIndex } from "../shot";

// A teardown runs after the journeys also when a journey failed, so the
// artifact has an index of every screenshot that was taken.
test("write the journey index", async () => {
  await writeShotIndex(JOURNEYS_DIR, "Fleet manager UI journeys");
});
