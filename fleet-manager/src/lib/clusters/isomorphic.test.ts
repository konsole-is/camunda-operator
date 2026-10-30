import { describeIsomorphic } from "../../../test/imports/scan";

// The modules of @/lib that the draft imports are isomorphic under their own
// guards.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/claims/names",
  "@/lib/environments",
  "@/lib/labels",
  "@/lib/policy",
]);
