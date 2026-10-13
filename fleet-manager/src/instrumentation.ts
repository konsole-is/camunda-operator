// Next.js calls register once when the server starts. The server stops when
// a bundled file is missing or broken, so a bad image never serves a page.
export async function register(): Promise<void> {
  // Next.js also compiles this file for the Edge runtime, which has no
  // process.exit. It drops this block there.
  if (process.env.NEXT_RUNTIME === "nodejs") {
    const { describeBundledData, loadBundledData } =
      await import("@/startup/bundled-data");
    try {
      console.log(describeBundledData(await loadBundledData()));
    } catch (error) {
      console.error(
        `The fleet manager could not start: ${error instanceof Error ? error.message : String(error)}`,
      );
      process.exit(1);
    }
  }
}
