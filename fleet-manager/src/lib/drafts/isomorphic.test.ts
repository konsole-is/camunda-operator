import { describeIsomorphic } from "../../../test/imports/scan";

// The modules of @/lib that the draft checks import are isomorphic under
// their own guards.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/policy",
  "@/lib/schema",
]);
