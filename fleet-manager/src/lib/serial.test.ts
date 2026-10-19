import { describe, expect, it } from "vitest";
import { inOrder } from "./serial";

const tick = () => new Promise((resolve) => setTimeout(resolve, 5));

describe("inOrder", () => {
  it("runs the calls of one key one after the other, in call order", async () => {
    const events: string[] = [];
    const job = (name: string) => async () => {
      events.push(`start ${name}`);
      await tick();
      events.push(`end ${name}`);
      return name;
    };

    const results = await Promise.all([
      inOrder("a", job("1")),
      inOrder("a", job("2")),
    ]);

    expect(results).toEqual(["1", "2"]);
    expect(events).toEqual(["start 1", "end 1", "start 2", "end 2"]);
  });

  it("a failed call does not stop the next call of the key", async () => {
    const failed = inOrder("b", async () => {
      throw new Error("boom");
    });
    const next = inOrder("b", async () => "ran");

    await expect(failed).rejects.toThrow("boom");
    expect(await next).toBe("ran");
  });

  it("calls of different keys do not wait for each other", async () => {
    let release = () => {};
    const blocked = inOrder("c", () => new Promise<void>((r) => (release = r)));

    expect(await inOrder("d", async () => "free")).toBe("free");
    release();
    await blocked;
  });
});
