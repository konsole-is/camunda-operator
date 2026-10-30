import { describeIsomorphic } from "../../../test/imports/scan";

// @/lib/labels and @/lib/policy are isomorphic under their own guard.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/labels",
  "@/lib/policy",
]);
