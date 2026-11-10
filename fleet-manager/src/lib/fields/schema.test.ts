import { describe, expect, it } from "vitest";
import { fullSchemas } from "@/generated/fieldschema";
import { componentFieldPaths, fieldSchemaOf, fullSchemaOf } from "./schema";

describe("componentFieldPaths", () => {
  it("offers the fields of a process block, also when no layer sets them", () => {
    const paths = componentFieldPaths("tasklist");

    expect(paths).toEqual(
      expect.arrayContaining([
        "tasklist.replicas",
        "tasklist.mode",
        "tasklist.resources",
        "tasklist.extraEnv",
        "tasklist.podLabels",
        "tasklist.scheduling.tolerations",
      ]),
    );
    expect(paths.every((p) => p.startsWith("tasklist."))).toBe(true);
    expect(paths).not.toContain("tasklist.resources.requests");
    expect(paths).not.toContain("tasklist.scheduling");
    expect(paths).toEqual([...paths].sort());
  });

  it("leaves out the fields that a claim cannot set", () => {
    const connectors = componentFieldPaths("connectors");
    expect(connectors).toContain("connectors.enabled");
    expect(connectors).not.toContain("connectors.version");
    expect(connectors).not.toContain("connectors.serviceAccount.name");

    const cluster = componentFieldPaths("cluster");
    for (const p of [
      "presetRef",
      "releaseRef",
      "version",
      "storageRef",
      "serviceAccount.name",
    ])
      expect(cluster).not.toContain(p);
    expect(cluster).toEqual(
      expect.arrayContaining(["extraEnv", "podLabels", "scheduling.tolerations"]),
    );
    expect(cluster.some((p) => p.startsWith("zeebe."))).toBe(false);
  });

  it("offers the Optimize attachment by its claim names", () => {
    expect(componentFieldPaths("optimize")).toEqual(
      expect.arrayContaining([
        "managementAuthConfigRef",
        "importer.replicas",
        "webapp.resources",
      ]),
    );
  });
});

it("gives the type of a field", () => {
  expect(fieldSchemaOf("cluster", "tasklist.replicas")).toEqual({
    type: "integer",
    format: "int32",
    minimum: 0,
  });
  expect(fieldSchemaOf("cluster", "gateway.mode")?.enum).toEqual([
    "Standalone",
    "Embedded",
  ]);
  expect(fieldSchemaOf("optimize", "zeebe.replicas")).toBeUndefined();
});

it("gives the whole schema of extraEnvFrom and the affinities by their name", () => {
  expect(fullSchemaOf("zeebe.extraEnvFrom")).toBe(fullSchemas.extraEnvFrom);
  expect(fullSchemaOf("scheduling.podAffinity")).toBe(fullSchemas.podAffinity);
  expect(fullSchemaOf("nodeAffinity")).toBe(fullSchemas.nodeAffinity);
  expect(fullSchemaOf("zeebe.replicas")).toBeUndefined();
});

it("readers give undefined for an Object.prototype name", () => {
  expect(fieldSchemaOf("cluster", "constructor")).toBeUndefined();
  expect(fullSchemaOf("x.toString")).toBeUndefined();
});
