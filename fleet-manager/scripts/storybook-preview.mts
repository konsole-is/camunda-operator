// Places and removes the Storybook previews on a checkout of the gh-pages
// branch, squashes the history of the branch, and writes the text of the
// preview comment. The workflows run it with plain Node.js and no install, so
// it imports Node.js built-ins only.
//
// Usage:
//   node scripts/storybook-preview.mts place <site> <from> <folder>
//   node scripts/storybook-preview.mts remove <site> <folder>
//   node scripts/storybook-preview.mts stale <site> [<open pull request number>...]
//   node scripts/storybook-preview.mts squash <site>
//   node scripts/storybook-preview.mts comment published <url> <sha>
//   node scripts/storybook-preview.mts comment removed <url>
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

/** PREVIEW_MARKER starts the preview comment, so a later run finds it. */
export const PREVIEW_MARKER = "<!-- storybook-preview -->";

// mike owns every other path of gh-pages: the docs versions, latest,
// versions.json, index.html, and .nojekyll.
const PREVIEW_FOLDER = /^(pr-preview\/pr-[1-9][0-9]*|storybook)$/;
const PULL_REQUEST_FOLDER = /^pr-([1-9][0-9]*)$/;

/** previewFolder returns the folder, relative to the site, of a pull request or of main. */
export function previewFolder(target: { pr: number } | "main"): string {
  return target === "main" ? "storybook" : `pr-preview/pr-${target.pr}`;
}

function requirePreviewFolder(folder: string): void {
  if (!PREVIEW_FOLDER.test(folder)) {
    throw new Error(
      `${JSON.stringify(folder)} is not a preview folder: use pr-preview/pr-<number> or storybook`,
    );
  }
}

/**
 * placePreview replaces siteDir/folder with a copy of the Storybook build in
 * fromDir, without its project.json. It throws, and changes nothing, when
 * folder is not pr-preview/pr-<number> or storybook, or when fromDir has no
 * index.html or no iframe.html.
 */
export function placePreview(
  siteDir: string,
  fromDir: string,
  folder: string,
): void {
  requirePreviewFolder(folder);
  for (const file of ["index.html", "iframe.html"]) {
    if (!fs.existsSync(path.join(fromDir, file))) {
      throw new Error(`${fromDir} is not a Storybook build: it has no ${file}`);
    }
  }
  const target = path.join(siteDir, folder);
  fs.rmSync(target, { recursive: true, force: true });
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.cpSync(fromDir, target, { recursive: true });
  // project.json holds the time of the build, so it changes on each build.
  fs.rmSync(path.join(target, "project.json"), { force: true });
}

/**
 * removePreview removes siteDir/folder and returns false when it did not
 * exist. It throws, and changes nothing, when folder is not
 * pr-preview/pr-<number> or storybook.
 */
export function removePreview(siteDir: string, folder: string): boolean {
  requirePreviewFolder(folder);
  const target = path.join(siteDir, folder);
  if (!fs.existsSync(target)) return false;
  fs.rmSync(target, { recursive: true, force: true });
  return true;
}

/**
 * stalePreviews returns the pr-preview/pr-<number> folders of siteDir whose
 * number is not in openPullRequests, in the order of their numbers.
 */
export function stalePreviews(
  siteDir: string,
  openPullRequests: readonly number[],
): string[] {
  const root = path.join(siteDir, "pr-preview");
  if (!fs.existsSync(root)) return [];
  const open = new Set(openPullRequests);
  return fs
    .readdirSync(root, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => PULL_REQUEST_FOLDER.exec(entry.name))
    .filter((match) => match !== null)
    .map((match) => Number(match[1]))
    .filter((n) => !open.has(n))
    .sort((a, b) => a - b)
    .map((n) => previewFolder({ pr: n }));
}

function git(siteDir: string, ...args: string[]): string {
  return execFileSync("git", ["-C", siteDir, ...args], {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  }).trim();
}

