import { describeIsomorphic } from "../../../test/imports/scan";

// @/lib/merge and @/lib/labels are isomorphic under their own guard.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/merge",
  "@/lib/labels",
]);
