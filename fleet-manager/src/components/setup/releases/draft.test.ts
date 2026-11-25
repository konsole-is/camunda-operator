import { readFileSync } from "node:fs";
import { parse } from "yaml";
import { describe, expect, it } from "vitest";
import { LAST_APPLIED_ANNOTATION } from "@/lib/labels";
import type { CamundaRelease } from "@/generated/crds/camundarelease";
import type { ReleaseUsage } from "@/lib/usage";
import { foreignFields } from "@/lib/write/ownership";
import type { KubeObject } from "@/lib/write/types";
import {
  changeCount,
  copyDraft,
  createChange,
  deleteChange,
  draftErrors,
  draftOf,
  RELEASE_CHANGE_NOTICE,
  type ReleaseDraft,
  saveEffects,
  specOf,
  updateChange,
} from "./draft";
import { release } from "./fixtures";

const noUsers: ReleaseUsage = {
  clusters: [],
  elasticsearchClusters: [],
  databaseServers: [],
  claims: [],
};

const NAME_RULE =
  "The name has only lowercase letters, digits, and hyphens. It starts and ends with a letter or a digit, and has at most 253 characters.";

const errorsOf = (
  spec: Parameters<typeof specOf>[0]["spec"],
  name = "r",
  opts = { isNew: false, taken: [] as string[] },
) => draftErrors({ name, spec }, opts);

describe("draftErrors", () => {
  it("refuses a version that is not x.y.z", () => {
    expect(errorsOf({ version: "8.9" })).toEqual([
      {
        path: "spec.version",
        message: "The Camunda version is a version such as 8.9.0.",
      },
    ]);
    expect(errorsOf({ version: "" })).toHaveLength(1);
    expect(errorsOf({ version: "8.9.0" })).toEqual([]);
  });

  it("refuses each optional version that has the wrong form", () => {
    expect(
      errorsOf({
        version: "8.9.0",
        connectors: { version: "8.9" },
        elasticsearch: { version: "v9" },
        databaseServer: { version: "17.1" },
      }),
    ).toEqual([
      {
        path: "spec.connectors.version",
        message: "The Connectors version is a version such as 8.9.7.",
      },
      {
        path: "spec.elasticsearch.version",
        message: "The Elasticsearch version is a version such as 9.2.4.",
      },
      {
        path: "spec.databaseServer.version",
        message: "The PostgreSQL version is a major version such as 17.",
      },
    ]);
  });

  it("accepts optional versions that are unset or empty", () => {
    expect(
      errorsOf({
        version: "8.9.0",
        connectors: { version: "" },
        databaseServer: {},
      }),
    ).toEqual([]);
  });

  it("checks the name of a new release", () => {
    const opts = { isNew: true, taken: ["taken"] };
    expect(errorsOf({ version: "8.9.0" }, "", opts)).toEqual([
      { path: "name", message: "Give the release a name." },
    ]);
    expect(errorsOf({ version: "8.9.0" }, "camunda.8-9", opts)).toEqual([
      { path: "name", message: NAME_RULE },
    ]);
  });

  // A claim names its release in spec.releaseRef, so a name that the field
  // refuses gives a release that no claim can use.
  it("accepts a new name only when spec.releaseRef of a claim accepts it", () => {
    const crd = parse(
      readFileSync(
        new URL(
          "../../../../../config/crd/bases/fleet.camunda.io_camundaclusterclaims.yaml",
          import.meta.url,
        ),
        "utf8",
      ),
    );
    const field =
      crd.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties
        .releaseRef;
    const referable = (name: string) =>
      new RegExp(field.pattern).test(name) && name.length <= field.maxLength;
    const opts = { isNew: true, taken: [] };
    for (const name of [
      "camunda-8-9-2",
      "7",
      "camunda.8-9-2",
      "a.b",
      "Camunda",
      "r".repeat(253),
      "r".repeat(254),
    ])
      expect(errorsOf({ version: "8.9.0" }, name, opts), name).toEqual(
        referable(name) ? [] : [{ path: "name", message: NAME_RULE }],
      );
  });

  it("refuses a taken name for a new release only", () => {
    expect(
      errorsOf({ version: "8.9.0" }, "taken", {
        isNew: true,
        taken: ["taken"],
      }),
    ).toEqual([{ path: "name", message: "A release with this name exists." }]);
    expect(
      errorsOf({ version: "8.9.0" }, "taken", {
        isNew: false,
        taken: ["taken"],
      }),
    ).toEqual([]);
  });
});

