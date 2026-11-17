import type { ConditionLike } from "@/components/fleet/status";
import type { CoreCamundaOptimizeConditionType } from "@/generated/conditions";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type {
  SectionComponent,
  SectionComponentId,
} from "@/lib/claims/sections";
import {
  CAMUNDA_CLUSTER_PLACEMENT,
  CAMUNDA_OPTIMIZE_PLACEMENT,
  conditionsAt,
} from "@/lib/conditions";
import type { Resource } from "@/lib/k8s/kinds";
import { isObject } from "@/lib/policy";
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
import {
  PROCESS_LABEL,
  type PodStateLike,
  type Process,
  podDot,
  topologyOf,
} from "./topology";

/** ProcessCardId names the card of a Camunda process or of Optimize. */
export type ProcessCardId = Process | "optimize";

/**
 * CardId names one component card: a process card, or the card of a
 * selection of the claim.
 */
export type CardId = ProcessCardId | SectionComponentId;

/**
 * CardNumber is one number of a card, as the card shows it. severity marks
 * a value that the card shows as a warning or an error.
 */
export type CardNumber = {
  label: string;
  value: string;
  severity?: "Warning" | "Error";
};

type Condition = ConditionLike & { type: string };

/**
 * ComponentCardModel is one card. pods are the names of the pods that the
 * card counts, and a section card counts none. embedded are the apps that
 * run in the card, each with its own id. A card that is not used has no
 * condition and no numbers. unknown says that the page could not read the
 * condition of the card, so its state is not known.
 */
export type ComponentCardModel<Id extends string = CardId> = {
  id: Id;
  label: string;
  used: boolean;
  condition?: Condition;
  unknown?: true;
  embedded: { process: Id; label: string }[];
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
const OPTIMIZE_READY: CoreCamundaOptimizeConditionType = "Ready";

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
  const numbersOf = (id: ProcessCardId, pods: readonly Resource<"Pod">[]) => [
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
        condition: componentReady(
          box.process,
          conditionsAt(
            CAMUNDA_CLUSTER_PLACEMENT,
            input.conditions,
            box.process,
          ),
        ),
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
      condition: componentReady(
        "optimize",
        conditionsAt(
          CAMUNDA_OPTIMIZE_PLACEMENT,
          input.optimize.status?.conditions as Condition[] | undefined,
          "optimize",
        ),
      ),
      embedded: [],
      pods: names(pods),
      numbers: numbersOf("optimize", pods),
    });
  }
  return cards;
}

/**
 * sectionCards returns one used card for each section component, in the
 * order given. The headline is the first summary item of the request, with
 * its Warning or Error severity, then the count of its resources. A section
 * whose request does not exist yet says so. The Ready condition of the
 * request is the state of the card. With requestsError, the error of the
 * read of the PluginRequests, or with requestsStale, when their list is out
 * of date, no card shows a request or a state: each says that the request
 * could not be read, or is not known. A summary or resources of the wrong
 * type, which a watch can give, show "Not valid".
 */
export function sectionCards(
  components: readonly SectionComponent[],
  requestsError?: string,
  requestsStale = false,
): ComponentCardModel[] {
  const unread =
    requestsError !== undefined
      ? "Could not read"
      : requestsStale
        ? "Not known"
        : undefined;
  return components.map((component) => ({
    id: component.id,
    label: component.label,
    used: true,
    ...(component.condition && !unread
      ? { condition: { type: "Ready", ...component.condition } }
      : {}),
    embedded: [],
    pods: [],
    numbers: sectionNumbers(component.request, unread),
  }));
}

