// The loader of the page scenarios. Playwright loads it too, so it has no
// import.meta and it never imports src/lib/k8s/client.ts.
import fs from "node:fs/promises";
import path from "node:path";
import type { KubeConfig } from "@kubernetes/client-node";
import { parse } from "yaml";
import { Discovery, KubeError, type KubeMethod, kubeRequest } from "./kube";
import {
  type KubeObject,
  alignObservedGeneration,
  createOrder,
  describeObject,
  diffStates,
  readObjects,
  replaceUIDs,
} from "./objects";

/** LOADER_MANAGER is the field manager of each write of the loader. */
export const LOADER_MANAGER = "scenario-loader";

const WRITE_QUERY = `fieldManager=${LOADER_MANAGER}&fieldValidation=Strict`;

// The API server sets these fields, so a create or an update leaves them out.
const SERVER_FIELDS = [
  "uid",
  "resourceVersion",
  "creationTimestamp",
  "generation",
  "managedFields",
  "deletionTimestamp",
  "deletionGracePeriodSeconds",
];

/**
 * ScenarioLoader writes the objects of a scenario into an API server, as an
 * admin, so that each live object reads as the generator recorded it: the
 * same fields, status, and field managers, with the real UIDs in place of
 * the scenario UIDs. No live object keeps an entry of LOADER_MANAGER.
 */
export class ScenarioLoader {
  private readonly discovery: Discovery;
  private readonly uids = new Map<string, string>();
  private current: readonly KubeObject[] = [];

  constructor(private readonly kc: KubeConfig) {
    this.discovery = new Discovery(kc);
  }

  /**
   * load creates objects. It rejects on the first write that the API server
   * refuses, such as a field that a CRD does not have, and names the object.
   */
  async load(objects: readonly KubeObject[]): Promise<void> {
    await this.apply(createOrder(objects), []);
    this.current = objects;
  }

  /**
   * advance moves the live objects from the last snapshot that load or
   * advance wrote to next: it creates, updates, and deletes. A deleted
   * object is gone, also when it had finalizers or is a Pod on a node.
   */
  async advance(next: readonly KubeObject[]): Promise<void> {
    const { create, update, remove } = diffStates(this.current, next);
    await this.apply(create, update);
    for (const o of remove) await this.remove(o);
    this.current = next;
  }

  /** realUID returns the UID of the live object of a scenario UID. */
  realUID(scenarioUID: string): string | undefined {
    return this.uids.get(scenarioUID);
  }

  // Each object is created first, so that every real UID is known before
  // any object is written with the full map: an annotation can name the UID
  // of an object that is created after it.
  private async apply(
    create: readonly KubeObject[],
    update: readonly KubeObject[],
  ): Promise<void> {
    const posted = new Map<KubeObject, KubeObject>();
    for (const o of create) posted.set(o, await this.create(o));
    for (const o of [...create, ...update]) {
      await this.settle(o, posted.get(o));
    }
  }

  private async create(o: KubeObject): Promise<KubeObject> {
    const body = await this.desired(o);
    const live = await this.send(o, "create", "POST", {
      collection: true,
      body,
    });
    if (o.metadata.uid && live.metadata.uid) {
      this.uids.set(o.metadata.uid, live.metadata.uid);
    }
    return body;
  }

  // settle writes o again with every real UID, then its status, then its
  // recorded managedFields. posted is the body of the create of o, if this
  // apply created it.
  private async settle(o: KubeObject, posted?: KubeObject): Promise<void> {
    const recorded = replaceUIDs(o, this.uids);
    const hasStatus = await this.discovery.hasStatus(o.apiVersion, o.kind);
    let live = await this.send(o, "read", "GET");

    const body = await this.desired(o);
    // The API server adds fields on create, such as the default tolerations
    // of a Pod, and can refuse a PUT that drops them. So the PUT runs only
    // when a UID that was not known at the create changed the body.
    if (!posted || JSON.stringify(body) !== JSON.stringify(posted)) {
      body.metadata.resourceVersion = live.metadata.resourceVersion;
      live = await this.send(o, "update", "PUT", { body });
    }

    if (hasStatus && recorded.status !== undefined) {
      live.status = alignObservedGeneration(
        recorded.status,
        o.metadata.generation ?? 1,
        live.metadata.generation ?? 1,
      );
      live = await this.send(o, "update the status of", "PUT", {
        subresource: "status",
        body: live,
      });
    }

    // A write that changes no field and sets managedFields stores the list
    // as given, so the writes above leave no entry of the loader. An empty
    // list leaves the entries as they are, and a list of one empty entry
    // removes them all.
    const fields = recorded.metadata.managedFields;
    live.metadata.managedFields = fields?.length ? fields : [{}];
    await this.send(o, "set the field managers of", "PUT", { body: live });

    // The finalizers of the recorded object keep it terminating.
    if (o.metadata.deletionTimestamp) {
      await this.send(o, "delete", "DELETE");
    }
  }

