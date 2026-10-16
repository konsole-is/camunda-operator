import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { parse } from "yaml";
import { describe, expect, it } from "vitest";
import { fieldDocs } from "@/generated/fielddocs";
import { typeCheck } from "./typecheck";

const repoDir = fileURLToPath(new URL("../../..", import.meta.url));

describe("fieldDocs", { timeout: 60_000 }, () => {
  it("has the description and the anchor of CamundaCluster spec.zeebe.partitions", () => {
    const crd = parse(
      fs.readFileSync(
        path.join(
          repoDir,
          "config/crd/bases/core.camunda.io_camundaclusters.yaml",
        ),
        "utf8",
      ),
    );
    const schema = crd.spec.versions[0].schema.openAPIV3Schema;
    const description =
      schema.properties.spec.properties.zeebe.properties.partitions.description;

    expect(
      fieldDocs["core.camunda.io"]?.CamundaCluster?.["spec.zeebe.partitions"],
    ).toEqual({ description, anchor: "#zeebespec" });
    expect(description).toMatch(/^Partitions is the number of partitions\./);
  });

  it.each([
    "core.camunda.io",
    "plugins.camunda.io",
    "fleet.camunda.io",
  ] as const)(
    "links every %s field to a type of the API reference",
    (group) => {
      const reference = fs.readFileSync(
        path.join(repoDir, "docs/crds/api-reference.md"),
        "utf8",
      );
      const anchors = new Set(
        [...reference.matchAll(/^#### (\w+)$/gm)].map(
          (m) => `#${m[1].toLowerCase()}`,
        ),
      );

      const kinds = Object.entries(fieldDocs[group] ?? {});
      expect(kinds).not.toEqual([]);
      const missing = kinds.flatMap(([kind, paths]) =>
        Object.entries(paths ?? {})
          .filter(([, doc]) => !anchors.has(doc?.anchor ?? ""))
          .map(([field, doc]) => `${kind} ${field} ${doc?.anchor}`),
      );
      expect(missing).toEqual([]);
    },
  );

  it("makes a caller handle a group, a kind, or a path without an entry", () => {
    const name = "test/generated/__virtual__/lookup.ts";
    const result = typeCheck({
      [name]: `import { fieldDocs } from "../../../src/generated/fielddocs";\nexport const description: string = fieldDocs["a.camunda.io"]["Kind"]["spec.x"].description;\n`,
    });

    expect(result[name]).not.toEqual([]);
  });
});
