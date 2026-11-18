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
