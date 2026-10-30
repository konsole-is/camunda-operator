import { describeIsomorphic } from "../../../test/imports/scan";

// Each of these is isomorphic under its own guard. @/lib/write/refs is the
// one isomorphic module of the server-only write folder.
describeIsomorphic(new URL(".", import.meta.url), [
  "@/lib/fields",
  "@/lib/labels",
  "@/lib/merge",
  "@/lib/policy",
  "@/lib/schema",
  "@/lib/write/refs",
]);
