import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { parse } from "yaml";
import { describe, expect, it } from "vitest";
import { fieldDocs } from "@/generated/fielddocs";

const repoDir = fileURLToPath(new URL("../../..", import.meta.url));

describe("fieldDocs", () => {
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
      fieldDocs["core.camunda.io"].CamundaCluster["spec.zeebe.partitions"],
    ).toEqual({ description, anchor: "#zeebespec" });
    expect(description).toMatch(/^Partitions is the number of partitions\./);
  });

  it("links every core.camunda.io field to a type of the API reference", () => {
    const reference = fs.readFileSync(
      path.join(repoDir, "docs/crds/api-reference.md"),
      "utf8",
    );
    const anchors = new Set(
      [...reference.matchAll(/^#### (\w+)$/gm)].map(
        (m) => `#${m[1].toLowerCase()}`,
      ),
    );

    const missing = Object.entries(fieldDocs["core.camunda.io"]).flatMap(
      ([kind, paths]) =>
        Object.entries(paths)
          .filter(([, doc]) => !anchors.has(doc.anchor))
          .map(([field, doc]) => `${kind} ${field} ${doc.anchor}`),
    );
    expect(missing).toEqual([]);
  });
});
