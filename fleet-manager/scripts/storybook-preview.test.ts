import { execFileSync, spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  PREVIEW_MARKER,
  placePreview,
  previewComment,
  previewFolder,
  removePreview,
  squashSite,
  stalePreviews,
} from "./storybook-preview.mjs";

const script = path.join(import.meta.dirname, "storybook-preview.mts");

function tempDir(prefix: string): string {
  return fs.mkdtempSync(path.join(os.tmpdir(), prefix));
}

function write(root: string, file: string, text: string): void {
  fs.mkdirSync(path.dirname(path.join(root, file)), { recursive: true });
  fs.writeFileSync(path.join(root, file), text);
}

/** A gh-pages checkout with the docs of mike, one preview, and main's Storybook. */
function site(): string {
  const dir = tempDir("site-");
  write(dir, ".nojekyll", "");
  write(dir, "index.html", "redirect to latest");
  write(dir, "versions.json", '[{"version":"dev"}]');
  write(dir, "dev/index.html", "dev docs");
  write(dir, "0.2/index.html", "0.2 docs");
  fs.symlinkSync("0.2", path.join(dir, "latest"));
  write(dir, "pr-preview/pr-7/index.html", "preview 7");
  write(dir, "storybook/index.html", "main storybook");
  return dir;
}

/** A storybook-static folder as the artifact holds it. */
function build(label: string): string {
  const dir = tempDir("storybook-");
  write(dir, "index.html", `manager ${label}`);
  write(dir, "iframe.html", `iframe ${label}`);
  write(dir, "project.json", '{"generatedAt":1}');
  write(dir, `assets/${label}.js`, label);
  return dir;
}

/** Every path under dir with its content, or its target for a symbolic link. */
function snapshot(dir: string, skip: string): Map<string, string> {
  const files = new Map<string, string>();
  const walk = (rel: string): void => {
    for (const entry of fs.readdirSync(path.join(dir, rel), {
      withFileTypes: true,
    })) {
      const child = path.join(rel, entry.name);
      if (child === skip) continue;
      const full = path.join(dir, child);
      if (entry.isSymbolicLink())
        files.set(child, `-> ${fs.readlinkSync(full)}`);
      else if (entry.isDirectory()) walk(child);
      else files.set(child, fs.readFileSync(full, "utf8"));
    }
  };
  walk("");
  return files;
}

function filesUnder(dir: string): string[] {
  return (fs.readdirSync(dir, { recursive: true }) as string[])
    .filter((f) => fs.statSync(path.join(dir, f)).isFile())
    .sort();
}

const refusedFolders = [
  "dev",
  "latest",
  "pr-preview",
  "pr-preview/pr-0",
  "pr-preview/pr-01",
  "pr-preview/pr-1/../../dev",
  "pr-preview/pr-1/",
  "/storybook",
  "../x",
  "",
];

describe("previewFolder", () => {
  it("names the folder of a pull request and of main", () => {
    expect(previewFolder({ pr: 12 })).toBe("pr-preview/pr-12");
    expect(previewFolder("main")).toBe("storybook");
  });
});

