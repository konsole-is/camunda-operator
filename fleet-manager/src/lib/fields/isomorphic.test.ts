import { describeIsomorphic } from "../../../test/imports/scan";

// @/lib/merge, @/lib/labels, and @/lib/policy are isomorphic under their own guard.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/merge",
  "@/lib/labels",
  "@/lib/policy",
]);
