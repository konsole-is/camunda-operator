import { expect, test } from "@playwright/test";
import { smallTargets } from "./targets";

test("the target check skips a link in a sentence and counts links side by side", async ({
  page,
}) => {
  await page.setContent(
    '<p>Read the <a href="#a">guide</a> first.</p>' +
      '<nav><a href="#b">One</a> <a href="#c">Two</a></nav>',
  );
  const lines = await smallTargets(page);
  expect(lines.filter((l) => l.includes('"guide"'))).toEqual([]);
  expect(lines.filter((l) => /"(One|Two)"/.test(l))).toHaveLength(2);
});

test("the target check counts only the part of a hit area that no other control covers", async ({
  page,
}) => {
  await page.setContent(
    "<style>" +
      ".row { position: relative; height: 20px; }" +
      '.row::after { content: ""; position: absolute; inset: -2px 0; }' +
      "</style>" +
      '<div role="button" class="row">First</div>' +
      '<div role="button" class="row">Second</div>' +
      '<div role="button" class="row" style="margin-top: 8px">Third</div>',
  );
  const lines = await smallTargets(page);
  expect(lines.filter((l) => l.includes('"First"'))).toHaveLength(1);
  expect(lines.filter((l) => l.includes('"Third"'))).toEqual([]);
});

test("the target check skips a control inside a closed details and counts its summary", async ({
  page,
}) => {
  await page.setContent(
    '<details><summary style="font-size: 8px">Brokers</summary>' +
      '<input aria-label="replicas" style="width: 10px"></details>',
  );
  const lines = await smallTargets(page);
  expect(lines.filter((l) => l.includes('"replicas"'))).toEqual([]);
  expect(lines.filter((l) => l.includes('"Brokers"'))).toHaveLength(1);
});

test("the target check skips the controls behind an open modal dialog and counts the dialog", async ({
  page,
}) => {
  // A modal Radix dialog turns pointer events off on the body and on again
  // on the dialog, and hides the rest of the page with aria-hidden.
  await page.setContent(
    '<body style="pointer-events: none">' +
      '<div aria-hidden="true"><button style="width: 10px">Behind</button></div>' +
      '<div role="dialog" style="pointer-events: auto">' +
      '<button style="font-size: 8px">Close</button></div></body>',
  );
  const lines = await smallTargets(page);
  expect(lines.filter((l) => l.includes('"Behind"'))).toEqual([]);
  expect(lines.filter((l) => l.includes('"Close"'))).toHaveLength(1);
});

test("the target check skips a control inside inert content", async ({
  page,
}) => {
  await page.setContent(
    '<div inert><button style="width: 10px">Inert</button></div>',
  );
  expect(await smallTargets(page)).toEqual([]);
});

test("the target check counts a control that a pointer can click under aria-hidden", async ({
  page,
}) => {
  await page.setContent(
    '<div aria-hidden="true"><button style="width: 10px">Hidden</button></div>',
  );
  const lines = await smallTargets(page);
  expect(lines.filter((l) => l.includes('"Hidden"'))).toHaveLength(1);
});

test("the target check counts a control without pointer events whose label takes the pointer", async ({
  page,
}) => {
  await page.setContent(
    '<label style="font-size: 8px">Pick <input type="checkbox" aria-label="pick"' +
      ' style="pointer-events: none; width: 8px; height: 8px"></label>',
  );
  const lines = await smallTargets(page);
  expect(lines.filter((l) => l.includes('"pick"'))).toHaveLength(1);
});

test("the target check counts a control without pointer events whose child takes the pointer", async ({
  page,
}) => {
  await page.setContent(
    '<button style="pointer-events: none; font-size: 8px">' +
      '<span style="pointer-events: auto">Go</span></button>',
  );
  const lines = await smallTargets(page);
  expect(lines.filter((l) => l.includes('"Go"'))).toHaveLength(1);
});

test("the target check skips a control without pointer events whose only child that takes the pointer is hidden", async ({
  page,
}) => {
  await page.setContent(
    '<button aria-label="hidden child" style="pointer-events: none; font-size: 8px">' +
      'Go<span style="pointer-events: auto; visibility: hidden">x</span></button>',
  );
  const lines = await smallTargets(page);
  expect(lines.filter((l) => l.includes('"hidden child"'))).toEqual([]);
});

test("the target check skips a control without pointer events whose only child that takes the pointer is empty", async ({
  page,
}) => {
  await page.setContent(
    '<button aria-label="empty child" style="pointer-events: none; font-size: 8px">' +
      'Go<span style="pointer-events: auto"></span></button>',
  );
  const lines = await smallTargets(page);
  expect(lines.filter((l) => l.includes('"empty child"'))).toEqual([]);
});
