import { execFile, spawn } from "node:child_process";
import { mkdir, mkdtemp, rm, symlink, writeFile } from "node:fs/promises";
import http from "node:http";
import type { AddressInfo } from "node:net";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";

const run = promisify(execFile);

/**
 * BareRepository is a bare Git repository on disk that an HTTP server
 * serves at url. log returns the subjects of main, newest first. show
 * returns the file at the head of main, or undefined. commitFromOtherClone
 * commits files and pushes them to main from a second clone, and
 * linkFromOtherClone does the same for one symbolic link.
 */
export type BareRepository = {
  url: string;
  dir: string;
  close(): Promise<void>;
  commitFromOtherClone(
    files: Record<string, string>,
    message: string,
  ): Promise<void>;
  linkFromOtherClone(
    link: string,
    target: string,
    message: string,
  ): Promise<void>;
  log(): Promise<string[]>;
  show(file: string): Promise<string | undefined>;
  files(): Promise<string[]>;
};

// The global and the system config of the machine can sign commits or
// rewrite addresses, so each git call of the tests runs without them.
const gitEnv = {
  ...process.env,
  GIT_CONFIG_GLOBAL: "/dev/null",
  GIT_CONFIG_NOSYSTEM: "1",
  GIT_AUTHOR_NAME: "Other admin",
  GIT_AUTHOR_EMAIL: "other@example.com",
  GIT_COMMITTER_NAME: "Other admin",
  GIT_COMMITTER_EMAIL: "other@example.com",
};

const git = async (cwd: string, ...args: string[]) =>
  (await run("git", args, { cwd, env: gitEnv })).stdout;

/**
 * serveBareRepository makes a bare repository with one commit of
 * README.md on main and serves it with git http-backend at
 * http://127.0.0.1:<port>/fleet.git. With auth, a request without the
 * basic credentials of auth gets 401. beforeFirstPush runs once, before
 * the server takes the first push.
 */
export async function serveBareRepository(
  opts: {
    auth?: { username: string; token: string };
    beforeFirstPush?: () => Promise<void>;
  } = {},
): Promise<BareRepository> {
  const root = await mkdtemp(path.join(os.tmpdir(), "fleet-bare-"));
  const dir = path.join(root, "fleet.git");
  await git(root, "init", "--bare", "-b", "main", dir);
  await git(dir, "config", "http.receivepack", "true");
  const seed = path.join(root, "seed");
  await git(root, "clone", dir, seed);
  await writeFile(path.join(seed, "README.md"), "# fleet\n");
  await git(seed, "add", "README.md");
  await git(seed, "commit", "-m", "Initial commit");
  await git(seed, "push", "origin", "HEAD:main");

  const expected = opts.auth
    ? "Basic " +
      Buffer.from(`${opts.auth.username}:${opts.auth.token}`).toString("base64")
    : undefined;
  let hook = opts.beforeFirstPush;

  const server = http.createServer(async (req, res) => {
    if (expected && req.headers.authorization !== expected) {
      res.writeHead(401, { "WWW-Authenticate": 'Basic realm="git"' });
      res.end();
      return;
    }
    const url = new URL(req.url ?? "/", "http://localhost");
    if (req.method === "POST" && url.pathname.endsWith("/git-receive-pack")) {
      const before = hook;
      hook = undefined;
      await before?.();
    }
    serveCGI(root, req, url, res);
  });
  await new Promise<void>((resolve) =>
    server.listen(0, "127.0.0.1", () => resolve()),
  );
  const { port } = server.address() as AddressInfo;
  const address = `http://127.0.0.1:${port}/fleet.git`;

  return {
    url: address,
    dir,
    close: async () => {
      server.closeAllConnections();
      await new Promise((resolve) => server.close(resolve));
      await rm(root, { recursive: true, force: true });
    },
    commitFromOtherClone: async (files, message) => {
      const other = await mkdtemp(path.join(root, "other-"));
      await git(root, "clone", dir, other);
      for (const [file, content] of Object.entries(files)) {
        await mkdir(path.dirname(path.join(other, file)), { recursive: true });
        await writeFile(path.join(other, file), content);
        await git(other, "add", file);
      }
      await git(other, "commit", "-m", message);
      await git(other, "push", "origin", "HEAD:main");
    },
    linkFromOtherClone: async (link, linkTarget, message) => {
      const other = await mkdtemp(path.join(root, "other-"));
      await git(root, "clone", dir, other);
      await mkdir(path.dirname(path.join(other, link)), { recursive: true });
      await symlink(linkTarget, path.join(other, link));
      await git(other, "add", link);
      await git(other, "commit", "-m", message);
      await git(other, "push", "origin", "HEAD:main");
    },
    log: async () =>
      (await git(dir, "log", "--format=%s", "main"))
        .split("\n")
        .filter(Boolean),
    show: async (file) => {
      try {
        return await git(dir, "show", `main:${file}`);
      } catch {
        return undefined;
      }
    },
    files: async () =>
      (await git(dir, "ls-tree", "-r", "--name-only", "main"))
        .split("\n")
        .filter(Boolean),
  };
}

// serveCGI runs git http-backend for one request and turns its CGI answer
// into the HTTP answer.
function serveCGI(
  root: string,
  req: http.IncomingMessage,
  url: URL,
  res: http.ServerResponse,
): void {
  const cgi = spawn("git", ["http-backend"], {
    env: {
      ...gitEnv,
      GIT_PROJECT_ROOT: root,
      GIT_HTTP_EXPORT_ALL: "1",
      PATH_INFO: url.pathname,
      QUERY_STRING: url.search.slice(1),
      REQUEST_METHOD: req.method ?? "GET",
      CONTENT_TYPE: req.headers["content-type"] ?? "",
      REMOTE_USER: "fleet",
      REMOTE_ADDR: "127.0.0.1",
      ...(req.headers["content-encoding"]
        ? { HTTP_CONTENT_ENCODING: req.headers["content-encoding"] }
        : {}),
    },
  });
  req.pipe(cgi.stdin);
  const chunks: Buffer[] = [];
  cgi.stdout.on("data", (chunk: Buffer) => chunks.push(chunk));
  cgi.on("close", () => {
    const output = Buffer.concat(chunks);
    const end = output.indexOf("\r\n\r\n");
    const head = output.subarray(0, end).toString("utf8");
    let status = 200;
    const headers: Record<string, string> = {};
    for (const line of head.split("\r\n")) {
      const colon = line.indexOf(":");
      const name = line.slice(0, colon).trim();
      const value = line.slice(colon + 1).trim();
      if (name.toLowerCase() === "status") status = Number(value.split(" ")[0]);
      else if (name) headers[name] = value;
    }
    res.writeHead(status, headers);
    res.end(output.subarray(end + 4));
  });
}