describe("placePreview", () => {
  it("replaces an older preview and keeps no file of it", () => {
    const dir = site();
    placePreview(dir, build("old"), "pr-preview/pr-8");
    placePreview(dir, build("new"), "pr-preview/pr-8");

    expect(filesUnder(path.join(dir, "pr-preview/pr-8"))).toEqual([
      "assets/new.js",
      "iframe.html",
      "index.html",
    ]);
    expect(
      fs.readFileSync(path.join(dir, "pr-preview/pr-8/index.html"), "utf8"),
    ).toBe("manager new");
  });

  it("leaves every other path of the site unchanged", () => {
    const dir = site();
    const before = snapshot(dir, "pr-preview/pr-8");

    placePreview(dir, build("new"), "pr-preview/pr-8");

    expect(snapshot(dir, "pr-preview/pr-8")).toEqual(before);
  });

  it("places the Storybook of main and leaves the previews unchanged", () => {
    const dir = site();
    const before = snapshot(dir, "storybook");

    placePreview(dir, build("main"), "storybook");

    expect(snapshot(dir, "storybook")).toEqual(before);
    expect(
      fs.readFileSync(path.join(dir, "storybook/index.html"), "utf8"),
    ).toBe("manager main");
  });

  it("removes project.json", () => {
    const dir = site();
    const from = build("new");

    placePreview(dir, from, "pr-preview/pr-8");

    expect(fs.existsSync(path.join(dir, "pr-preview/pr-8/project.json"))).toBe(
      false,
    );
    expect(fs.existsSync(path.join(from, "project.json"))).toBe(true);
  });

  it.each(refusedFolders)(
    "refuses the folder %j outside the preview folders",
    (folder) => {
      const dir = site();
      const before = snapshot(dir, "");

      expect(() => placePreview(dir, build("new"), folder)).toThrow(
        /not a preview folder/,
      );
      expect(snapshot(dir, "")).toEqual(before);
    },
  );

  it.each(["index.html", "iframe.html"])(
    "refuses a source without %s",
    (missing) => {
      const dir = site();
      const from = build("new");
      fs.rmSync(path.join(from, missing));

      expect(() => placePreview(dir, from, "pr-preview/pr-7")).toThrow(missing);
      expect(
        fs.readFileSync(path.join(dir, "pr-preview/pr-7/index.html"), "utf8"),
      ).toBe("preview 7");
    },
  );
});

describe("removePreview", () => {
  it("removes only that folder and says whether it existed", () => {
    const dir = site();
    const before = snapshot(dir, "pr-preview/pr-7");

    expect(removePreview(dir, "pr-preview/pr-7")).toBe(true);
    expect(fs.existsSync(path.join(dir, "pr-preview/pr-7"))).toBe(false);
    expect(snapshot(dir, "pr-preview/pr-7")).toEqual(before);

    expect(removePreview(dir, "pr-preview/pr-7")).toBe(false);
  });

  it.each(refusedFolders)(
    "refuses the folder %j outside the preview folders",
    (folder) => {
      const dir = site();
      const before = snapshot(dir, "");

      expect(() => removePreview(dir, folder)).toThrow(/not a preview folder/);
      expect(snapshot(dir, "")).toEqual(before);
    },
  );
});

describe("stalePreviews", () => {
  it("lists the folders of closed pull requests in the order of their numbers", () => {
    const dir = tempDir("site-");
    for (const n of [12, 3, 10])
      write(dir, `pr-preview/pr-${n}/index.html`, "");

    expect(stalePreviews(dir, [10])).toEqual([
      "pr-preview/pr-3",
      "pr-preview/pr-12",
    ]);
  });

  it("skips an entry that is not the folder of a pull request", () => {
    const dir = tempDir("site-");
    write(dir, "pr-preview/pr-0/index.html", "");
    write(dir, "pr-preview/notes/index.html", "");
    write(dir, "pr-preview/pr-4", "a file");

    expect(stalePreviews(dir, [])).toEqual([]);
  });

  it("returns nothing when there is no pr-preview folder", () => {
    expect(stalePreviews(tempDir("site-"), [1, 2])).toEqual([]);
  });
});

describe("previewComment", () => {
  const url = "https://konsole-is.github.io/camunda-operator/pr-preview/pr-8/";
  const sha = "0123456789abcdef0123456789abcdef01234567";

  it("starts with the marker and holds the URL and the short SHA", () => {
    const body = previewComment("published", url, sha);

    expect(body.startsWith(`${PREVIEW_MARKER}\n`)).toBe(true);
    expect(body).toContain(url);
    expect(body).toContain("0123456");
    expect(body).not.toContain(sha);
  });

  it("says that the preview is gone after the pull request closed", () => {
    const body = previewComment("removed", url, sha);

    expect(body.startsWith(`${PREVIEW_MARKER}\n`)).toBe(true);
    expect(body).toMatch(/removed/);
    expect(body).not.toContain(`](${url})`);
  });

  it("names no commit after the pull request closed", () => {
    const body = previewComment("removed", url, sha);

    expect(body).not.toContain("0123456");
  });
});

