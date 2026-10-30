import type { ConditionLike } from "@/components/fleet/status";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import {
  CAMUNDA_CLUSTER_PLACEMENT,
  CAMUNDA_OPTIMIZE_PLACEMENT,
  conditionsAt,
} from "@/lib/conditions";
import type { Resource } from "@/lib/k8s/kinds";
import {
  COMPONENT_LABEL,
  OPTIMIZE_COMPONENTS,
  OPTIMIZE_UID_LABEL,
} from "@/lib/labels";
import type {
  PartitionHealth,
  PodUsage,
  Unavailable,
} from "@/lib/observe/types";
import { PROCESS_LABEL, type Process, podDot, topologyOf } from "./topology";

/** CardId names one component card. */
export type CardId = Process | "optimize";

/** CardNumber is one number of a card, as the card shows it. */
export type CardNumber = { label: string; value: string };

type Condition = ConditionLike & { type: string };

/**
 * ComponentCardModel is one card. pods are the names of the pods that the
 * card counts. A card that is not used has no condition and no numbers.
 */
export type ComponentCardModel = {
  id: CardId;
  label: string;
  used: boolean;
  condition?: Condition;
  embedded: { process: Process; label: string }[];
  pods: string[];
  numbers: CardNumber[];
};

/**
 * CardInput is what the cards are made from. spec is the merged spec of the
 * cluster. optimize is the CamundaOptimize of the cluster, if any. An
 * undefined usage or partitions is not loaded yet.
 */
export type CardInput = {
  spec: CamundaClusterSpec;
  pods: readonly Resource<"Pod">[];
  conditions?: readonly Condition[];
  optimize?: Resource<"CamundaOptimize">;
  usage?: PodUsage[] | Unavailable;
  partitions?: PartitionHealth[] | Unavailable;
};

const OPTIMIZE_LABEL = "Optimize";

/**
 * componentCards returns the card of each Camunda process of a cluster: the
 * boxes of topologyOf, then Connectors when they are not enabled, then
 * Optimize. An embedded process is no card of its own, but an entry in
 * embedded of its host. Disabled Connectors and a cluster without optimize
 * give a card that is not used. Brokers show the pods ready and the
 * partitions with a leader. Every other used card shows the pods ready and
 * the highest use.
 */
export function componentCards(input: CardInput): ComponentCardModel[] {
  const sorted = [...input.pods].sort((a, b) =>
    (a.metadata?.name ?? "").localeCompare(b.metadata?.name ?? ""),
  );
  const podsWhere = (components: readonly string[]) =>
    sorted.filter((p) =>
      components.includes(p.metadata?.labels?.[COMPONENT_LABEL] ?? ""),
    );
  const numbersOf = (id: CardId, pods: readonly Resource<"Pod">[]) => [
    podsReady(pods),
    id === "zeebe"
      ? partitionsLed(input.partitions)
      : highestUse(input.usage, names(pods)),
  ];

  const cards: ComponentCardModel[] = topologyOf(input.spec, input.pods).map(
    (box) => {
      const pods = podsWhere([box.process]);
      return {
        id: box.process,
        label: box.label,
        used: true,
        condition: conditionsAt(
          CAMUNDA_CLUSTER_PLACEMENT,
          input.conditions,
          box.process,
        )[0],
        embedded: box.embedded,
        pods: names(pods),
        numbers: numbersOf(box.process, pods),
      };
    },
  );

  if (!cards.some((c) => c.id === "connectors")) {
    cards.push(notUsed("connectors", PROCESS_LABEL.connectors));
  }

  if (!input.optimize) {
    cards.push(notUsed("optimize", OPTIMIZE_LABEL));
  } else {
    // A deleted Optimize of the cluster can leave pods that still stop.
    const uid = input.optimize.metadata?.uid;
    const pods = podsWhere(OPTIMIZE_COMPONENTS).filter(
      (p) => p.metadata?.labels?.[OPTIMIZE_UID_LABEL] === uid,
    );
    cards.push({
      id: "optimize",
      label: OPTIMIZE_LABEL,
      used: true,
      condition: conditionsAt(
        CAMUNDA_OPTIMIZE_PLACEMENT,
        input.optimize.status?.conditions as Condition[] | undefined,
        "optimize",
      ).find((c) => c.type === "Ready"),
      embedded: [],
      pods: names(pods),
      numbers: numbersOf("optimize", pods),
    });
  }
  return cards;
}

/** podsReady counts the pods whose podDot is ok, as "<ready>/<total>". */
export function podsReady(pods: readonly Resource<"Pod">[]): CardNumber {
  const ready = pods.filter((p) => podDot(p).tone === "ok").length;
  return { label: "Pods ready", value: `${ready}/${pods.length}` };
}

/**
 * highestUse gives the highest CPU or memory use of one of pods against its
 * limit, such as "82 % memory". It says "No limit" when usage has a sample of
 * some of pods and none of those samples has a limit. When usage is a list
 * that has no sample of any of pods, it says "Not available".
 */
export function highestUse(
  usage: PodUsage[] | Unavailable | undefined,
  pods: readonly string[],
): CardNumber {
  const label = "Highest use";
  if (usage === undefined) return { label, value: "Loading" };
  if (usage === "unavailable") return { label, value: "Not available" };

  const sampled = usage.filter((u) => pods.includes(u.pod));
  if (sampled.length === 0) return { label, value: "Not available" };
  let highest: { share: number; resource: "cpu" | "memory" } | undefined;
  for (const u of sampled) {
    for (const [resource, used, limit] of [
      ["cpu", u.cpu.usedCores, u.cpu.limitCores],
      ["memory", u.memory.usedBytes, u.memory.limitBytes],
    ] as const) {
      if (!limit) continue;
      const share = used / limit;
      if (!highest || share > highest.share) highest = { share, resource };
    }
  }
  if (!highest) return { label, value: "No limit" };
  return {
    label,
    value: `${Math.round(highest.share * 100)} % ${highest.resource}`,
  };
}

/**
 * partitionsLed counts the partitions that have a leader, as
 * "<led>/<total>".
 */
export function partitionsLed(
  partitions: PartitionHealth[] | Unavailable | undefined,
): CardNumber {
  const label = "Partitions with a leader";
  if (partitions === undefined) return { label, value: "Loading" };
  if (partitions === "unavailable") return { label, value: "Not available" };
  const led = partitions.filter((p) => p.leader).length;
  return { label, value: `${led}/${partitions.length}` };
}

function notUsed(id: CardId, label: string): ComponentCardModel {
  return { id, label, used: false, embedded: [], pods: [], numbers: [] };
}

function names(pods: readonly Resource<"Pod">[]): string[] {
  return pods.map((p) => p.metadata?.name ?? "");
}
