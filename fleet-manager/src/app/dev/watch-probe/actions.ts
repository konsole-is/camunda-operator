"use server";

/** ping answers "pong", so the watch check can wait for a server action. */
export async function ping(): Promise<string> {
  return "pong";
}
