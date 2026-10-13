import { expect, test } from "@playwright/test";

test("a production build serves the app and returns 404 for the gallery", async ({
  request,
}) => {
  expect((await request.get("/fleet")).status()).toBe(200);
  for (const path of ["/dev/gallery", "/dev/gallery/statuschip/ready"]) {
    expect((await request.get(path)).status(), path).toBe(404);
  }
});
