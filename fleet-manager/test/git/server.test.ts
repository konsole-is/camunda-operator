import fs from "node:fs";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import git from "isomorphic-git";
import gitHttp from "isomorphic-git/http/node";
import { afterEach, describe, expect, it } from "vitest";
import { type BareRepository, serveBareRepository } from "./server";

let repo: BareRepository | undefined;
let dir: string | undefined;

afterEach(async () => {
  await repo?.close();
  if (dir) await rm(dir, { recursive: true, force: true });
  repo = dir = undefined;
});

const author = { name: "test", email: "test@example.com" };

describe("serveBareRepository", () => {
  it("an isomorphic-git clone and push reach the bare repository", async () => {
    repo = await serveBareRepository();
    dir = await mkdtemp(path.join(os.tmpdir(), "fleet-clone-"));

    await git.clone({ fs, http: gitHttp, dir, url: repo.url });
    await writeFile(path.join(dir, "a.txt"), "hello\n");
    await git.add({ fs, dir, filepath: "a.txt" });
    await git.commit({ fs, dir, message: "Add a", author });
    await git.push({ fs, http: gitHttp, dir, url: repo.url });

    expect(await repo.log()).toEqual(["Add a", "Initial commit"]);
    expect(await repo.show("a.txt")).toBe("hello\n");
    expect(await repo.files()).toEqual(["README.md", "a.txt"]);
  });

  it("asks for the credentials of auth", async () => {
    repo = await serveBareRepository({
      auth: { username: "u", token: "t" },
    });
    dir = await mkdtemp(path.join(os.tmpdir(), "fleet-clone-"));

    await expect(
      git.clone({
        fs,
        http: gitHttp,
        dir,
        url: repo.url,
        onAuthFailure: () => ({ cancel: true }),
      }),
    ).rejects.toThrow();
    await git.clone({
      fs,
      http: gitHttp,
      dir,
      url: repo.url,
      onAuth: () => ({ username: "u", password: "t" }),
    });
  });

  it("runs beforeFirstPush before the first push lands", async () => {
    repo = await serveBareRepository({
      beforeFirstPush: () =>
        repo!.commitFromOtherClone({ "other.txt": "x" }, "Other admin"),
    });
    dir = await mkdtemp(path.join(os.tmpdir(), "fleet-clone-"));
    await git.clone({ fs, http: gitHttp, dir, url: repo.url });
    await writeFile(path.join(dir, "a.txt"), "hello\n");
    await git.add({ fs, dir, filepath: "a.txt" });
    await git.commit({ fs, dir, message: "Add a", author });

    await expect(
      git.push({ fs, http: gitHttp, dir, url: repo.url }),
    ).rejects.toThrow();
    expect(await repo.log()).toEqual(["Other admin", "Initial commit"]);
  });
});
