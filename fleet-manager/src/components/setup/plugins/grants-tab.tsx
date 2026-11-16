import type { ReactNode } from "react";
import { Panel, Term, Terms } from "@/components/cluster/claim-tab";
import { ConditionList } from "@/components/fleet/condition-list";
import type { Grant } from "@/generated/crds/grant";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { ConditionBadge } from "./condition-badge";
import {
  GRANT_CONDITIONS,
  GRANT_REASON_TEXT,
  type GrantTemplateView,
  type GrantView,
  type InvalidTemplate,
  TEMPLATE_GONE,
  WITHOUT_TEMPLATE,
  grantConditions,
  grantTemplates,
  refusalText,
} from "./grants";
import { NotKnown, ValueError } from "./tab-parts";

/**
 * GrantsTab shows each grant template of registration with its Grants of
 * grants, and the Grants of a template that the registration no longer has.
 * While Grant is in unread, it shows no counts and no condition of a Grant,
 * and names the kind. now is the time that the ages count to.
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
        This option has no grant template, so it writes no Grant.
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
          <InvalidTemplatePanel
            key={i}
            template={template}
            unread={grantsUnread}
            now={now}
          />
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
          <p className="text-sm text-text-2">{TEMPLATE_GONE}</p>
          <Grants
            grants={withoutTemplate}
            unread={grantsUnread}
            templateGone
            now={now}
          />
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
            `${template.ready} ready, ${template.refused} refused${template.oldTemplate > 0 ? `, ${template.oldTemplate} on an old template` : ""}`
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
        <Grants grants={template.grants} unread={unread} now={now} />
      ) : (
        !unread && (
          <p className="text-sm text-muted">No Grant of this template yet.</p>
        )
      )}
    </Panel>
  );
}

function InvalidTemplatePanel({
  template,
  unread,
  now,
}: {
  template: InvalidTemplate;
  unread: boolean;
  now: string;
}) {
  if (template.name === undefined) return <ValueError text={template.error} />;
  return (
    <Panel label={template.name}>
      <ValueError text={template.error} />
      {template.grants.length > 0 && (
        <Grants grants={template.grants} unread={unread} now={now} />
      )}
    </Panel>
  );
}

function Grants({
  grants,
  unread,
  templateGone = false,
  now,
}: {
  grants: readonly GrantView[];
  unread: boolean;
  templateGone?: boolean;
  now: string;
}) {
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
          {unread ? (
            <GrantConditionsNotKnown />
          ) : grant.error ? (
            <ValueError text={grant.error} />
          ) : (
            <>
              <ConditionList
                label={`Conditions of ${grant.name}`}
                conditions={grantConditions(grant)}
                now={now}
              />
              {grant.state === "oldTemplate" && !templateGone && (
                <p className="text-sm text-text-2">
                  {GRANT_REASON_TEXT.TemplateChanged}
                </p>
              )}
              {refusalText(grant) && (
                <p className="text-sm text-warn">{refusalText(grant)}</p>
              )}
            </>
          )}
        </li>
      ))}
    </ul>
  );
}

/** GrantConditionsNotKnown shows each condition of GRANT_CONDITIONS as Not known. */
export function GrantConditionsNotKnown() {
  return (
    <span className="flex flex-wrap gap-1.5">
      {GRANT_CONDITIONS.map((type) => (
        <ConditionBadge key={type} type={type} unknown />
      ))}
    </span>
  );
}
