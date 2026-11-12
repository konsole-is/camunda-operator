import { beforeEach, describe, expect, it, vi } from "vitest";
import { MAX_LOG_STREAMS } from "@/lib/observe/logs";
import {
  PodLogNotFound,
  PodLogUnavailable,
  followPodLog,
} from "@/lib/observe/pod-log";
import { GET } from "./route";

vi.mock("@/lib/observe/pod-log", async (real) => ({
  ...(await real<typeof import("@/lib/observe/pod-log")>()),
  followPodLog: vi.fn(),
}));

const lines = (...values: string[]) =>
  new ReadableStream<string>({
    start(controller) {
      values.forEach((v) => controller.enqueue(v));
      controller.close();
    },
  });

const call = (query: string) =>
  GET(new Request(`http://localhost/api/logs/merged?${query}`));

async function frames(response: Response) {
  const text = await response.text();
  return text
    .split("\n\n")
    .filter(Boolean)
    .map((frame) => {
      const [event, data] = frame.split("\n");
      return {
        event: event.replace("event: ", ""),
        data: JSON.parse(data.replace("data: ", "")),
      };
    });
}

beforeEach(() => vi.resetAllMocks());

describe("GET /api/logs/merged", () => {
  it.each([
    ["no log", "tailLines=5", "log"],
    ["a log without a pod", "log=team-a", "log"],
    ["a log with an empty pod", "log=team-a%2F", "log"],
    ["a log with four parts", "log=a%2Fb%2Fc%2Fd", "log"],
    [
      "more logs than one view follows",
      Array.from(
        { length: MAX_LOG_STREAMS + 1 },
        (_, i) => `log=n%2Fp-${i}`,
      ).join("&"),
      "log",
    ],
    ["tailLines=0", "log=n%2Fp&tailLines=0", "tailLines"],
    ["sinceSeconds=x", "log=n%2Fp&sinceSeconds=x", "sinceSeconds"],
  ])("answers 400 for %s and names the parameter", async (_, query, name) => {
    const response = await call(query);

    expect(response.status).toBe(400);
    expect(await response.text()).toContain(name);
    expect(followPodLog).not.toHaveBeenCalled();
  });

  it("follows each log with the shared parameters and the signal of the request", async () => {
    vi.mocked(followPodLog).mockImplementation(async () => lines());

    const request = new Request(
      "http://localhost/api/logs/merged?log=team-a%2Forders-zeebe-0&log=plugins%2Fctl-0%2Fmanager&tailLines=200&sinceSeconds=60",
    );
    const response = await GET(request);
    await response.text();

    expect(response.headers.get("Content-Type")).toContain("text/event-stream");
    expect(vi.mocked(followPodLog).mock.calls.map(([req]) => req)).toEqual([
      {
        namespace: "team-a",
        pod: "orders-zeebe-0",
        tailLines: 200,
        sinceSeconds: 60,
      },
      {
        namespace: "plugins",
        pod: "ctl-0",
        container: "manager",
        tailLines: 200,
        sinceSeconds: 60,
      },
    ]);
    for (const [, signal] of vi.mocked(followPodLog).mock.calls) {
      expect(signal).toBe(request.signal);
    }
  });

  it("sends the lines of each log with the index of its log, then an end for each", async () => {
    vi.mocked(followPodLog)
      .mockResolvedValueOnce(lines("a1", "a2"))
      .mockResolvedValueOnce(lines("b1"));

    const got = await frames(await call("log=n%2Fa&log=n%2Fb"));

    expect(got.filter((f) => f.event === "line")).toEqual(
      expect.arrayContaining([
        { event: "line", data: { source: 0, line: "a1" } },
        { event: "line", data: { source: 0, line: "a2" } },
        { event: "line", data: { source: 1, line: "b1" } },
      ]),
    );
    const a = got.filter((f) => f.data.source === 0).map((f) => f.data.line);
    expect(a).toEqual(["a1", "a2", undefined]);
    expect(got.filter((f) => f.event === "end").map((f) => f.data)).toEqual(
      expect.arrayContaining([{ source: 0 }, { source: 1 }]),
    );
  });

  it.each([
    [
      "a pod that does not exist",
      new PodLogNotFound("The pod n/a does not exist"),
      "The pod n/a does not exist",
    ],
    [
      "a container that is not running",
      new PodLogUnavailable(409, "The container c of pod n/a is not running"),
      "The container c of pod n/a is not running",
    ],
    [
      "another error",
      new Error("secret token xyz"),
      "The log stream of pod a broke",
    ],
  ])(
    "sends a failure for %s and follows the other logs",
    async (_, error, message) => {
      vi.mocked(followPodLog)
        .mockRejectedValueOnce(error)
        .mockResolvedValueOnce(lines("b1"));

      const got = await frames(await call("log=n%2Fa&log=n%2Fb"));

      expect(got).toContainEqual({
        event: "failure",
        data: { source: 0, message },
      });
      expect(got).toContainEqual({
        event: "line",
        data: { source: 1, line: "b1" },
      });
    },
  );

  it("sends a failure with a fixed text when a log breaks", async () => {
    let sent = false;
    vi.mocked(followPodLog).mockResolvedValue(
      new ReadableStream<string>({
        pull(controller) {
          if (sent) {
            controller.error(new Error("secret token xyz"));
            return;
          }
          sent = true;
          controller.enqueue("a1");
        },
      }),
    );

    const got = await frames(await call("log=n%2Fa"));

    expect(got).toEqual([
      { event: "line", data: { source: 0, line: "a1" } },
      {
        event: "failure",
        data: { source: 0, message: "The log stream of pod a broke" },
      },
    ]);
  });

  it("cancels each log when the client cancels, also a log that opens after it", async () => {
    const cancelled: string[] = [];
    const open = (name: string) =>
      new ReadableStream<string>({
        cancel() {
          cancelled.push(name);
        },
      });
    let late!: (stream: ReadableStream<string>) => void;
    vi.mocked(followPodLog)
      .mockResolvedValueOnce(open("a"))
      .mockReturnValueOnce(new Promise((resolve) => (late = resolve)));

    const response = await call("log=n%2Fa&log=n%2Fb");
    await new Promise((resolve) => setTimeout(resolve, 0));
    await response.body!.cancel();
    late(open("b"));
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(cancelled.sort()).toEqual(["a", "b"]);
  });

  it("reads a log only as fast as the client reads the answer", async () => {
    let pulled = 0;
    vi.mocked(followPodLog).mockImplementation(
      async () =>
        new ReadableStream<string>({
          pull(controller) {
            pulled++;
            controller.enqueue(`line ${pulled}`);
          },
        }),
    );

    const response = await call("log=n%2Fa&log=n%2Fb");
    const reader = response.body!.getReader();
    await reader.read();
    await new Promise((resolve) => setTimeout(resolve, 50));

    expect(pulled).toBeLessThan(100);
    await reader.cancel();
  });
});