function sectionNumbers(
  request: Resource<"PluginRequest"> | undefined,
  unread: string | undefined,
): CardNumber[] {
  if (unread) {
    return [{ label: "Request", value: unread, severity: "Warning" }];
  }
  if (!request) return [{ label: "Request", value: "Not made yet" }];
  const summary: unknown = request.status?.summary;
  const resources: unknown = request.status?.resources ?? [];
  const first: unknown = Array.isArray(summary) ? summary[0] : undefined;
  const severity = isObject(first) ? first.severity : undefined;
  const headline: CardNumber =
    summary === undefined || (Array.isArray(summary) && first === undefined)
      ? { label: "Summary", value: "Not written yet" }
      : isObject(first) &&
          typeof first.label === "string" &&
          typeof first.value === "string"
        ? {
            label: first.label,
            value: first.value,
            ...(severity === "Warning" || severity === "Error"
              ? { severity }
              : {}),
          }
        : notValid("Summary");
  return [
    headline,
    Array.isArray(resources)
      ? { label: "Resources", value: String(resources.length) }
      : notValid("Resources"),
  ];
}

function notValid(label: string): CardNumber {
  return { label, value: "Not valid", severity: "Warning" };
}

/**
 * componentReady returns the condition that decides the state of component
 * id, from the conditions that the maps place at it: Ready for Optimize, and
 * the first one for every other component, which has one. It returns
 * undefined when there is none.
 */
export function componentReady<C extends { type: string }>(
  id: ProcessCardId,
  conditions: readonly C[],
): C | undefined {
  return id === "optimize"
    ? conditions.find((c) => c.type === OPTIMIZE_READY)
    : conditions[0];
}

/** podsReady counts the pods whose podDot is ok, as "<ready>/<total>". */
export function podsReady(pods: readonly PodStateLike[]): CardNumber {
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
  const highest = highestUseOf(usage, pods);
  return {
    label: "Highest use",
    value:
      typeof highest === "string"
        ? highest
        : `${Math.round(highest.share * 100)} % ${highest.resource}`,
  };
}

/** HighestUse is the use of one resource by one pod, as a share of its limit. */
export type HighestUse = {
  pod: string;
  resource: "cpu" | "memory";
  share: number;
};

/**
 * highestUseOf returns the highest use of one of resources by one of pods
 * against its limit. In place of a use it returns the words of highestUse:
 * "Loading" for an undefined usage, "Not available", or "No limit".
 */
export function highestUseOf(
  usage: PodUsage[] | Unavailable | undefined,
  pods: readonly string[],
  resources: readonly ("cpu" | "memory")[] = ["cpu", "memory"],
): HighestUse | "Loading" | "Not available" | "No limit" {
  if (usage === undefined) return "Loading";
  if (usage === "unavailable") return "Not available";

  const sampled = usage.filter((u) => pods.includes(u.pod));
  if (sampled.length === 0) return "Not available";
  let highest: HighestUse | undefined;
  for (const u of sampled) {
    for (const [resource, used, limit] of [
      ["cpu", u.cpu.usedCores, u.cpu.limitCores],
      ["memory", u.memory.usedBytes, u.memory.limitBytes],
    ] as const) {
      if (!limit || !resources.includes(resource)) continue;
      const share = used / limit;
      if (!highest || share > highest.share) {
        highest = { pod: u.pod, resource, share };
      }
    }
  }
  return highest ?? "No limit";
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

function notUsed(id: ProcessCardId, label: string): ComponentCardModel {
  return { id, label, used: false, embedded: [], pods: [], numbers: [] };
}

function names(pods: readonly Resource<"Pod">[]): string[] {
  return pods.map((p) => p.metadata?.name ?? "");
}

/**
 * componentOf returns the component that value names when it is a used card
 * or an app embedded in a used card, else undefined.
 */
export function componentOf(
  value: string | null,
  cards: readonly ComponentCardModel[],
): CardId | undefined {
  for (const card of cards) {
    if (!card.used) continue;
    if (card.id === value) return card.id;
    const app = card.embedded.find((e) => e.process === value);
    if (app) return app.process;
  }
  return undefined;
}

/** isSectionId reports whether id names the card of a selection of the claim. */
export function isSectionId(id: string): id is SectionComponentId {
  return id.startsWith("section:");
}

/**
 * overviewHref is the URL of the Overview tab with the detail of id open,
 * or with the cards when id is null. componentOf reads the id back from
 * its component parameter.
 */
export function overviewHref(id: CardId | null): string {
  return id === null
    ? "?tab=overview"
    : `?${new URLSearchParams({ tab: "overview", component: id })}`;
}