  // Envtest runs no garbage collector and no kubelet, so remove drops the
  // finalizers and deletes without a grace period.
  private async remove(o: KubeObject): Promise<void> {
    try {
      if (o.metadata.finalizers?.length) {
        await this.send(o, "remove the finalizers of", "PATCH", {
          body: { metadata: { finalizers: null } },
          contentType: "application/merge-patch+json",
        });
      }
      await this.send(o, "delete", "DELETE", { query: "gracePeriodSeconds=0" });
    } catch (error) {
      if (error instanceof KubeError && error.status === 404) return;
      throw error;
    }
  }

  // desired is o with the known real UIDs, without the fields that the API
  // server sets, and without the status when the status is a subresource.
  private async desired(o: KubeObject): Promise<KubeObject> {
    const body = replaceUIDs(o, this.uids);
    for (const field of SERVER_FIELDS) delete body.metadata[field];
    if (await this.discovery.hasStatus(o.apiVersion, o.kind)) {
      delete body.status;
    }
    return body;
  }

  private async send(
    o: KubeObject,
    action: string,
    method: KubeMethod,
    opts: {
      collection?: boolean;
      subresource?: "status";
      body?: unknown;
      contentType?: string;
      query?: string;
    } = {},
  ): Promise<KubeObject> {
    const query =
      opts.query ??
      (method === "GET" || method === "DELETE" ? "" : WRITE_QUERY);
    const target = await this.discovery.path(o, opts);
    try {
      return await kubeRequest<KubeObject>(
        this.kc,
        method,
        query ? `${target}?${query}` : target,
        opts.body,
        opts.contentType,
      );
    } catch (error) {
      if (error instanceof KubeError) {
        throw new KubeError(
          error.status,
          `Could not ${action} ${describeObject(o)}: "${error.message}"`,
        );
      }
      throw error;
    }
  }
}

/** Scenario is a scenario that openScenario loaded. */
export interface Scenario {
  name: string;
  dir: string;
  /**
   * advance moves the live objects to the snapshot <dir>/<state>.yaml. It
   * rejects a state that scenario.yaml does not list, and names the states.
   */
  advance(state: string): Promise<void>;
}

// The generator writes these files beside the states, and no state has
// their names.
const RESERVED = ["objects", "scenario", "observe"];

/**
 * openScenario loads <dir>/objects.yaml into the API server of kc, an admin
 * kubeconfig. The name of the scenario is the name of dir.
 */
export async function openScenario(
  kc: KubeConfig,
  dir: string,
): Promise<Scenario> {
  const name = path.basename(dir);
  const file = parse(
    await fs.readFile(path.join(dir, "scenario.yaml"), "utf8"),
  ) as { states?: { name: string }[] } | null;
  const states = (file?.states ?? []).map((s) => s.name);
  const loader = new ScenarioLoader(kc);
  await loader.load(await readObjects(path.join(dir, "objects.yaml")));

  return {
    name,
    dir,
    async advance(state: string) {
      if (RESERVED.includes(state) || !states.includes(state)) {
        const known =
          states.length > 0
            ? `Its states are: ${states.join(", ")}`
            : "It has no states";
        throw new Error(`The scenario ${name} has no state ${state}. ${known}`);
      }
      await loader.advance(await readObjects(path.join(dir, `${state}.yaml`)));
    },
  };
}