describe("specOf", () => {
  it("drops empty fields and blocks from the spec", () => {
    expect(
      specOf({
        name: "r",
        spec: {
          version: "8.9.0",
          connectors: { version: "" },
          elasticsearch: {},
          databaseServer: { version: "17" },
          images: { camunda: "", connectors: "reg/c:1" },
          extraEnv: [],
          zeebe: { extraEnv: [], extraEnvFrom: [] },
          operate: { extraEnv: [{ name: "A", value: "1" }] },
        },
      }),
    ).toEqual({
      version: "8.9.0",
      databaseServer: { version: "17" },
      images: { connectors: "reg/c:1" },
      operate: { extraEnv: [{ name: "A", value: "1" }] },
    });
  });
});

describe("draftOf and copyDraft", () => {
  it("copies the spec of the source under the new name", () => {
    const source = release("camunda-8-9-0", {
      version: "8.9.0",
      extraEnv: [{ name: "A", value: "1" }],
    });
    const draft = copyDraft(source, "camunda-8-9-2");
    expect(draft).toEqual({
      name: "camunda-8-9-2",
      spec: { version: "8.9.0", extraEnv: [{ name: "A", value: "1" }] },
    });
    draft.spec.extraEnv?.push({ name: "B", value: "2" });
    expect(source.spec.extraEnv).toHaveLength(1);
  });

  it("starts an empty draft without a source", () => {
    expect(copyDraft(undefined, "x")).toEqual({
      name: "x",
      spec: { version: "" },
    });
    expect(draftOf(undefined)).toEqual({ name: "", spec: { version: "" } });
  });
});

describe("changeCount", () => {
  it("counts the changed top-level fields and the name of a new release", () => {
    const live = release("r", { version: "8.9.0" });
    expect(changeCount(live, draftOf(live), false)).toBe(0);
    expect(
      changeCount(
        live,
        { name: "r", spec: { version: "8.9.1", extraEnv: [{ name: "A" }] } },
        false,
      ),
    ).toBe(2);
    expect(
      changeCount(undefined, { name: "n", spec: { version: "" } }, true),
    ).toBe(1);
  });
});

describe("change builders", () => {
  it("updates with the resourceVersion and no force, the labels, and the annotations without the last applied file", () => {
    const live = release("r", { version: "8.9.0" });
    live.metadata.annotations = {
      [LAST_APPLIED_ANNOTATION]: "{}",
      note: "kept",
    };
    expect(
      updateChange(live, {
        name: "r",
        spec: { version: "8.9.1", extraEnv: [] },
      }),
    ).toEqual({
      intent: "Change the release r",
      apply: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaRelease",
          metadata: {
            name: "r",
            labels: { "app.kubernetes.io/name": "camunda-operator" },
            annotations: { note: "kept" },
          },
          spec: { version: "8.9.1" },
        },
      ],
      preconditions: { "CamundaRelease//r": "41" },
    });
  });

  describe("a save over the empty values of another tool", () => {
    // live holds fields that kubectl set: the version, an empty map, an
    // empty list, and a block of only empty values.
    const live = release("r", {
      version: "8.9.0",
      images: {},
      extraEnv: [],
      connectors: { version: "" },
    });
    live.metadata.managedFields = [
      {
        manager: "kubectl-client-side-apply",
        operation: "Update",
        fieldsV1: {
          "f:spec": {
            "f:version": {},
            "f:images": {},
            "f:extraEnv": {},
            "f:connectors": { "f:version": {} },
          },
        },
      },
    ];
    const rows = (object: CamundaRelease, draft: ReleaseDraft) => {
      const change = updateChange(object, draft);
      const { changed, cleared } = foreignFields(
        object as unknown as KubeObject,
        change.apply[0],
        change.keep?.["CamundaRelease//r"],
      );
      return {
        changed: changed.map((f) => f.path),
        cleared: cleared.map((f) => f.path),
      };
    };

    it("shows only the changed field", () => {
      const draft = draftOf(live);
      draft.spec.version = "8.9.1";
      expect(rows(live, draft)).toEqual({
        changed: ["spec.version"],
        cleared: [],
      });
    });

    it("still shows a field that the edit cleared", () => {
      const set = release("r", {
        ...live.spec,
        connectors: { version: "8.9.7" },
      });
      set.metadata.managedFields = live.metadata.managedFields;
      const draft = draftOf(set);
      draft.spec.connectors = { version: "" };
      expect(rows(set, draft).cleared).toEqual(["spec.connectors"]);
    });
  });

  it("creates a release by name in create", () => {
    expect(
      createChange({ name: "n", spec: { version: "8.9.2", extraEnv: [] } }),
    ).toEqual({
      intent: "Create the release n",
      apply: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaRelease",
          metadata: { name: "n" },
          spec: { version: "8.9.2" },
        },
      ],
      create: ["CamundaRelease//n"],
    });
  });

  it("deletes each release with its precondition and its UID", () => {
    const one = release("a", { version: "8.9.0" });
    const two = release("b", { version: "8.9.1" });
    expect(deleteChange([one])).toMatchObject({
      intent: "Delete the release a",
    });
    expect(deleteChange([one, two])).toEqual({
      intent: "Delete the releases a, b",
      apply: [],
      delete: [
        { apiVersion: "core.camunda.io/v1", kind: "CamundaRelease", name: "a" },
        { apiVersion: "core.camunda.io/v1", kind: "CamundaRelease", name: "b" },
      ],
      preconditions: { "CamundaRelease//a": "41", "CamundaRelease//b": "41" },
      uids: { "CamundaRelease//a": "uid-a", "CamundaRelease//b": "uid-b" },
    });
  });
});

