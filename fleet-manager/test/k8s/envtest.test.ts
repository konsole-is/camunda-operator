import fs from "node:fs";
import path from "node:path";
import { ApiextensionsV1Api, KubeConfig } from "@kubernetes/client-node";
import { parseAllDocuments } from "yaml";
import { describe, expect, inject, it } from "vitest";
import { CRD_DIR, startEnvtest } from "../envtest/envtest";

function crdNamesOnDisk(): string[] {
  return fs
    .readdirSync(CRD_DIR)
    .filter((file) => file.endsWith(".yaml"))
    .flatMap((file) =>
      parseAllDocuments(fs.readFileSync(path.join(CRD_DIR, file), "utf8")),
    )
    .map((doc) => doc.toJS() as { metadata: { name: string } })
    .map((crd) => crd.metadata.name)
    .sort();
}

describe("the envtest API server", () => {
  it("serves every CRD of config/crd/bases", async () => {
    const kc = new KubeConfig();
    kc.loadFromFile(inject("kubeconfig"));
    const crds = await kc
      .makeApiClient(ApiextensionsV1Api)
      .listCustomResourceDefinition();

    const served = crds.items.map((crd) => crd.metadata!.name!).sort();
    expect(served).toEqual(crdNamesOnDisk());
  });

  it("does not start without KUBEBUILDER_ASSETS and names make setup-envtest", async () => {
    await expect(startEnvtest({ assetsDir: "" })).rejects.toThrow(
      /KUBEBUILDER_ASSETS is not set.*make setup-envtest/,
    );
  });
});
