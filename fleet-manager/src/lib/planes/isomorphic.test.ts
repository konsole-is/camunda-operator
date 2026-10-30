import { describeIsomorphic } from "../../../test/imports/scan";

// @/lib/labels, @/lib/claims/names, and @/lib/environments are isomorphic
// under their own guards.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/labels",
  "@/lib/claims/names",
  "@/lib/environments",
]);