/**
 * squashSite replaces the history of gh-pages on the remote origin of
 * siteDir with one commit that holds the tree of its head, and returns false
 * when the branch already has one commit. siteDir is a clone, shallow or not,
 * that can push, with a commit author in its config. When another push lands
 * during the squash, it reads the branch again, for a maximum of attempts
 * reads. It never removes a commit that it did not read, and it throws when
 * the last attempt fails.
 */
export function squashSite(siteDir: string, attempts = 3): boolean {
  let failure = "";
  for (let attempt = 1; attempt <= attempts; attempt++) {
    git(
      siteDir,
      "fetch",
      "--quiet",
      "--depth=1",
      "origin",
      "+refs/heads/gh-pages:refs/remotes/origin/gh-pages",
    );
    const head = git(siteDir, "rev-parse", "refs/remotes/origin/gh-pages");
    // A shallow clone hides the parents from the history, but the commit
    // object still names them.
    if (!/^parent /m.test(git(siteDir, "cat-file", "commit", head))) {
      return false;
    }
    const root = git(
      siteDir,
      "commit-tree",
      `${head}^{tree}`,
      "-m",
      "Squash the history of gh-pages into one commit",
    );
    try {
      git(
        siteDir,
        "push",
        "--quiet",
        `--force-with-lease=refs/heads/gh-pages:${head}`,
        "origin",
        `${root}:refs/heads/gh-pages`,
      );
      return true;
    } catch (error) {
      failure = String((error as { stderr?: unknown }).stderr ?? error).trim();
    }
  }
  throw new Error(
    `Could not push the squashed gh-pages in ${attempts} attempts: ${failure}`,
  );
}

/**
 * previewComment returns the body of the preview comment. It starts with
 * PREVIEW_MARKER. A published comment names the commit by its short SHA. A
 * removed comment names no commit and ignores sha.
 */
export function previewComment(
  state: "published" | "removed",
  url: string,
  sha: string,
): string {
  if (state === "published") {
    return `${PREVIEW_MARKER}
**Storybook preview:** [${url}](${url})

Built from ${sha.slice(0, 7)}. Each push to this pull request replaces the preview. After a push, GitHub Pages can take a minute to show the new build.
`;
  }
  return `${PREVIEW_MARKER}
**Storybook preview:** removed

The pull request closed, so the workflow removed its Storybook preview.
`;
}

function pullRequestNumber(text: string): number {
  if (!/^[1-9][0-9]*$/.test(text)) {
    throw new Error(
      `${JSON.stringify(text)} is not the number of a pull request`,
    );
  }
  return Number(text);
}

function main(args: string[]): void {
  const [command, ...rest] = args;
  switch (command) {
    case "place": {
      const [site, from, folder] = rest;
      placePreview(site ?? "", from ?? "", folder ?? "");
      return;
    }
    case "remove": {
      const [site, folder] = rest;
      const removed = removePreview(site ?? "", folder ?? "");
      console.log(removed ? `Removed ${folder}` : `${folder} does not exist`);
      return;
    }
    case "stale": {
      const [site, ...open] = rest;
      const folders = stalePreviews(site ?? "", open.map(pullRequestNumber));
      for (const folder of folders) console.log(folder);
      return;
    }
    case "squash": {
      const [site] = rest;
      console.log(
        squashSite(site ?? "")
          ? "Squashed the history of gh-pages into one commit"
          : "gh-pages has one commit",
      );
      return;
    }
    case "comment": {
      const [state, url, sha] = rest;
      if (state !== "published" && state !== "removed") {
        throw new Error(
          `${JSON.stringify(state)} is not a comment state: use published or removed`,
        );
      }
      process.stdout.write(previewComment(state, url ?? "", sha ?? ""));
      return;
    }
    default:
      throw new Error(
        `${JSON.stringify(command)} is not a command: use place, remove, stale, squash, or comment`,
      );
  }
}

if (import.meta.main) {
  try {
    main(process.argv.slice(2));
  } catch (error) {
    console.error((error as Error).message);
    process.exit(1);
  }
}