describe("saveEffects", () => {
  const live = release("r", { version: "8.9.0" });

  it("says the pods roll and names the version change when users exist", () => {
    expect(
      saveEffects(
        live,
        { name: "r", spec: { version: "8.9.1" } },
        { ...noUsers, clusters: ["a/one", "b/two"] },
      ),
    ).toEqual([
      "2 clusters use this release. Their pods roll.",
      "The Camunda version changes from 8.9.0 to 8.9.1 for each cluster that does not set its own.",
    ]);
  });

  it("names each version change for the kind of user that runs it", () => {
    const base = release("r", {
      version: "8.9.0",
      connectors: { version: "8.9.7" },
      elasticsearch: { version: "9.2.4" },
      databaseServer: { version: "16" },
    });
    expect(
      saveEffects(
        base,
        {
          name: "r",
          spec: {
            version: "8.9.0",
            elasticsearch: { version: "9.2.8" },
            databaseServer: { version: "17" },
          },
        },
        {
          ...noUsers,
          clusters: ["a/one"],
          elasticsearchClusters: ["a/search"],
          databaseServers: ["a/db"],
        },
      ),
    ).toEqual([
      "1 cluster, 1 Elasticsearch cluster, 1 database server use this release. Their pods roll.",
      "The Connectors version changes from 8.9.7 to none for each cluster that does not set its own.",
      "The Elasticsearch version changes from 9.2.4 to 9.2.8 for each Elasticsearch cluster that does not set its own.",
      "The PostgreSQL version changes from 16 to 17 for each database server that does not set its own.",
    ]);
  });

  it("names no version change for a kind of user that the release does not have", () => {
    expect(
      saveEffects(
        live,
        { name: "r", spec: { version: "8.9.1" } },
        { ...noUsers, elasticsearchClusters: ["a/search"] },
      ),
    ).toEqual(["1 Elasticsearch cluster uses this release. Their pods roll."]);
  });

  it("says that a claim takes the change when its cluster is made", () => {
    expect(
      saveEffects(
        live,
        { name: "r", spec: { version: "8.9.0", extraEnv: [{ name: "A" }] } },
        { ...noUsers, claims: ["a/one"] },
      ),
    ).toEqual([
      "1 claim names this release. Their clusters take the change when they are made.",
    ]);
  });

  it("says that no resource uses an unused release", () => {
    expect(
      saveEffects(live, { name: "r", spec: { version: "8.9.1" } }, noUsers),
    ).toEqual(["No resource uses this release."]);
  });

  it("says that a new release has no user", () => {
    expect(
      saveEffects(
        undefined,
        { name: "n", spec: { version: "8.9.1" } },
        noUsers,
      ),
    ).toEqual(["The release is new. No resource uses it yet."]);
  });
});

describe("RELEASE_CHANGE_NOTICE", () => {
  it("says that a change reaches every user and that their pods roll", () => {
    expect(RELEASE_CHANGE_NOTICE).toBe(
      "A change reaches every resource that uses this release. Their pods roll. A version change moves each user that does not set its own version. The Rollout tab moves clusters one by one.",
    );
  });
});
