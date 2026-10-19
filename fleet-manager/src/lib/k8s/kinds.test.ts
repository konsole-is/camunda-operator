import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { parse, parseAllDocuments } from "yaml";
import { describe, expect, expectTypeOf, it } from "vitest";
import {
  KindMap,
  collectionPath,
  objectPath,
  type KindName,
  type Resource,
} from "./kinds";

const appDir = fileURLToPath(new URL("../../..", import.meta.url));
const crdDir = path.join(appDir, "..", "config", "crd", "bases");

interface CRD {
  spec: {
    group: string;
    scope: string;
    names: { kind: string; plural: string };
    versions: { name: string; served: boolean; storage: boolean }[];
  };
}

const crds: CRD[] = fs
  .readdirSync(crdDir)
  .filter((f) => f.endsWith(".yaml"))
  .flatMap((f) =>
    parseAllDocuments(fs.readFileSync(path.join(crdDir, f), "utf8")),
  )
  .map((doc) => doc.toJS() as CRD);

interface Rule {
  apiGroups: string[];
  resources: string[];
  verbs: string[];
}

// The template is YAML once the lines of Helm actions are gone.
function clusterRoleRules(): Rule[] {
  const text = fs
    .readFileSync(
      path.join(appDir, "deploy", "chart", "clusterrole.yaml"),
      "utf8",
    )
    .split("\n")
    .filter((line) => !line.trim().startsWith("{{"))
    .join("\n")
    .replace(/\{\{.*?\}\}/g, "x");
  return (parse(text) as { rules: Rule[] }).rules;
}

describe("KindMap", () => {
  it.each(crds.map((crd) => [crd.spec.names.kind, crd] as const))(
    "maps %s as its CRD serves it",
    (kind, crd) => {
      expect(KindMap[kind as KindName]).toEqual({
        group: crd.spec.group,
        version: crd.spec.versions.find((v) => v.served && v.storage)!.name,
        plural: crd.spec.names.plural,
        scope: crd.spec.scope,
      });
    },
  );

  it.each(Object.entries(KindMap))(
    "%s can be listed and watched under the ClusterRole of the chart",
    (_kind, info) => {
      const rules = clusterRoleRules();
      for (const verb of ["get", "list", "watch"]) {
        const allowed = rules.some(
          (r) =>
            r.apiGroups.includes(info.group) &&
            r.resources.includes(info.plural) &&
            r.verbs.includes(verb),
        );
        expect(allowed, `${verb} on ${info.plural}.${info.group}`).toBe(true);
      }
    },
  );

  it("builds the collection path of core and group kinds", () => {
    expect(collectionPath(KindMap.Secret)).toBe("/api/v1/secrets");
    expect(collectionPath(KindMap.Environment)).toBe(
      "/apis/fleet.camunda.io/v1alpha1/environments",
    );
  });
});

describe("objectPath", () => {
  it("names one object of a cluster-scoped and of a namespaced kind", () => {
    expect(objectPath(KindMap.Environment, undefined, "prod")).toBe(
      "/apis/fleet.camunda.io/v1alpha1/environments/prod",
    );
    expect(objectPath(KindMap.Secret, "team-a", "git")).toBe(
      "/api/v1/namespaces/team-a/secrets/git",
    );
  });

  it("encodes the namespace and the name", () => {
    expect(objectPath(KindMap.Secret, "a b", "c/d")).toBe(
      "/api/v1/namespaces/a%20b/secrets/c%2Fd",
    );
  });
});

describe("Resource", () => {
  it("types a timestamp as the string that the JSON holds", () => {
    expectTypeOf<
      NonNullable<Resource<"Pod">["metadata"]>["creationTimestamp"]
    >().toEqualTypeOf<string | undefined>();
    expectTypeOf<
      Resource<"Environment">["metadata"]["creationTimestamp"]
    >().toEqualTypeOf<string | undefined>();
  });
});
