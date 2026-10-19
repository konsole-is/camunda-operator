import "server-only";

export type { SettingKey, Settings } from "./keys";
export { getSettings, updateSettings } from "./store";
export { type TestResult, testGit, testPrometheus } from "./connections";
