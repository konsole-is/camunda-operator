"use client";

import { Monitor, Moon, Sun } from "lucide-react";
import {
  type ReactNode,
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
} from "react";
import { cn } from "@/lib/utils";
import {
  THEME_CHOICES,
  type ThemeChoice,
  applyTheme,
  themeCookie,
} from "@/lib/theme";

type ThemeState = { choice: ThemeChoice; choose: (c: ThemeChoice) => void };

const ThemeContext = createContext<ThemeState | null>(null);

/**
 * ThemeProvider holds the theme choice for ThemeSwitch. initial must be the
 * choice that the class of <html> shows, which the root layout renders from
 * the theme cookie. A choice changes the class and the cookie at once.
 */
export function ThemeProvider({
  initial,
  children,
}: {
  initial: ThemeChoice;
  children: ReactNode;
}) {
  const [choice, setChoice] = useState(initial);
  const choose = useCallback((next: ThemeChoice) => {
    applyTheme(next, document.documentElement);
    document.cookie = themeCookie(next);
    setChoice(next);
  }, []);
  const state = useMemo(() => ({ choice, choose }), [choice, choose]);
  return <ThemeContext value={state}>{children}</ThemeContext>;
}

const LABELS: Record<ThemeChoice, { label: string; Icon: typeof Sun }> = {
  system: { label: "System", Icon: Monitor },
  light: { label: "Light", Icon: Sun },
  dark: { label: "Dark", Icon: Moon },
};

/**
 * ThemeSwitch is a group of three buttons, System, Light, and Dark, that
 * marks the current choice with aria-pressed. It throws outside a
 * ThemeProvider.
 */
export function ThemeSwitch() {
  const state = useContext(ThemeContext);
  if (!state) throw new Error("ThemeSwitch needs a ThemeProvider");
  return (
    <div
      role="group"
      aria-label="Theme"
      className="flex items-center gap-0.5 rounded-lg border border-border p-0.5"
    >
      {THEME_CHOICES.map((choice) => {
        const { label, Icon } = LABELS[choice];
        const pressed = state.choice === choice;
        return (
          <button
            key={choice}
            type="button"
            aria-label={label}
            title={label}
            aria-pressed={pressed}
            onClick={() => state.choose(choice)}
            className={cn(
              "inline-flex size-9 items-center justify-center rounded-md",
              pressed
                ? "bg-panel-2 text-text ring-1 ring-control-edge"
                : "text-muted hover:text-text",
            )}
          >
            <Icon aria-hidden className="size-4" />
          </button>
        );
      })}
    </div>
  );
}
