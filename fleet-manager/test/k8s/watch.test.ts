import { randomUUID } from "node:crypto";
import {
  KubeConfig,
  KubernetesObjectApi,
  type KubernetesObject,
} from "@kubernetes/client-node";
import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import { GET } from "@/app/api/watch/route";
import { defaultCache } from "@/lib/k8s/cache";

process.env.KUBECONFIG = inject("kubeconfig");

const kc = new KubeConfig();
kc.loadFromFile(inject("kubeconfig"));
const api = KubernetesObjectApi.makeApiClient(kc);
const namespace = `watch-${randomUUID().slice(0, 8)}`;

interface Frame {
  event: string;
  retry?: number;
  data: {
    watch: string;
    type?: string;
    kind: string;
    object?: KubernetesObject;
    objects?: KubernetesObject[];
  };
}

// Stream reads the SSE frames of one response and lets a test wait for one.
class Stream {
  private readonly frames: Frame[] = [];
  private waiters: (() => void)[] = [];
  readonly closed: Promise<void>;

  constructor(response: Response) {
    const reader = response
      .body!.pipeThrough(new TextDecoderStream())
      .getReader();
    this.closed = (async () => {
      let buffer = "";
      for (;;) {
        const { value, done } = await reader.read();
        if (done) return;
        buffer += value;
        for (
          let end = buffer.indexOf("\n\n");
          end >= 0;
          end = buffer.indexOf("\n\n")
        ) {
          const text = buffer.slice(0, end);
          buffer = buffer.slice(end + 2);
          if (text.startsWith(":")) continue;
          const retry = /^retry: (\d+)$/m.exec(text)?.[1];
          this.frames.push({
            retry: retry === undefined ? undefined : Number(retry),
            event: /^event: (.*)$/m.exec(text)![1],
            data: JSON.parse(/^data: (.*)$/m.exec(text)![1]),
          });
          this.waiters.splice(0).forEach((wake) => wake());
        }
      }
    })();
  }

  async next(
    match: (frame: Frame) => boolean,
    timeoutMs: number,
  ): Promise<Frame> {
    const deadline = Date.now() + timeoutMs;
    for (;;) {
      const found = this.frames.find(match);
      if (found) return found;
      const left = deadline - Date.now();
      if (left <= 0) {
        throw new Error(`No matching SSE frame within ${timeoutMs} ms`);
      }
      await Promise.race([
        new Promise<void>((wake) => this.waiters.push(wake)),
        new Promise((wake) => setTimeout(wake, left)),
      ]);
    }
  }
}

// openMany opens one connection with one parameter watch for each query.
function openMany(queries: string[]): { stream: Stream; abort: () => void } {
  const controller = new AbortController();
  const params = new URLSearchParams(queries.map((q) => ["watch", q]));
  const response = GET(
    new Request(`http://localhost/api/watch?${params}`, {
      signal: controller.signal,
    }),
  );
  return { stream: new Stream(response), abort: () => controller.abort() };
}

function open(query: string): { stream: Stream; abort: () => void } {
  return openMany([query]);
}

const isDelta = (type: string, name: string) => (f: Frame) =>
  f.event === "delta" &&
  f.data.type === type &&
  f.data.object?.metadata?.name === name;

beforeAll(async () => {
  await api.create({
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name: namespace },
  });
});
afterAll(async () => {
  await defaultCache().stop();
});

