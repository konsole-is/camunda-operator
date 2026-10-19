import "server-only";

const TAILS_KEY: unique symbol = Symbol.for("fleet-manager.serial");

// Next.js can load this module once for each route, so the order lives on
// globalThis and every route shares it.
const tails = ((
  globalThis as typeof globalThis & {
    [TAILS_KEY]?: Map<string, Promise<unknown>>;
  }
)[TAILS_KEY] ??= new Map());

/**
 * inOrder runs fn when every earlier call of inOrder with the same key in
 * this server process has settled, and returns its result. A call that
 * failed does not stop the calls after it.
 */
export function inOrder<T>(key: string, fn: () => Promise<T>): Promise<T> {
  const run = (tails.get(key) ?? Promise.resolve()).then(() => fn());
  const tail = run.catch(() => {});
  tails.set(key, tail);
  void tail.then(() => {
    if (tails.get(key) === tail) tails.delete(key);
  });
  return run;
}
