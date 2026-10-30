import { describeIsomorphic } from "../../../test/imports/scan";

// @/lib/labels and @/lib/claims/names are isomorphic under their own guards.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/labels",
  "@/lib/claims/names",
]);
