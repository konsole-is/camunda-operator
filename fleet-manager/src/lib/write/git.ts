import "server-only";
import fs from "node:fs";
import { lstat, mkdir, mkdtemp, rm, stat, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import git, { type HttpClient } from "isomorphic-git";
import gitHttp from "isomorphic-git/http/node";
import { inOrder } from "@/lib/serial";
import { readSecretValues } from "@/lib/secrets/values";
import { appNamespace } from "@/lib/settings/keys";
import { apiErrorsOf, reasonOf, staleError } from "./errors";
import { gitBase, gitPath, renderObject } from "./git-files";
import type {
  ApiError,
  Change,
  KubeObject,
  ObjectRef,
  Preview,
  PreviewObject,
  RefKey,
  WriteResult,
  Writer,
} from "./types";
import {
  kindInfoOf,
  previewObject,
  refKey,
  refOf,
  refsOf,
  refusalOf,
} from "./writer";

/** GitTarget is the repository of the settings. branch "" is the default branch of the remote. */
export type GitTarget = {
  url: string;
  branch: string;
  path: string;
  credentialsSecretRef: string;
};

/**
 * GitWriterOptions are the parts of the Git writer. direct runs the dry
 * runs and the directOnly changes. read returns the live object of a ref,
 * or undefined when it does not exist. http is the Git transport; the
 * default follows no redirect and gives up on a request after 30 s.
 */
export type GitWriterOptions = {
  target: GitTarget;
  direct: Writer;
  read: (ref: ObjectRef) => Promise<KubeObject | undefined>;
  http?: HttpClient;
};

/** GIT_AUTHOR is the author of each commit of the Git writer. */
export const GIT_AUTHOR = {
  name: "fleet-manager",
  email: "fleet-manager@noreply.invalid",
};

const REQUEST_TIMEOUT_MS = 30_000;

// The transport keeps the credentials on a redirect, also to http://, so
// it follows none.
const defaultHttp: HttpClient = {
  request: (request) =>
    gitHttp.request({
      ...request,
      signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
      fetchOptions: { maxRedirects: 0 },
    } as Parameters<HttpClient["request"]>[0]),
};

type Remote = {
  fs: typeof fs;
  http: HttpClient;
  dir: string;
  url: string;
  onAuth?: () => { username: string; password: string };
  onAuthFailure: () => { cancel: true };
};

/**
 * gitWriter returns the writer of Git mode. A directOnly change goes to
 * direct. Any other change becomes one commit on the branch of target,
 * with the intent as its subject and GIT_AUTHOR as its author, and the
 * result carries the commit, the revision, and the file of each applied
 * object. A change whose files are already in the repository makes no
 * commit and returns the head commit.
 *
 * apply makes no commit when the fleet manager does not write a ref, a
 * Secret is in the change (Invalid), an object with a precondition changed
 * or was deleted in the cluster (Stale), or the dry run against the
 * cluster fails. The dry run skips an ExternalSecret and an object in a
 * namespace that the same change creates; the preview shows them as
 * rendered. A delete of an object without a file, and a file behind a
 * symbolic link of the repository, are Invalid.
 *
 * When another push lands first, apply fetches and commits again on top
 * of it once. It fails with Conflict when that push changed a file of the
 * change, or when the branch moved again. The credentials of
 * target.credentialsSecretRef go only to an https:// address. No error
 * holds a credential or the text of a Git server answer. Changes to one
 * repository and branch run one after the other in this server process.
 */
export function gitWriter(options: GitWriterOptions): Writer {
  const { target, direct, read } = options;
  const http = options.http ?? defaultHttp;

  const preview = async (change: Change): Promise<Preview> => {
    if (change.directOnly) return direct.preview(change);
    const refusals = refsOf(change).flatMap(gitRefusalOf);
    if (refusals.length > 0) return { objects: [], errors: refusals };
    const skipped = await notDryRun(change);
    const deletes = change.delete ?? [];
    // force: GitOps applies the objects under its own field manager, so a
    // dry run without force fails on each field that GitOps set.
    const checked: Change = {
      ...change,
      force: true,
      apply: change.apply.filter((o) => !skipped.has(o)),
      delete: deletes.filter((r) => r.kind !== "ExternalSecret"),
    };
    const result =
      checked.apply.length + checked.delete!.length > 0
        ? await direct.preview(checked)
        : { objects: [], errors: [] };
    const shown: PreviewObject[] = [...skipped].map((o) =>
      previewObject(refOf(o), undefined, o),
    );
    for (const ref of deletes) {
      if (ref.kind !== "ExternalSecret") continue;
      shown.push(previewObject(ref, await read(ref), undefined));
    }
    return { objects: [...result.objects, ...shown], errors: result.errors };
  };

  // notDryRun returns the objects that the API server cannot dry-run: an
  // ExternalSecret, which the app may not write, and an object in a
  // namespace that only this change creates.
  const notDryRun = async (change: Change): Promise<Set<KubeObject>> => {
    const created = new Set<string>();
    for (const object of change.apply) {
      if (object.kind !== "Namespace") continue;
      const exists = await read(refOf(object));
      if (exists === undefined) created.add(object.metadata.name);
    }
    return new Set(
      change.apply.filter(
        (o) =>
          o.kind === "ExternalSecret" ||
          (o.metadata.namespace !== undefined &&
            created.has(o.metadata.namespace)),
      ),
    );
  };

  const staleErrors = async (change: Change): Promise<ApiError[]> => {
    const errors: ApiError[] = [];
    for (const ref of refsOf(change)) {
      const expected = change.preconditions?.[refKey(ref)];
      if (expected === undefined) continue;
      try {
        const object = await read(ref);
        if (object?.metadata.resourceVersion !== expected) {
          errors.push(staleError(ref));
        }
      } catch (error) {
        errors.push(...apiErrorsOf(error, ref, false));
      }
    }
    return errors;
  };

  const apply = async (change: Change): Promise<WriteResult> => {
    if (change.directOnly) return direct.apply(change);
    return inOrder(`git ${target.url} ${target.branch}`, async () => {
      const refusals = refsOf(change).flatMap(gitRefusalOf);
      if (refusals.length > 0) return { ok: false, errors: refusals };
      const stale = await staleErrors(change);
      if (stale.length > 0) return { ok: false, errors: stale };
      const { errors } = await preview(change);
      if (errors.length > 0) return { ok: false, errors };
      return commitChange(change);
    });
  };

  const commitChange = async (change: Change): Promise<WriteResult> => {
    const base = gitBase(target.path);
    if (base === undefined) {
      return failure({
        code: "Invalid",
        path: "git.path",
        message: "git.path must be a folder inside the repository.",
      });
    }
    const auth = await credentials();
    if (!auth.ok) return failure(auth.error);

    const dir = await mkdtemp(path.join(os.tmpdir(), "fleet-git-"));
    try {
      const remote: Remote = {
        fs,
        http,
        dir,
        url: target.url,
        ...(auth.values
          ? {
              onAuth: () => ({
                username: auth.values!.username,
                password: auth.values!.token,
              }),
            }
          : {}),
        // Without cancel, a refused token makes isomorphic-git ask again.
        onAuthFailure: () => ({ cancel: true }),
      };
      return await pushChange(remote, base, change);
    } catch (error) {
      return failure(gitErrorOf(error, target));
    } finally {
      await rm(dir, { recursive: true, force: true });
    }
  };

  const credentials = async (): Promise<
    | { ok: true; values?: { username: string; token: string } }
    | { ok: false; error: ApiError }
  > => {
    const secret = target.credentialsSecretRef;
    if (!secret) return { ok: true };
    if (URL.parse(target.url)?.protocol !== "https:") {
      return {
        ok: false,
        error: {
          code: "Invalid",
          path: "git.repository",
          message: `${target.url} must start with https:// to use the credentials of the Secret ${secret}.`,
        },
      };
    }
    const namespace = appNamespace();
    let values: Record<string, string> | undefined;
    try {
      values = await readSecretValues(namespace, secret);
    } catch (error) {
      return {
        ok: false,
        error: {
          code: "Unavailable",
          message: `Could not read the Secret ${secret} in namespace ${namespace}: ${reasonOf(error)}`,
        },
      };
    }
    if (!values) {
      return {
        ok: false,
        error: {
          code: "Unavailable",
          message: `The Secret ${secret} does not exist in namespace ${namespace}.`,
        },
      };
    }
    for (const key of ["username", "token"]) {
      if (!values[key]) {
        return {
          ok: false,
          error: {
            code: "Unavailable",
            message: `The Secret ${secret} has no key ${key}.`,
          },
        };
      }
    }
    return {
      ok: true,
      values: { username: values.username, token: values.token },
    };
  };

  const pushChange = async (
    remote: Remote,
    base: string,
    change: Change,
  ): Promise<WriteResult> => {
    const { dir } = remote;
    await git.clone({
      ...remote,
      singleBranch: true,
      ...(target.branch ? { ref: target.branch } : {}),
    });
    const branch = (await git.currentBranch({ fs, dir })) ?? target.branch;
    const cloned = await git.resolveRef({ fs, dir, ref: "HEAD" });

    const written = await writeFiles(dir, base, change);
    if (!written.ok) return failure(...written.errors);
    const { revisions, paths, files } = written;
    if (!(await changed(dir, files))) {
      return { ok: true, mode: "git", commit: cloned, revisions, paths };
    }
    const commitAndPush = async () => {
      const commit = await git.commit({
        fs,
        dir,
        message: change.intent,
        author: GIT_AUTHOR,
      });
      await git.push({ ...remote, ref: branch });
      return commit;
    };

    try {
      const commit = await commitAndPush();
      return { ok: true, mode: "git", commit, revisions, paths };
    } catch (error) {
      if (!isRejected(error)) throw error;
    }

    await git.fetch({ ...remote, ref: branch, singleBranch: true });
    const fetched = await git.resolveRef({
      fs,
      dir,
      ref: `refs/remotes/origin/${branch}`,
    });
    // The branch did not move, so the server refused the push itself, as a
    // protected branch does.
    if (fetched === cloned) {
      return failure({
        code: "Unavailable",
        message: `${target.url} refused the push to the branch ${branch}.`,
      });
    }
    for (const file of files) {
      if (
        (await blobOf(dir, cloned, file)) !== (await blobOf(dir, fetched, file))
      ) {
        return failure({
          code: "Conflict",
          message: `Someone changed ${file} in ${target.url} since the fleet manager read it. Open the page again.`,
        });
      }
    }
    await git.writeRef({
      fs,
      dir,
      ref: `refs/heads/${branch}`,
      value: fetched,
      force: true,
    });
    await git.checkout({ fs, dir, ref: branch, force: true });
    const again = await writeFiles(dir, base, change);
    if (!again.ok) return failure(...again.errors);
    try {
      const commit = await commitAndPush();
      return { ok: true, mode: "git", commit, revisions, paths };
    } catch (error) {
      if (!isRejected(error)) throw error;
      return failure({
        code: "Conflict",
        message: `${target.url} changed again while the fleet manager pushed. Save again.`,
      });
    }
  };

  return { mode: "git", preview, apply };
}

type Written =
  | {
      ok: true;
      revisions: Record<RefKey, string>;
      paths: Record<RefKey, string>;
      files: string[];
    }
  | { ok: false; errors: ApiError[] };

// writeFiles writes and stages the files of change in the clone at dir.
async function writeFiles(
  dir: string,
  base: string,
  change: Change,
): Promise<Written> {
  const revisions: Record<RefKey, string> = {};
  const paths: Record<RefKey, string> = {};
  const files: string[] = [];
  const errors: ApiError[] = [];
  for (const object of change.apply) {
    const ref = refOf(object);
    const file = gitPath(base, kindInfoOf(ref.kind)!, ref);
    const { text, revision } = renderObject(object);
    if (await throughLink(dir, file)) {
      errors.push(linkError(file, ref));
      continue;
    }
    await mkdir(path.dirname(path.join(dir, file)), { recursive: true });
    await writeFile(path.join(dir, file), text);
    await git.add({ fs, dir, filepath: file });
    revisions[refKey(ref)] = revision;
    paths[refKey(ref)] = file;
    files.push(file);
  }
  for (const ref of change.delete ?? []) {
    const file = gitPath(base, kindInfoOf(ref.kind)!, ref);
    if (await throughLink(dir, file)) {
      errors.push(linkError(file, ref));
      continue;
    }
    if (!(await exists(path.join(dir, file)))) {
      errors.push({
        code: "Invalid",
        message: `${file} is not in the repository. GitOps does not manage ${ref.kind} ${ref.name}.`,
        ref,
      });
      continue;
    }
    await rm(path.join(dir, file));
    await git.remove({ fs, dir, filepath: file });
    files.push(file);
  }
  if (errors.length > 0) return { ok: false, errors };
  return { ok: true, revisions, paths, files };
}

// changed reports whether a file of files differs from the head commit.
async function changed(dir: string, files: string[]): Promise<boolean> {
  const rows = await git.statusMatrix({ fs, dir, filepaths: files });
  return rows.some(
    ([, head, workdir, stage]) => !(head === 1 && workdir === 1 && stage === 1),
  );
}

async function blobOf(
  dir: string,
  commit: string,
  file: string,
): Promise<string | undefined> {
  try {
    return (await git.readBlob({ fs, dir, oid: commit, filepath: file })).oid;
  } catch (error) {
    if ((error as { code?: string }).code === "NotFoundError") return undefined;
    throw error;
  }
}

// A symbolic link in the repository can point out of the clone, and a
// write through it would change a file of the app server.
async function throughLink(dir: string, file: string): Promise<boolean> {
  let current = dir;
  for (const segment of file.split("/")) {
    current = path.join(current, segment);
    try {
      if ((await lstat(current)).isSymbolicLink()) return true;
    } catch {
      return false;
    }
  }
  return false;
}

function linkError(file: string, ref: ObjectRef): ApiError {
  return {
    code: "Invalid",
    message: `${file} is behind a symbolic link in the repository. The fleet manager does not write through a link.`,
    ref,
  };
}

async function exists(file: string): Promise<boolean> {
  try {
    await stat(file);
    return true;
  } catch {
    return false;
  }
}

// A push is rejected when the branch moved since the clone: isomorphic-git
// finds it before it sends the pack, or the server refuses the update.
function isRejected(error: unknown): boolean {
  const code = (error as { code?: string }).code;
  return code === "PushRejectedError" || code === "GitPushError";
}

function gitRefusalOf(ref: ObjectRef): ApiError[] {
  const refusals = refusalOf(ref);
  if (refusals.length > 0) return refusals;
  // The name and the namespace are segments of the file path, and the dry
  // run that would refuse such a name skips some objects.
  if ([ref.name, ref.namespace].some((part) => /[/\\]/.test(part ?? ""))) {
    return [
      {
        code: "Invalid",
        message: `The name or the namespace of the ${ref.kind} is not a valid name.`,
        ref,
      },
    ];
  }
  if (ref.kind !== "Secret") return refusals;
  return [
    {
      code: "Invalid",
      message:
        "A Secret never goes to Git. The fleet manager commits an ExternalSecret for it.",
      ref,
    },
  ];
}

function gitErrorOf(error: unknown, target: GitTarget): ApiError {
  const status = (error as { data?: { statusCode?: number } }).data?.statusCode;
  const code = (error as { code?: string }).code;
  if (code === "UserCanceledError" || status === 401 || status === 403) {
    return {
      code: "Forbidden",
      message: `${target.url} refused the credentials.`,
    };
  }
  if (code === "NotFoundError" && target.branch) {
    return {
      code: "Unavailable",
      message: `${target.url} has no branch ${target.branch}.`,
    };
  }
  return {
    code: "Unavailable",
    message: `Could not reach ${target.url}: ${reasonOf(error)}`,
  };
}

function failure(...errors: ApiError[]): WriteResult {
  return { ok: false, errors };
}
