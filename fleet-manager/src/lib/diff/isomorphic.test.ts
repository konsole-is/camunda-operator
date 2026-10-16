import { describeIsomorphic } from "../../../test/imports/scan";

describeIsomorphic(new URL(".", import.meta.url), ["yaml"]);
