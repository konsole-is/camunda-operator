import { describe, expect, it } from "vitest";
import { GET } from "./route";

describe("GET /api/healthz", () => {
  it("answers ok", async () => {
    const response = GET();

    expect(response.status).toBe(200);
    expect(await response.text()).toBe("ok");
  });
});
