import { describeIsomorphic } from "../../../test/imports/scan";

// @/lib/merge, @/lib/labels, and @/lib/policy are isomorphic under their own
// guard. @/generated/fieldschema holds only data.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/merge",
  "@/lib/labels",
  "@/lib/policy",
  "@/generated/fieldschema",
]);
