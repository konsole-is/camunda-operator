import { sha256 } from "@noble/hashes/sha2.js";
import { bytesToHex, utf8ToBytes } from "@noble/hashes/utils.js";
import { describe, expect, it } from "vitest";
import { requestName } from "./requests";

const hashOf = (name: string) =>
  bytesToHex(sha256(utf8ToBytes(name))).slice(0, 8);

// The cases of pkg/components/claimrequests/names_test.go.
describe("requestName", () => {
  const mail = { section: "alerts", registration: "mail" };

  it("names a request of a One section <claim>-<section>", () => {
    expect(requestName("orders", mail, "One")).toBe("orders-alerts");
  });

  it("treats an unknown mode as One", () => {
    expect(requestName("orders", mail, "")).toBe("orders-alerts");
  });

  it("names a request of a Many section <claim>-<section>-<registration>", () => {
    expect(requestName("orders", mail, "Many")).toBe("orders-alerts-mail");
  });

  it("keeps a name of 63 characters", () => {
    const claim = "c".repeat(63 - "-alerts".length);
    expect(
      requestName(claim, { section: "alerts", registration: "" }, ""),
    ).toBe(`${claim}-alerts`);
  });

  it("cuts a longer name to 54 characters and adds 8 hex characters of its SHA-256", () => {
    const claim = "c".repeat(40);
    const registration = "r".repeat(30);
    const full = `${claim}-alerts-${registration}`;
    expect(
      requestName(claim, { section: "alerts", registration }, "Many"),
    ).toBe(`${full.slice(0, 54)}-${hashOf(full)}`);
  });

  it("drops a dot or a dash before the hash", () => {
    const claim = "c".repeat(40);
    const section = "s".repeat(12) + "." + "s".repeat(20);
    const full = `${claim}-${section}`;
    expect(requestName(claim, { section, registration: "" }, "One")).toBe(
      `${full.slice(0, 53)}-${hashOf(full)}`,
    );
  });
});