// The global git config of the machine, such as core.hooksPath, must not
// change what the git commands of the tests and of squashSite do.
function isolateGit(): void {
  const empty = path.join(tempDir("gitconfig-"), "config");
  fs.writeFileSync(empty, "");
  vi.stubEnv("GIT_CONFIG_GLOBAL", empty);
  vi.stubEnv("GIT_CONFIG_NOSYSTEM", "1");
}

function git(dir: string, ...args: string[]): string {
  return execFileSync("git", ["-C", dir, ...args], {
    encoding: "utf8",
  }).trim();
}

function commitAll(dir: string, message: string): void {
  git(dir, "add", "-A");
  git(dir, "commit", "-q", "-m", message);
}

/** A clone of remote on gh-pages with an author, as the workflow makes it. */
function clone(remote: string, depth?: number): string {
  const dir = tempDir("clone-");
  const args = ["clone", "-q", "--branch", "gh-pages"];
  if (depth !== undefined) args.push(`--depth=${depth}`);
  execFileSync("git", [...args, `file://${remote}`, dir]);
  git(dir, "config", "user.name", "test");
  git(dir, "config", "user.email", "test@example.com");
  return dir;
}

/** A bare remote whose gh-pages has three commits, the last with the files of site(). */
function remoteWithHistory(): string {
  const remote = tempDir("remote-");
  git(remote, "init", "-q", "--bare", "--initial-branch=gh-pages");
  const work = site();
  git(work, "init", "-q", "--initial-branch=gh-pages");
  git(work, "config", "user.name", "test");
  git(work, "config", "user.email", "test@example.com");
  write(work, "pr-preview/pr-3/index.html", "preview 3");
  commitAll(work, "Publish the Storybook of pull request 3");
  fs.rmSync(path.join(work, "pr-preview/pr-3"), { recursive: true });
  commitAll(work, "Remove the Storybook of pull request 3");
  write(work, "dev/index.html", "dev docs, newer");
  commitAll(work, "Deploy dev");
  git(work, "push", "-q", `file://${remote}`, "gh-pages");
  return remote;
}

/**
 * Makes another writer push a new commit to gh-pages just before each of the
 * first `times` pushes from site start, after site read the branch, and
 * returns the clone of that writer. It puts a git in front of the real one on
 * PATH.
 */
function raceEachPush(remote: string, site: string, times: number): string {
  const writer = clone(remote);
  const real = execFileSync("sh", ["-c", "command -v git"], {
    encoding: "utf8",
  }).trim();
  const bin = tempDir("bin-");
  const count = path.join(bin, "count");
  fs.writeFileSync(
    path.join(bin, "git"),
    `#!/bin/sh
if [ "$1" = -C ] && [ "$2" = "${site}" ] && [ "$3" = push ]; then
  n=$(cat "${count}" 2>/dev/null || echo 0)
  if [ "$n" -lt ${times} ]; then
    n=$((n + 1))
    echo "$n" > "${count}"
    "${real}" -C "${writer}" pull -q --rebase origin gh-pages
    echo "push $n" > "${writer}/writer-$n.txt"
    "${real}" -C "${writer}" add -A
    "${real}" -C "${writer}" commit -q -m "Writer push $n"
    "${real}" -C "${writer}" push -q origin HEAD:gh-pages
  fi
fi
exec "${real}" "$@"
`,
  );
  fs.chmodSync(path.join(bin, "git"), 0o755);
  vi.stubEnv("PATH", `${bin}${path.delimiter}${process.env.PATH ?? ""}`);
  return writer;
}

