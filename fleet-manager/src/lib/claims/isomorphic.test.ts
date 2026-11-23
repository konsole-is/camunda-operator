import { describeIsomorphic } from "../../../test/imports/scan";

// @/lib/labels, @/lib/policy, and @/lib/session are isomorphic under their
// own guard, and @/lib/write/ownership and @/lib/write/refs under the guard
// of lib/write. Client components already import the last three to build a
// claim write.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/gotemplate",
  "@/lib/labels",
  "@/lib/policy",
  "@/lib/session",
  "@/lib/write/ownership",
  "@/lib/write/refs",
  "@noble/hashes/sha2.js",
  "@noble/hashes/utils.js",
]);
