import { describeIsomorphic } from "../../../test/imports/scan";

// @/lib/policy is isomorphic under its own guard.
describeIsomorphic(new URL(".", import.meta.url), ["@/lib/policy"]);
