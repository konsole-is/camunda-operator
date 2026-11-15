import { describeIsomorphic } from "../../../test/imports/scan";

// @/lib/policy, @/lib/labels, and @/lib/merge are isomorphic under their
// own guards.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/policy",
  "@/lib/labels",
  "@/lib/merge",
]);
