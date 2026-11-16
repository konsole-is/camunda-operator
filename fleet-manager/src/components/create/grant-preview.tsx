import { useId } from "react";
import { stringify } from "yaml";
import type { GrantPreview } from "@/lib/claims/templates";

/**
 * GrantPreviewList shows the grant templates of the chosen options as they
 * render for a new claim: each with its section and option, its name and
 * description, and the rendered statement as YAML, or the error of the
 * template with its path. While unread names a kind that could not be
 * read, it names that kind and shows no previews, because the list can be
 * incomplete. Without previews and unread kinds, it renders nothing.
 */
export function GrantPreviewList({
  previews,
  unread = [],
}: {
  previews: readonly GrantPreview[];
  unread?: readonly string[];
}) {
  const id = useId();
  if (previews.length === 0 && unread.length === 0) return null;
  return (
    <section aria-labelledby={id} className="flex min-w-0 flex-col gap-3">
      <h3 id={id} className="font-heading text-sm font-semibold">
        Permissions the options ask for
      </h3>
      {unread.length > 0 ? (
        <p role="alert" className="text-sm text-bad">
          {`The fleet manager could not read the ${unread.join(", ")} objects, so the permissions are not known.`}
        </p>
      ) : (
        <ul className="flex min-w-0 flex-col gap-3">
          {previews.map((preview) => (
            <li
              key={`${preview.section}/${preview.registration}/${preview.template}`}
              className="flex min-w-0 flex-col gap-2 rounded-md border border-divider p-3"
            >
              <div className="flex flex-col gap-0.5">
                <p className="text-sm font-medium text-text">
                  {`${preview.sectionLabel}: ${preview.optionLabel}`}
                </p>
                <p className="text-xs text-muted">{`Template ${preview.template}`}</p>
                {preview.description && (
                  <p className="text-xs text-text-2">{preview.description}</p>
                )}
                {preview.onlyWhen && (
                  <p className="text-xs text-text-2">
                    {preview.onlyWhen.length === 1
                      ? `Only when the input ${preview.onlyWhen[0]} has a value`
                      : `Only when the inputs ${preview.onlyWhen.join(", ")} have a value`}
                  </p>
                )}
              </div>
              {"error" in preview.result ? (
                <p role="alert" className="text-sm text-bad">
                  {preview.result.path
                    ? `${preview.result.path}: ${preview.result.error}`
                    : preview.result.error}
                </p>
              ) : (
                <pre className="max-h-72 overflow-auto rounded bg-bg p-3 font-mono text-xs whitespace-pre-wrap text-text-2">
                  {stringify(preview.result.value)}
                </pre>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
