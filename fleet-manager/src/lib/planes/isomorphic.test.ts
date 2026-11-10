import { describeIsomorphic } from "../../../test/imports/scan";

// @/lib/labels, @/lib/claims/names, @/lib/environments, and @/lib/policy are
// isomorphic under their own guards.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/labels",
  "@/lib/claims/names",
  "@/lib/environments",
  "@/lib/policy",
]);
