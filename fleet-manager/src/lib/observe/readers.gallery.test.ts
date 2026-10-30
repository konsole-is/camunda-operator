import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { list } from "@/lib/k8s/read";
import { podUsage as realPodUsage, podUsageFrom } from "./kube";
import { followPodLog as realFollowPodLog } from "./pod-log";
import {
  OBSERVE_FILE_VARIABLE,
  followPodLog,
  parseObserveFile,
  podUsage,
} from "./readers.gallery";

vi.mock("./kube", () => ({ podUsage: vi.fn(), podUsageFrom: vi.fn() }));
vi.mock("./pod-log", () => ({ followPodLog: vi.fn() }));
vi.mock("@/lib/k8s/read", () => ({ list: vi.fn() }));

let dir = "";

async function observe(text: string): Promise<void> {
  const file = path.join(dir, "observe.yaml");
  await writeFile(file, text);
  vi.stubEnv(OBSERVE_FILE_VARIABLE, file);
}

async function readAll(stream: ReadableStream<string>): Promise<string[]> {
  const lines: string[] = [];
  for await (const line of stream) lines.push(line);
  return lines;
}

const signal = new AbortController().signal;

beforeEach(async () => {
  vi.resetAllMocks();
  dir = await mkdtemp(path.join(tmpdir(), "observe-"));
});
afterEach(async () => {
  vi.unstubAllEnvs();
  await rm(dir, { recursive: true, force: true });
});

describe("followPodLog of the gallery build", () => {
  it("reads the log lines of a pod from observe.yaml, the last tailLines of them", async () => {
    await observe(`schemaVersion: 1
logs:
  - namespace: team-a
    pod: p-0
    lines: [a, b, c]
`);
    const stream = await followPodLog(
      { namespace: "team-a", pod: "p-0", tailLines: 2 },
      signal,
    );
    expect(await readAll(stream)).toEqual(["b", "c"]);
    expect(realFollowPodLog).not.toHaveBeenCalled();
  });

  it("reads the lines of the container that the request names", async () => {
    await observe(`schemaVersion: 1
logs:
  - { namespace: team-a, pod: p-0, container: main, lines: [main] }
  - { namespace: team-a, pod: p-0, container: side, lines: [side] }
`);
    const stream = await followPodLog(
      { namespace: "team-a", pod: "p-0", container: "side" },
      signal,
    );
    expect(await readAll(stream)).toEqual(["side"]);
  });

  it("follows the real log of a pod that observe.yaml does not name", async () => {
    await observe(`schemaVersion: 1
logs:
  - { namespace: team-a, pod: p-0, lines: [a] }
`);
    const real = new ReadableStream<string>();
    vi.mocked(realFollowPodLog).mockResolvedValue(real);
    const req = { namespace: "team-a", pod: "p-1" };
    expect(await followPodLog(req, signal)).toBe(real);
    expect(realFollowPodLog).toHaveBeenCalledWith(req, signal);
  });
});

describe("podUsage of the gallery build", () => {
  it("reads the usage of the selected pods from observe.yaml", async () => {
    await observe(`schemaVersion: 1
usage:
  - { namespace: team-a, pod: p-0, cpu: 250m, memory: 1Gi }
  - { namespace: team-a, pod: p-1, cpu: 500m, memory: 2Gi }
`);
    vi.mocked(list).mockResolvedValue([{ metadata: { name: "p-0" } }] as never);
    vi.mocked(podUsageFrom).mockResolvedValue([]);

    await podUsage("team-a", "app=x");

    expect(list).toHaveBeenCalledWith("Pod", {
      namespace: "team-a",
      labelSelector: "app=x",
    });
    expect(podUsageFrom).toHaveBeenCalledWith("team-a", "app=x", {
      items: [
        {
          metadata: { name: "p-0" },
          containers: [{ usage: { cpu: "250m", memory: "1Gi" } }],
        },
      ],
    });
    expect(realPodUsage).not.toHaveBeenCalled();
  });

  it("returns what the real reader returns without observe.yaml", async () => {
    vi.stubEnv(OBSERVE_FILE_VARIABLE, undefined);
    vi.mocked(realPodUsage).mockResolvedValue("unavailable");
    expect(await podUsage("team-a", "app=x")).toBe("unavailable");
    expect(realPodUsage).toHaveBeenCalledWith("team-a", "app=x");
  });

  it("returns what the real reader returns when the file does not exist", async () => {
    vi.stubEnv(OBSERVE_FILE_VARIABLE, path.join(dir, "missing.yaml"));
    vi.mocked(realPodUsage).mockResolvedValue("unavailable");
    expect(await podUsage("team-a", "app=x")).toBe("unavailable");
  });
});

describe("parseObserveFile", () => {
  it("refuses another schemaVersion", () => {
    expect(() => parseObserveFile("schemaVersion: 2")).toThrow(/schemaVersion/);
  });

  it("reads a missing list as empty", () => {
    expect(parseObserveFile("schemaVersion: 1")).toEqual({
      logs: [],
      usage: [],
    });
  });
});
