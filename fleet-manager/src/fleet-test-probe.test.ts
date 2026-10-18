import { expect, test } from "vitest";

test("fails on purpose to prove the saved reports", () => {
  expect("saved").toBe("lost");
});
