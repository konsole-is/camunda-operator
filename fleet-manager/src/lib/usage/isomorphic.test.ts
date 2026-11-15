import { describeIsomorphic } from "../../../test/imports/scan";

// @/lib/policy and @/lib/merge are isomorphic under their own guards.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/policy",
  "@/lib/merge",
]);
