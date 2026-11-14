import { GLOBALS_UPDATED, SET_GLOBALS } from "storybook/internal/core-events";
import { addons } from "storybook/manager-api";
import { storybookTheme, themes } from "./theme";

addons.setConfig({ theme: themes.dark });

// The frame of the Storybook follows the theme global of the toolbar. The
// preview sends SET_GLOBALS once it read the globals of the URL.
addons.register("fleet-manager/theme", (api) => {
  const follow = ({ globals }: { globals: Record<string, unknown> }) =>
    addons.setConfig({ theme: themes[storybookTheme(globals.theme)] });
  const channel = api.getChannel();
  channel?.on(SET_GLOBALS, follow);
  channel?.on(GLOBALS_UPDATED, follow);
});
