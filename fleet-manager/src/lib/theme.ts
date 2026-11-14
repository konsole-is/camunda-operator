/**
 * ThemeChoice is a theme that a user can choose. system follows the light or
 * dark setting of the operating system, also when that setting changes while
 * the page is open.
 */
export type ThemeChoice = "system" | "light" | "dark";

/** THEME_CHOICES lists each choice in the order that the switch shows them. */
export const THEME_CHOICES: readonly ThemeChoice[] = [
  "system",
  "light",
  "dark",
];

/**
 * THEME_COOKIE is the cookie that keeps the choice. The root layout reads it
 * and renders the class of <html>, so the first paint has the chosen theme.
 */
export const THEME_COOKIE = "fleet-theme";

/** themeOf returns the choice that value names, or system for any other value. */
export function themeOf(value: string | undefined): ThemeChoice {
  return value === "light" || value === "dark" ? value : "system";
}

/**
 * themeClass returns the class of <html> for choice, or undefined for system:
 * without a class, globals.css takes the theme from prefers-color-scheme.
 */
export function themeClass(choice: ThemeChoice): "light" | "dark" | undefined {
  return choice === "system" ? undefined : choice;
}

/** applyTheme gives root the class of choice and removes the class of any other. */
export function applyTheme(choice: ThemeChoice, root: Element): void {
  root.classList.remove("light", "dark");
  const name = themeClass(choice);
  if (name) root.classList.add(name);
}

const YEAR_SECONDS = 365 * 24 * 60 * 60;

/**
 * themeCookie returns the text for document.cookie that keeps choice for a
 * year. For system it removes the cookie.
 */
export function themeCookie(choice: ThemeChoice): string {
  const name = themeClass(choice);
  return name
    ? `${THEME_COOKIE}=${name}; Path=/; Max-Age=${YEAR_SECONDS}; SameSite=Lax`
    : `${THEME_COOKIE}=; Path=/; Max-Age=0; SameSite=Lax`;
}
