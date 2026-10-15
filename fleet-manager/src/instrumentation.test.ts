import { afterEach, describe, expect, it, vi } from "vitest";
import { register } from "./instrumentation";

describe("register", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.restoreAllMocks();
  });

  it("reads the images table and the catalog when the server starts", async () => {
    vi.stubEnv("NEXT_RUNTIME", "nodejs");
    const log = vi.spyOn(console, "log").mockImplementation(() => {});

    await register();

    expect(log).toHaveBeenCalledOnce();
    expect(log.mock.calls[0][0]).toMatch(/^Read the images table /);
  });

  it("stops the server when a file is missing", async () => {
    vi.stubEnv("NEXT_RUNTIME", "nodejs");
    vi.stubEnv("FLEET_CATALOG", "/nonexistent/official-plugins.json");
    const error = vi.spyOn(console, "error").mockImplementation(() => {});
    const exit = vi
      .spyOn(process, "exit")
      .mockImplementation(() => undefined as never);

    await register();

    expect(exit).toHaveBeenCalledWith(1);
    expect(error.mock.calls[0][0]).toContain(
      "/nonexistent/official-plugins.json",
    );
  });

  it("reads nothing outside the Node.js runtime", async () => {
    vi.stubEnv("NEXT_RUNTIME", "edge");
    vi.stubEnv("FLEET_CATALOG", "/nonexistent/official-plugins.json");
    const exit = vi
      .spyOn(process, "exit")
      .mockImplementation(() => undefined as never);

    await register();

    expect(exit).not.toHaveBeenCalled();
  });
});
