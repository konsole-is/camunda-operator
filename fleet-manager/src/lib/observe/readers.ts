import "server-only";

// A route or a page imports these readers from here, never from kube.ts or
// pod-log.ts. The envtest build swaps this module for readers.envtest.ts,
// which the page scenarios on envtest need (next.config.ts).
export { podUsage } from "./kube";
export { followPodLog } from "./pod-log";
