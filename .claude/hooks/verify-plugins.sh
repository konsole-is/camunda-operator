#!/bin/sh
# Warn when a plugin that this repository declares is not installed.
#
# The enabledPlugins block in .claude/settings.json only enables a plugin. It
# does not install one. Claude Code loads the skills of a plugin only when the
# install registry holds a record for that plugin. If the record is absent,
# every skill of the plugin fails with "skill not found", and the cause is not
# obvious, because settings.json still lists the plugin as enabled.
#
# This hook does not repair the state. It reports the problem and gives the
# command that repairs it.

set -u

config_dir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
registry="$config_dir/plugins/installed_plugins.json"

for plugin in ocf@operator-component-framework simple-english@simple-english feature-dev-workflow@feature-dev-workflow; do
	if ! grep -q "\"$plugin\"" "$registry" 2>/dev/null; then
		echo "The plugin $plugin is not installed, so its skills will not load."
		echo "To install it, run: claude plugin install $plugin --scope user"
		echo "Then start a new session."
	fi
done

exit 0