describe("GET /api/watch against envtest", () => {
  it("a watch delivers a new object to a subscriber within 5 seconds", async () => {
    const { stream, abort } = open(`kind=Secret&namespace=${namespace}`);
    await stream.next((f) => f.event === "snapshot", 15_000);

    await api.create({
      apiVersion: "v1",
      kind: "Secret",
      metadata: { name: "new", namespace },
    });

    await stream.next(isDelta("ADDED", "new"), 5_000);
    abort();
  });

  it("a watch delivers a change of an object within 5 seconds", async () => {
    const name = `env-${randomUUID().slice(0, 8)}`;
    await api.create({
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      metadata: { name },
      spec: { displayName: "Before" },
    });
    const { stream, abort } = open("kind=Environment");
    await stream.next(
      (f) =>
        (f.event === "snapshot" &&
          f.data.objects!.some((o) => o.metadata?.name === name)) ||
        isDelta("ADDED", name)(f),
      15_000,
    );

    const live = await api.read({
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind: "Environment",
      metadata: { name },
    });
    await api.replace({
      ...live,
      spec: { displayName: "After" },
    } as KubernetesObject);

    const frame = await stream.next(isDelta("MODIFIED", name), 5_000);
    expect(frame.data.object).toMatchObject({
      spec: { displayName: "After" },
    });
    abort();
  });

  it("a Secret in a delta has no data and no stringData", async () => {
    const value = "sse-secret-value";
    const { stream, abort } = open(`kind=Secret&namespace=${namespace}`);
    await stream.next((f) => f.event === "snapshot", 15_000);

    await api.create({
      apiVersion: "v1",
      kind: "Secret",
      metadata: { name: "token", namespace },
      stringData: { token: value },
    });

    const frame = await stream.next(isDelta("ADDED", "token"), 5_000);
    expect(frame.data.object).not.toHaveProperty("data");
    expect(frame.data.object).not.toHaveProperty("stringData");
    expect(JSON.stringify(frame)).not.toContain(value);
    expect(JSON.stringify(frame)).not.toContain(
      Buffer.from(value).toString("base64"),
    );
    abort();
  });

  it("a new connection starts with a snapshot of the matching objects", async () => {
    await api.create({
      apiVersion: "v1",
      kind: "Secret",
      metadata: { name: "labelled", namespace, labels: { tier: "web" } },
    });
    // The Secret informer is warm, so the create can take a moment to reach it.
    await expect
      .poll(
        async () => {
          const { stream, abort } = open(
            `kind=Secret&namespace=${namespace}&labelSelector=tier%3Dweb`,
          );
          const frame = await stream.next(
            (f) => f.event === "snapshot",
            15_000,
          );
          abort();
          return frame.data.objects!.map((o) => o.metadata!.name);
        },
        { timeout: 5_000 },
      )
      .toEqual(["labelled"]);
  });

  it("an object whose labels stop matching arrives as DELETED", async () => {
    await api.create({
      apiVersion: "v1",
      kind: "Secret",
      metadata: { name: "moving", namespace, labels: { tier: "db" } },
    });
    const { stream, abort } = open(
      `kind=Secret&namespace=${namespace}&labelSelector=tier%3Ddb`,
    );
    // The Secret informer is warm, so the object arrives in the snapshot or
    // as an ADDED right after it.
    await stream.next(
      (f) =>
        (f.event === "snapshot" &&
          f.data.objects!.some((o) => o.metadata?.name === "moving")) ||
        isDelta("ADDED", "moving")(f),
      15_000,
    );

    const live = await api.read({
      apiVersion: "v1",
      kind: "Secret",
      metadata: { name: "moving", namespace },
    });
    await api.replace({
      ...live,
      metadata: { ...live.metadata, labels: { tier: "web" } },
    } as KubernetesObject);

    await stream.next(isDelta("DELETED", "moving"), 5_000);
    abort();
  });

  it("closing the request ends the stream", async () => {
    const { stream, abort } = open(`kind=Secret&namespace=${namespace}`);
    await stream.next((f) => f.event === "snapshot", 15_000);

    abort();

    await stream.closed;
  });

  it("answers 400 with the problem for a request without a watch", async () => {
    const response = GET(new Request("http://localhost/api/watch?kind=Pod"));

    expect(response.status).toBe(400);
    expect(await response.text()).toBe(
      "Name at least one watch with the parameter watch",
    );
  });

  it("names a watch that is not valid in a failure frame, and serves the other watch", async () => {
    const secrets = `kind=Secret&namespace=${namespace}`;
    const { stream, abort } = openMany(["kind=Widget", secrets]);

    const failure = await stream.next((f) => f.event === "failure", 15_000);
    await stream.next(
      (f) => f.event === "snapshot" && f.data.watch === secrets,
      15_000,
    );
    await api.create({
      apiVersion: "v1",
      kind: "Secret",
      metadata: { name: "after-widget", namespace },
    });

    expect(failure.data).toEqual({
      watch: "kind=Widget",
      kind: "Widget",
      message: 'The fleet manager does not read the kind "Widget"',
    });
    await stream.next(isDelta("ADDED", "after-widget"), 5_000);
    abort();
  });

  it("names a kind that the API server does not serve, then ends the stream so EventSource connects again", async () => {
    const query = `kind=Secret&kind=HTTPRoute&namespace=${namespace}`;
    const { stream } = open(query);

    const frame = await stream.next((f) => f.event === "failure", 15_000);
    await stream.closed;

    expect(frame.data.watch).toBe(query);
    expect(frame.data.kind).toBe("HTTPRoute");
    expect(JSON.stringify(frame.data)).toMatch(/does not serve HTTPRoute/);
    expect(frame.retry).toBe(5_000);
  });

  it("one connection serves two watches, and each delta reaches only its watch", async () => {
    for (const side of ["left", "right"]) {
      await api.create({
        apiVersion: "v1",
        kind: "Secret",
        metadata: { name: side, namespace, labels: { side } },
      });
    }
    const left = `kind=Secret&namespace=${namespace}&labelSelector=side%3Dleft`;
    const right = `kind=Secret&namespace=${namespace}&labelSelector=side%3Dright`;
    const { stream, abort } = openMany([left, right]);
    for (const query of [left, right]) {
      await stream.next(
        (f) => f.event === "snapshot" && f.data.watch === query,
        15_000,
      );
    }

    await api.create({
      apiVersion: "v1",
      kind: "Secret",
      metadata: { name: "left-2", namespace, labels: { side: "left" } },
    });

    await stream.next(
      (f) => isDelta("ADDED", "left-2")(f) && f.data.watch === left,
      5_000,
    );
    await expect(
      stream.next(
        (f) =>
          f.data.watch === right &&
          (f.data.object?.metadata?.name === "left-2" ||
            !!f.data.objects?.some((o) => o.metadata?.name === "left-2")),
        0,
      ),
    ).rejects.toThrow("No matching SSE frame");
    abort();
  });
});