describe("squashSite", () => {
  beforeEach(isolateGit);
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("leaves gh-pages with one commit that holds the tree of its head", () => {
    const remote = remoteWithHistory();
    const tree = git(remote, "rev-parse", "gh-pages^{tree}");

    expect(squashSite(clone(remote, 1))).toBe(true);

    expect(git(remote, "rev-list", "--count", "gh-pages")).toBe("1");
    expect(git(remote, "rev-parse", "gh-pages^{tree}")).toBe(tree);
  });

  it("returns false and pushes nothing when gh-pages has one commit", () => {
    const remote = remoteWithHistory();
    squashSite(clone(remote, 1));
    const head = git(remote, "rev-parse", "gh-pages");

    expect(squashSite(clone(remote, 1))).toBe(false);

    expect(git(remote, "rev-parse", "gh-pages")).toBe(head);
  });

  it("squashes the commits that landed after the clone", () => {
    const remote = remoteWithHistory();
    const dir = clone(remote, 1);
    const writer = clone(remote);
    write(writer, "pr-preview/pr-9/index.html", "preview 9");
    commitAll(writer, "Publish the Storybook of pull request 9");
    git(writer, "push", "-q", "origin", "gh-pages");

    expect(squashSite(dir)).toBe(true);

    expect(git(remote, "rev-list", "--count", "gh-pages")).toBe("1");
    expect(git(remote, "show", "gh-pages:pr-preview/pr-9/index.html")).toBe(
      "preview 9",
    );
  });

  it("keeps the commit that another writer pushes during the squash", () => {
    const remote = remoteWithHistory();
    const dir = clone(remote, 1);
    raceEachPush(remote, dir, 1);

    expect(squashSite(dir)).toBe(true);

    expect(git(remote, "rev-list", "--count", "gh-pages")).toBe("1");
    expect(git(remote, "show", "gh-pages:writer-1.txt")).toBe("push 1");
  });

  it("throws after the last attempt and keeps the history of the other writer", () => {
    const remote = remoteWithHistory();
    const dir = clone(remote, 1);
    const writer = raceEachPush(remote, dir, 2);

    expect(() => squashSite(dir, 2)).toThrow(/2 attempts/);

    expect(git(remote, "rev-parse", "gh-pages")).toBe(
      git(writer, "rev-parse", "HEAD"),
    );
    expect(git(remote, "rev-list", "--count", "gh-pages")).toBe("5");
  });
});

describe("the command line", () => {
  function run(...args: string[]) {
    return spawnSync(process.execPath, [script, ...args], {
      encoding: "utf8",
    });
  }

  it("places a preview and prints the stale folders one on each line", () => {
    const dir = site();
    write(dir, "pr-preview/pr-9/index.html", "preview 9");

    execFileSync(process.execPath, [
      script,
      "place",
      dir,
      build("new"),
      "pr-preview/pr-8",
    ]);
    const stale = run("stale", dir, "8");

    expect(fs.existsSync(path.join(dir, "pr-preview/pr-8/iframe.html"))).toBe(
      true,
    );
    expect(stale.status).toBe(0);
    expect(stale.stdout).toBe("pr-preview/pr-7\npr-preview/pr-9\n");
  });

  it("exits 1 and names the folder that it refuses", () => {
    const dir = site();

    const result = run("remove", dir, "dev");

    expect(result.status).toBe(1);
    expect(result.stderr).toContain('"dev"');
    expect(fs.existsSync(path.join(dir, "dev/index.html"))).toBe(true);
  });

  it("refuses a number that is not a pull request number", () => {
    const result = run("stale", site(), "8x");

    expect(result.status).toBe(1);
    expect(result.stderr).toContain('"8x"');
  });

  it("squashes the history of the site and says whether it did", () => {
    isolateGit();
    try {
      const dir = clone(remoteWithHistory(), 1);

      const first = run("squash", dir);
      const second = run("squash", dir);

      expect(first.status).toBe(0);
      expect(second.status).toBe(0);
      expect(first.stdout).not.toBe(second.stdout);
    } finally {
      vi.unstubAllEnvs();
    }
  });

  it("prints the comment", () => {
    const result = run(
      "comment",
      "published",
      "https://example.com/",
      "abcdef0123",
    );

    expect(result.status).toBe(0);
    expect(result.stdout).toBe(
      previewComment("published", "https://example.com/", "abcdef0123"),
    );
  });
});
