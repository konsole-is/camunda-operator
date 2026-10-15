import type { TestProject } from "vitest/node";
import { startEnvtest } from "./envtest";

declare module "vitest" {
  export interface ProvidedContext {
    /** kubeconfig is the path of an admin kubeconfig of the envtest API server. */
    kubeconfig: string;
  }
}

// The Vitest project envtest starts one API server for all its files.
export default async function setup(project: TestProject) {
  const envtest = await startEnvtest();
  project.provide("kubeconfig", envtest.kubeconfigPath);
  return () => envtest.stop();
}
