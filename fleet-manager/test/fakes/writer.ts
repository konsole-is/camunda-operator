// FakeWriter stands in for the writer of the app in a test, so that a test
// sees each Change that a page sends and scripts what comes back. A test
// replaces the writer module with this one:
//
//   vi.mock("@/lib/write/writer", () => import("<relative>/test/fakes/writer"))
//
// The module then serves only the path of runChange and runPreview. A test
// that needs refKey imports it from @/lib/write/refs. A test resets
// fakeWriter before each test.
import { createHash } from "node:crypto";
import { refKey } from "@/lib/write/refs";
import type {
  Change,
  Preview,
  RefKey,
  WriteMode,
  WriteResult,
  Writer,
} from "@/lib/write/types";

/**
 * FakeWriter is a Writer that records each Change and returns scripted
 * results. Without a script, apply succeeds in its mode, and in Git mode it
 * makes a new commit, gives each applied object a revision that changes
 * with its content, gives each applied object a path in paths and each
 * deleted object a path in deleted, both
 * <namespace>/<kind in lower case>/<name>.yaml. A directOnly change gets
 * the direct result in Git mode too. Without a script, preview returns no
 * objects and no errors.
 */
export class FakeWriter implements Writer {
  mode: WriteMode;
  /** applied holds a copy of each Change that apply got, in order. */
  readonly applied: Change[] = [];
  /** previewed holds a copy of each Change that preview got, in order. */
  readonly previewed: Change[] = [];
  /** results are the results of the next applies, first one first. */
  results: WriteResult[] = [];
  /** previews are the results of the next previews, first one first. */
  previews: Preview[] = [];
  private commits = 0;

  constructor(mode: WriteMode = "direct") {
    this.mode = mode;
  }

  /** reset empties the records and the scripts and sets the mode, "direct" by default. */
  reset(mode: WriteMode = "direct"): void {
    this.mode = mode;
    this.applied.length = 0;
    this.previewed.length = 0;
    this.results = [];
    this.previews = [];
    this.commits = 0;
  }

  async preview(change: Change): Promise<Preview> {
    this.previewed.push(structuredClone(change));
    return this.previews.shift() ?? { objects: [], errors: [] };
  }

  async apply(change: Change): Promise<WriteResult> {
    this.applied.push(structuredClone(change));
    const scripted = this.results.shift();
    if (scripted !== undefined) return scripted;
    if (this.mode === "direct" || change.directOnly)
      return { ok: true, mode: "direct" };
    const revisions: Record<RefKey, string> = {};
    const paths: Record<RefKey, string> = {};
    const deleted: Record<RefKey, string> = {};
    for (const object of change.apply) {
      const key = refKey({ ...object.metadata, kind: object.kind });
      revisions[key] = createHash("sha1")
        .update(JSON.stringify(object))
        .digest("hex");
      paths[key] = fileOf({ ...object.metadata, kind: object.kind });
    }
    for (const ref of change.delete ?? []) deleted[refKey(ref)] = fileOf(ref);
    this.commits++;
    return {
      ok: true,
      mode: "git",
      commit: this.commits.toString(16).padStart(40, "0"),
      revisions,
      paths,
      deleted,
    };
  }
}

function fileOf(ref: {
  kind: string;
  namespace?: string;
  name?: string;
}): string {
  return `${ref.namespace ?? ""}/${ref.kind.toLowerCase()}/${ref.name}.yaml`;
}

/** fakeWriter is the writer that getWriter of this module returns. */
export const fakeWriter = new FakeWriter();

/** getWriter returns fakeWriter. */
export async function getWriter(): Promise<Writer> {
  return fakeWriter;
}

/** writeMode returns the mode of fakeWriter. */
export async function writeMode(): Promise<WriteMode> {
  return fakeWriter.mode;
}
