"use server";

import {
  type Settings,
  type TestResult,
  testGit,
  testPrometheus,
  updateSettings,
} from "@/lib/settings";
import type { WriteResult } from "@/lib/write/types";

/** saveSettings saves patch when the settings still have resourceVersion. */
export async function saveSettings(
  patch: Partial<Settings>,
  resourceVersion: string,
): Promise<WriteResult> {
  return updateSettings(patch, resourceVersion);
}

/** testGitConnection tests the saved Git settings. */
export async function testGitConnection(): Promise<TestResult> {
  return testGit();
}

/** testPrometheusQuery tests the saved Prometheus settings. */
export async function testPrometheusQuery(): Promise<TestResult> {
  return testPrometheus();
}
