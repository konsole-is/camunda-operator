// serve.mts serves a folder under a path prefix only, as GitHub Pages serves
// the Storybook of a pull request:
//
//   node e2e/stories/serve.mts <folder> <prefix> <port>
//
// A path outside the prefix, or one that leaves the folder, gets 404.
import { createReadStream, statSync } from "node:fs";
import { createServer } from "node:http";
import path from "node:path";

const [folder, prefix, port] = process.argv.slice(2);
const root = path.resolve(folder);

const TYPES: Record<string, string> = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript",
  ".mjs": "text/javascript",
  ".css": "text/css",
  ".json": "application/json",
  ".svg": "image/svg+xml",
  ".woff": "font/woff",
  ".woff2": "font/woff2",
  ".png": "image/png",
};

function fileOf(urlPath: string): string | undefined {
  if (!urlPath.startsWith(prefix)) return undefined;
  let relative: string;
  try {
    relative = decodeURIComponent(urlPath.slice(prefix.length));
  } catch {
    return undefined;
  }
  const file = path.join(root, relative);
  if (file !== root && !file.startsWith(root + path.sep)) return undefined;
  try {
    return statSync(file).isDirectory() ? path.join(file, "index.html") : file;
  } catch {
    return undefined;
  }
}

createServer((req, res) => {
  const file = fileOf(new URL(req.url ?? "/", "http://localhost").pathname);
  if (!file) {
    res.writeHead(404).end();
    return;
  }
  const stream = createReadStream(file);
  stream.on("open", () => {
    res.writeHead(200, {
      "content-type": TYPES[path.extname(file)] ?? "application/octet-stream",
    });
    stream.pipe(res);
  });
  stream.on("error", () => res.writeHead(404).end());
}).listen(Number(port), "127.0.0.1");
