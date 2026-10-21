import Link from "next/link";
import { WatchProbe } from "@/gallery/watch-probe";

// The watch check of make fleet-gallery opens this page against envtest. It
// mounts 8 data hooks, so over HTTP/1.1 a connection per hook would leave
// the browser no connection for the link and the action.
export default function WatchProbePage() {
  return (
    <main className="mx-auto max-w-[1280px] px-4 py-6 md:px-8">
      <h1 className="mb-6 font-heading text-3xl font-bold">Watch probe</h1>
      <WatchProbe />
      <Link href="/dev/watch-probe/next" className="text-info">
        Next page
      </Link>
    </main>
  );
}
