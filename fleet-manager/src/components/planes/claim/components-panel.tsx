"use client";

import { useContext, useId } from "react";
import { errorsAt } from "@/components/cluster/settings-editor";
import { SecretRefsReaderContext } from "@/components/fleet/namespaced-secret-ref-picker";
import { MailFields } from "@/components/planes/create/components-step";
import { Checkbox } from "@/components/ui/checkbox";
import type { Step } from "@/lib/fields";
import type { MailDraft } from "@/lib/planes/components";
import { mailDraftOf } from "@/lib/planes/draft";
import { componentEdits } from "@/lib/planes/edits";
import { CONSOLE_OFF, CONSOLE_ON, WEB_MODELER_OFF } from "@/lib/planes/impacts";
import { claimAfter } from "@/lib/session";
import { FieldErrors } from "./fields";
import { usePlaneClaimEdit } from "./plane-claim-edit";

const MAIL: Step[] = [
  { field: "spec" },
  { field: "management" },
  { field: "webModeler" },
  { field: "mail" },
];

/**
 * ComponentsPanel turns Console and Web Modeler on or off, and edits the
 * mail of Web Modeler while it runs. While a switch differs from the live
 * claim, it says what the change does. Turning an app off also removes its
 * version. It renders nothing without a session.
 */
export function ComponentsPanel() {
  const plane = usePlaneClaimEdit();
  const id = useId();
  const { secrets } = useContext(SecretRefsReaderContext)(plane.namespace);
  const edit = plane.edit;
  const session = edit?.session;
  if (!edit || !session) return null;
  const after = claimAfter(session);
  const live = session.claim.spec?.management;
  const management = after.spec?.management;
  const runs = (app: "console" | "webModeler") => management?.[app] != null;
  const ran = (app: "console" | "webModeler") => live?.[app] != null;
  const turn = (app: "console" | "webModeler", on: boolean) => {
    if (!after.spec) return;
    for (const e of componentEdits(after.spec, app, on))
      if ("reset" in e) edit.reset(e.steps);
      else edit.set(e.steps, e.value);
  };
  const mail = mailDraftOf(management?.webModeler?.mail ?? undefined);
  // Each edit sets one field, so the fields that the form does not show,
  // such as the keys of the credentials, stay.
  const setMail = (key: keyof MailDraft, value: unknown) => {
    const at = (...names: string[]) => [
      ...MAIL,
      ...names.map((field) => ({ field })),
    ];
    // The picker offers only Secrets with the default keys, so a new
    // Secret drops the keys of the old one.
    const liveRef = live?.webModeler?.mail?.credentialsSecretRef;
    if (key === "credentialsSecret")
      if (value === "") edit.reset(at("credentialsSecretRef"));
      else
        edit.set(
          at("credentialsSecretRef"),
          value === liveRef?.name ? liveRef : { name: value },
        );
    else if (key === "fromName" && value === "") edit.reset(at("fromName"));
    else edit.set(at(key), value);
  };

  return (
    <div className="flex min-w-0 flex-col gap-6">
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <Checkbox
            id={`${id}-console`}
            checked={runs("console")}
            onCheckedChange={(state) => turn("console", state === true)}
          />
          <label htmlFor={`${id}-console`} className="text-sm text-text">
            Run Console
          </label>
        </div>
        {runs("console") !== ran("console") && (
          <p className="text-sm text-muted">
            {runs("console") ? CONSOLE_ON : CONSOLE_OFF}
          </p>
        )}
      </div>
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <Checkbox
            id={`${id}-web-modeler`}
            checked={runs("webModeler")}
            onCheckedChange={(state) => turn("webModeler", state === true)}
          />
          <label htmlFor={`${id}-web-modeler`} className="text-sm text-text">
            Run Web Modeler
          </label>
        </div>
        {ran("webModeler") && !runs("webModeler") && (
          <p className="text-sm text-muted">{WEB_MODELER_OFF}</p>
        )}
        {runs("webModeler") &&
          (secrets ? (
            <MailFields mail={mail} secrets={secrets} onChange={setMail} />
          ) : (
            <p className="text-sm text-muted">
              Reading the Secrets of the namespace.
            </p>
          ))}
        <FieldErrors errors={errorsAt(edit.errors, MAIL)} />
      </div>
    </div>
  );
}
