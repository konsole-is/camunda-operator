import { describe, expect, it } from "vitest";
import {
  EMPTY_ADMIN,
  EMPTY_MAIL,
  adminErrors,
  mailErrors,
  runningOf,
} from "./components";
import { EMPTY_PLANE_DRAFT, type PlaneDraft } from "./draft";

const draft = (over: Partial<PlaneDraft> = {}): PlaneDraft => ({
  ...EMPTY_PLANE_DRAFT,
  mode: "keycloak",
  admin: { ...EMPTY_ADMIN, username: "admin" },
  ...over,
});
const mail = { ...EMPTY_MAIL, smtpHost: "smtp", fromAddress: "a@b.c" };

describe("EMPTY_MAIL", () => {
  it("starts at port 587 with STARTTLS on", () => {
    expect(EMPTY_MAIL).toMatchObject({ smtpPort: 587, tls: true });
  });
});

describe("runningOf", () => {
  it("runs Keycloak only in Keycloak mode, and Console and Web Modeler when on", () => {
    expect(runningOf(draft({ console: true }))).toEqual({
      keycloak: true,
      console: true,
      webModeler: false,
    });
    expect(
      runningOf(draft({ mode: "externalKeycloak", webModeler: true })),
    ).toEqual({ keycloak: false, console: false, webModeler: true });
  });
});

describe("adminErrors", () => {
  it.each(["keycloak", "externalKeycloak"] as const)(
    "asks for the username in %s mode",
    (mode) => {
      expect(adminErrors(draft({ mode, admin: EMPTY_ADMIN }))).toEqual([
        "Enter the username of the first administrator.",
      ]);
    },
  );

  it("refuses an email shorter than 3 characters", () => {
    expect(
      adminErrors(
        draft({ admin: { ...EMPTY_ADMIN, username: "a", email: "x" } }),
      ),
    ).toEqual(["Enter a valid email address."]);
  });

  it("needs the email while Web Modeler is on in a Keycloak mode", () => {
    expect(adminErrors(draft({ webModeler: true }))).toEqual([
      "Web Modeler needs the email of the first administrator.",
    ]);
  });

  it("asks for the claim name and value in OIDC mode", () => {
    expect(adminErrors(draft({ mode: "oidc" }))).toEqual([
      "Enter the claim name of the first administrator.",
      "Enter the claim value of the first administrator.",
    ]);
  });

  it("refuses a claim name with an equals sign", () => {
    expect(
      adminErrors(
        draft({
          mode: "oidc",
          admin: { ...EMPTY_ADMIN, claimName: "a=b", claimValue: "1" },
        }),
      ),
    ).toEqual(["The claim name cannot hold an equals sign."]);
  });

  it("asks for no username in OIDC mode", () => {
    expect(
      adminErrors(
        draft({
          mode: "oidc",
          webModeler: true,
          admin: { ...EMPTY_ADMIN, claimName: "sub", claimValue: "42" },
        }),
      ),
    ).toEqual([]);
  });
});

describe("mailErrors", () => {
  it("passes a complete mail", () => {
    expect(mailErrors(mail)).toEqual([]);
  });

  it.each([
    [{ smtpHost: "" }, "Enter the SMTP host."],
    [{ fromAddress: "ab" }, "Enter the address that Web Modeler sends from."],
    [{ smtpPort: 0 }, "Enter an SMTP port from 1 to 65535."],
    [{ smtpPort: 65536 }, "Enter an SMTP port from 1 to 65535."],
    [{ smtpPort: 25.5 }, "Enter an SMTP port from 1 to 65535."],
  ])("refuses %o", (over, message) => {
    expect(mailErrors({ ...mail, ...over })).toEqual([message]);
  });
});
