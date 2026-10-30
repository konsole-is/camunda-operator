import http from "node:http";
import type { AddressInfo } from "node:net";

/** PrometheusStub is a Prometheus HTTP API on 127.0.0.1 that the app of a page scenario queries. */
export interface PrometheusStub {
  url: string;
  /** requests holds the URL of each request, in order. */
  requests: URL[];
  close(): Promise<void>;
}

/**
 * startPrometheusStub starts the stub on a free port. answer gives the JSON
 * body of a request, or undefined for the default: an empty matrix for
 * /api/v1/query_range, a vector with one sample 1 for /api/v1/query, and
 * 404 for another path.
 */
export async function startPrometheusStub(
  answer?: (url: URL) => unknown | undefined,
): Promise<PrometheusStub> {
  const requests: URL[] = [];
  const server = http.createServer((req, res) => {
    const url = new URL(req.url ?? "/", "http://127.0.0.1");
    requests.push(url);
    const body = answer?.(url) ?? defaultAnswer(url);
    if (body === undefined) {
      res.writeHead(404).end();
      return;
    }
    res
      .writeHead(200, { "Content-Type": "application/json" })
      .end(JSON.stringify(body));
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  const { port } = server.address() as AddressInfo;
  return {
    url: `http://127.0.0.1:${port}`,
    requests,
    close: () =>
      new Promise((resolve) => {
        server.closeAllConnections();
        server.close(() => resolve());
      }),
  };
}

function defaultAnswer(url: URL): unknown {
  switch (url.pathname) {
    case "/api/v1/query_range":
      return { status: "success", data: { resultType: "matrix", result: [] } };
    case "/api/v1/query":
      return {
        status: "success",
        data: {
          resultType: "vector",
          result: [{ metric: {}, value: [Date.now() / 1000, "1"] }],
        },
      };
    default:
      return undefined;
  }
}
