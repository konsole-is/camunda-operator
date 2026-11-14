# Fleet manager

The fleet manager is the web app of the operator. It is a Next.js app. The Make targets of the repository root
install it, test it, and build it. Run each `make` command of this page from the repository root.

## Run Storybook

Storybook shows each UI component of the app in its states, with the styles, fonts, and theme of the app.

1. Install the dependencies from the repository root:

   ```bash
   make fleet-install
   ```

2. Start Storybook in `fleet-manager/`:

   ```bash
   cd fleet-manager
   ../bin/pnpm-<version>/bin/pnpm storybook
   ```

   `make fleet-install` puts pnpm into `bin/`. The version is the `packageManager` field of `package.json`.
   If pnpm of that version is on your `PATH`, `pnpm storybook` is the same command.

3. Open <http://localhost:6006>.

Each component has a docs page and one entry for each story. The Controls panel changes the props, the Actions
panel shows each call of an event handler, and the Accessibility panel runs axe on the story.

The **Theme** menu of the toolbar selects the dark theme or the light theme of the app. The stories, the docs pages,
and the frame of Storybook change to that theme. Storybook opens in the dark theme. To open a story in the light
theme from a link, add `&globals=theme:light` to its URL.

## Write a story

Put the stories of a component in a stories file next to the component. Name the file after the component in kebab
case, with the extension `.stories.tsx`: the stories of `StatusChip` go into
`src/components/fleet/status-chip.stories.tsx`. One stories file holds the stories of one component.

```tsx
import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { BackupHeaderActions } from "./backup-actions-view";

const meta = {
  title: "cluster/BackupActions",
  component: BackupHeaderActions,
  args: { context: context("elasticsearch"), apply: fn(applied), show: fn() },
} satisfies Meta<typeof BackupHeaderActions>;
export default meta;
type Story = StoryObj<typeof meta>;

export const BackUpNowWithoutBackupStorage: Story = {
  name: "Back up now without backup storage",
  args: { context: context("none") },
};
```

In this example, `context` and `applied` are fixtures at the top of the stories file.

Obey these rules:

- Set `title` to the directory under `src/components`, a slash, and the component name. The test
  `src/stories.test.tsx`, which `make fleet-test` runs, fails on another title.
- Set `name` on each story. The export name sets only the id of the story.
- Put the props in `args`, so that the Controls panel can change them. Put shared props in the `args` of `meta`.
- Use `fn()` for each event handler. If the handler returns a promise, use `fn(async () => value)`.
- Use `render` only for markup around the component. Put a layout wrapper into `decorators`.
- If a user gets to a state through the UI, write a `play` function that does the same input.
- If a component reads the path or the query, set them with
  `parameters: { nextjs: { navigation: { pathname, query } } }`.
- Render a view component with fixture data. Do not render a component that reads live data: a `*Live` component,
  or a component that calls `useResources`, `useResource`, `useMetrics`, or `fetch`. A static Storybook has no
  server, so such a story shows no data.
- Use invented fixture data, such as `example.com`, `acme-backups`, or `team-a`. Do not use a real host, account
  ID, token, or Secret value.
- Put fixtures that one stories file uses at the top of that file. Put fixtures that more than one stories file
  uses into `fixtures.ts` in the directory of the component that owns them.

A story can import `@/lib/fielddocs`, because Storybook replaces `server-only` with an empty module. A module that
uses Node.js APIs, such as the file system or the Kubernetes client, fails in the browser. Give the story fixture
data instead.

Some components still have their stories in the older gallery format: a stories file that exports a list and has
no default export. Storybook does not read these files. For a new component, write a Storybook stories file.

## Check the stories

Run the stories check from the repository root:

```bash
make fleet-stories-test
```

The check builds Storybook into `fleet-manager/storybook-static` and serves the build under a sub-path, as GitHub
Pages does. Then it opens each story at 1440 px and at 390 px, first in the dark theme and then in the light
theme. The check fails when one of these occurs:

- The story throws when it renders.
- The `play` function of the story fails.
- The page scrolls sideways.
- axe finds a violation of serious or critical impact, in one of the two themes.
- The story requests a file outside the Storybook folder, for example from `/api/watch` or from another host.

The check writes a screenshot of each story at each width and in each theme into `fleet-manager/ui-stories`. The
file of the light theme has the suffix `-light`. Open `fleet-manager/ui-stories/index.html` to see all of them on one
page.

In CI, the `Fleet manager` workflow (`.github/workflows/fleet-manager.yml`) runs the same check. The run uploads two artifacts: `storybook` holds the
static build for 7 days, and `ui-stories` holds the screenshots and `index.html`.

To build Storybook without the check, run `make fleet-build-stories`.

## Open the Storybook of a pull request

When a pull request from this repository changes `fleet-manager/`, CI publishes its Storybook on GitHub Pages at
`https://konsole-is.github.io/camunda-operator/pr-preview/pr-<number>/`. A comment on the pull request gives the
link and the commit of the build.

- Each push replaces the preview and edits the same comment. After a push, GitHub Pages can take a minute to show
  the new build.
- CI publishes the preview when Storybook builds, also when another check fails.
- A pull request from a fork gets no preview. To see its stories, download the `storybook` artifact of its run.
- When the pull request closes, merged or not, CI removes the preview and edits the comment. A weekly run removes a
  preview that a closed pull request left.
- The Storybook of `main` is at <https://konsole-is.github.io/camunda-operator/storybook/>.

The preview is public, so a story uses invented fixture data only.
