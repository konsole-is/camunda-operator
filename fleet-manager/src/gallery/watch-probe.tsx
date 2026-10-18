"use client";
import { useState } from "react";
import { ping } from "@/app/dev/watch-probe/actions";
import { useResource, useResources } from "@/lib/k8s/hooks";

/**
 * WatchProbe mounts one data hook for each of 8 kinds that envtest serves,
 * shows the count of items of each, and calls a server action on Ping.
 */
export function WatchProbe() {
  const counts: [string, number][] = [
    ["Pod", useResources("Pod", {}, []).items.length],
    ["Service", useResources("Service", {}, []).items.length],
    [
      "PersistentVolumeClaim",
      useResources("PersistentVolumeClaim", {}, []).items.length,
    ],
    ["Namespace", useResources("Namespace", {}, []).items.length],
    ["Secret", useResources("Secret", {}, []).items.length],
    ["Event", useResources("Event", {}, []).items.length],
    ["Environment", useResources("Environment", {}, []).items.length],
    [
      "CamundaCluster",
      useResource("CamundaCluster", { namespace: "default", name: "probe" })
        .item
        ? 1
        : 0,
    ],
  ];
  const [answer, setAnswer] = useState("");
  return (
    <div className="mb-6 flex flex-col gap-2">
      <ul>
        {counts.map(([kind, count]) => (
          <li key={kind} data-kind={kind}>
            {kind}: {count}
          </li>
        ))}
      </ul>
      <button
        type="button"
        className="w-fit rounded border px-3 py-1"
        onClick={async () => setAnswer(await ping())}
      >
        Ping
      </button>
      <output data-testid="ping">{answer}</output>
    </div>
  );
}
