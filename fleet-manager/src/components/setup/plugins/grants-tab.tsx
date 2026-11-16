import type { ReactNode } from "react";
import { Panel, Term, Terms } from "@/components/cluster/claim-tab";
import { ConditionList } from "@/components/fleet/condition-list";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import type { Grant } from "@/generated/crds/grant";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import {
  GRANT_REASON_TEXT,
  type GrantTemplateView,
  type GrantView,
  grantTemplates,
} from "./grants";
import { NotKnown, ValueError } from "./tab-parts";

const WITHOUT_TEMPLATE = "Grants of a template the registration no longer has";

/**
 * GrantsTab shows each grant template of registration with its Grants of
 * grants, and the Grants of a template that the registration no longer has.
 * While Grant is in unread, it shows no counts and says that the Grants are
 * not known. now is the time that the ages count to.
 */
export function GrantsTab({
  registration,
  grants,
  unread,
  now,
}: {
  registration: PluginRegistration;
  grants: readonly Grant[];
  unread: readonly string[];
  now: string;
}): ReactNode {
  const { templates, withoutTemplate } = grantTemplates(registration, grants);
  const grantsUnread = unread.includes("Grant");
  if (templates.length === 0 && withoutTemplate.length === 0)
    return grantsUnread ? (
      <div className="flex flex-col gap-1">
        <p className="text-sm">This option has no grant template.</p>
        <NotKnown kinds={["Grant"]} />
      </div>
    ) : (
      <p className="text-sm text-muted">
        This option owns the policies of its role and writes no Grant.
      </p>
    );
  return (
    <div className="flex flex-col gap-4">
      {grantsUnread && (
        <p className="text-sm text-muted">
          The lists can miss Grants: Grant is not read.
        </p>
      )}
      {templates.map((template, i) =>
        "error" in template ? (
          <ValueError key={i} text={template.error} />
        ) : (
          <TemplatePanel
            key={template.name}
            template={template}
            unread={grantsUnread}
            now={now}
          />
        ),
      )}
      {withoutTemplate.length > 0 && (
        <Panel label={WITHOUT_TEMPLATE}>
          <Grants grants={withoutTemplate} now={now} />
        </Panel>
      )}
    </div>
  );
}

function TemplatePanel({
  template,
  unread,
  now,
}: {
  template: GrantTemplateView;
  unread: boolean;
  now: string;
}) {
  return (
    <Panel label={template.name}>
      {template.description && (
        <p className="text-sm text-text-2">{template.description}</p>
      )}
      <Terms>
        <Term label="Format">
          <span className="font-mono">{template.format}</span>
        </Term>
        <Term label="Needs the inputs">
          {template.requiresInputs.length > 0 ? (
            <span className="font-mono">
              {template.requiresInputs.join(", ")}
            </span>
          ) : (
            "None"
          )}
        </Term>
        <Term label="Grants">
          {unread ? (
            <NotKnown kinds={["Grant"]} />
          ) : (
            `${template.ready} ready, ${template.refused} refused`
          )}
        </Term>
      </Terms>
      <pre
        role="region"
        aria-label={`Statement of ${template.name}`}
        tabIndex={0}
        className="overflow-x-auto rounded-md bg-panel-2 p-3 text-xs outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <code>{template.template}</code>
      </pre>
      {template.grants.length > 0 ? (
        <Grants grants={template.grants} now={now} />
      ) : (
        !unread && (
          <p className="text-sm text-muted">No Grant of this template yet.</p>
        )
      )}
    </Panel>
  );
}

/** Grants lists grants, each with its cluster, its request, and its conditions. */
export function Grants({
  grants,
  now,
}: {
  grants: readonly GrantView[];
  now: string;
}): ReactNode {
  return (
    <ul className="flex flex-col divide-y divide-divider">
      {grants.map((grant) => (
        <li
          key={`${grant.namespace}/${grant.name}`}
          className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0"
        >
          <span className="font-mono text-sm break-all">{`${grant.namespace}/${grant.name}`}</span>
          <Terms>
            <Term label="Cluster">{grant.cluster ?? "-"}</Term>
            <Term label="Request">
              <span className="font-mono break-all">
                {grant.request ?? "-"}
              </span>
            </Term>
          </Terms>
          {grant.error ? (
            <ValueError text={grant.error} />
          ) : (
            <>
              <ConditionList
                label={`Conditions of ${grant.name}`}
                conditions={conditionsOf(grant)}
                now={now}
              />
              {refusal(grant) && (
                <p className="text-sm text-warn">{refusal(grant)}</p>
              )}
            </>
          )}
        </li>
      ))}
    </ul>
  );
}

function conditionsOf(grant: GrantView): ConditionRow[] {
  return [grant.applied, grant.matchesTemplate, grant.ready].filter(
    (c): c is ConditionRow => c !== undefined,
  );
}

// refusal gives the meaning of the reason of the first False condition of a
// refused Grant, when GRANT_REASON_TEXT has one.
function refusal(grant: GrantView): string | undefined {
  if (grant.state !== "refused") return undefined;
  const reason = conditionsOf(grant).find((c) => c.status === "False")?.reason;
  return reason && Object.hasOwn(GRANT_REASON_TEXT, reason)
    ? GRANT_REASON_TEXT[reason as keyof typeof GRANT_REASON_TEXT]
    : undefined;
}
