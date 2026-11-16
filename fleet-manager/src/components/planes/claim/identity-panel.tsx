"use client";

import { useEditorGeneration } from "@/components/cluster/edit/editor-generation";
import { useContext } from "react";
import { errorsAt } from "@/components/cluster/settings-editor";
import { LockedField } from "@/components/fleet/locked-field";
import { SecretRefsReaderContext } from "@/components/fleet/namespaced-secret-ref-picker";
import { SecretRefPicker } from "@/components/fleet/secret-ref-picker";
import {
  ExternalKeycloakFields,
  IdentityCards,
} from "@/components/planes/create/identity-cards";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import { lockLabel, type Step } from "@/lib/fields";
import {
  type ExternalKeycloakDraft,
  externalKeycloakDraftOf,
  identityModeOf,
  sameFamily,
} from "@/lib/planes/identity";
import { FIRST_ADMIN_STAYS, planeModeMove } from "@/lib/planes/impacts";
import { claimAfter } from "@/lib/session";
import { ClaimTextField, FieldErrors } from "./fields";
import { planeFieldsOf, usePlaneClaimEdit } from "./plane-claim-edit";
import { UrlFields } from "./plane-section-panel";

const at = (...names: string[]): Step[] => names.map((field) => ({ field }));
const PROVIDER = at("spec", "management", "identityProvider");
const EXTERNAL = [...PROVIDER, { field: "externalKeycloak" }];
const ADMIN = at("spec", "management", "identity", "admin");

// A picker that starts before the list of Secrets is known would report
// its Secret as gone, and the panel would remove it from the claim.
const READING_SECRETS = "Reading the Secrets of the namespace.";

/**
 * IdentityPanel edits where the users of the plane sign in, and the first
 * administrator of the mode after the change. A pick of a mode moves the
 * claim to it in one change. OIDC cannot be picked while the platform
 * config after the change uses basic authentication. After a move between
 * OIDC and a Keycloak mode, the administrator fields of the new mode show
 * as required. It renders nothing without a session.
 */
export function IdentityPanel() {
  const plane = usePlaneClaimEdit();
  const { secrets } = useContext(SecretRefsReaderContext)(plane.namespace);
  const edit = plane.edit;
  // An editor that keeps its own text starts again after a discard or a
  // write.
  const { generation } = useEditorGeneration(edit?.pending ?? 0);
  const session = edit?.session;
  if (!edit || !session) return null;
  const after = claimAfter(session);
  const management = after.spec?.management;
  const mode = identityModeOf(management?.identityProvider);
  const ref =
    after.spec?.platformConfigRef || plane.environment?.spec.platformConfigRef;
  const config = plane.platformConfigs.find((c) => c.metadata?.name === ref);
  const move = planeModeMove(session);
  const crossed = move !== undefined && !sameFamily(move.from, move.to);
  const fields = planeFieldsOf(after, plane);
  const lockOf = (key: string) => fields.find((f) => f.key === key)?.lock;
  const admin = management?.identity?.admin ?? {};
  const errors = (...names: string[]) =>
    errorsAt(edit.errors, [...ADMIN, ...names.map((field) => ({ field }))]);
  const text = (value: unknown) => (typeof value === "string" ? value : "");
  const setAdmin = (name: string) => (value: string) =>
    value === ""
      ? edit.reset([...ADMIN, { field: name }])
      : edit.set([...ADMIN, { field: name }], value);

  const liveExternal =
    session.claim.spec?.management?.identityProvider?.externalKeycloak;
  const setExternal = (draft: ExternalKeycloakDraft) => {
    const before = externalKeycloakDraftOf(
      management?.identityProvider?.externalKeycloak ?? undefined,
    );
    if (draft.url !== before.url)
      edit.set([...EXTERNAL, { field: "url" }], draft.url);
    if (draft.realm !== before.realm)
      if (draft.realm === "") edit.reset([...EXTERNAL, { field: "realm" }]);
      else edit.set([...EXTERNAL, { field: "realm" }], draft.realm);
    if (draft.adminSecret !== before.adminSecret)
      edit.set(
        [
          ...EXTERNAL,
          { field: "adminCredentialsSecretRef" },
          { field: "name" },
        ],
        draft.adminSecret,
      );
    if (JSON.stringify(draft.caBundle) !== JSON.stringify(before.caBundle))
      if (draft.caBundle)
        edit.set([...EXTERNAL, { field: "caBundleSecretRef" }], draft.caBundle);
      else edit.reset([...EXTERNAL, { field: "caBundleSecretRef" }]);
  };

  return (
    <div className="flex min-w-0 flex-col gap-6">
      <div className="flex flex-col gap-2">
        <IdentityCards
          value={mode}
          platformConfig={
            config as unknown as CamundaPlatformConfig | undefined
          }
          onChange={(to) =>
            plane.moveIdentity(
              to,
              to === "externalKeycloak"
                ? externalKeycloakDraftOf(liveExternal ?? undefined)
                : undefined,
            )
          }
        />
        <FieldErrors
          errors={errorsAt(edit.errors, PROVIDER).filter(
            (e) => e.path === "spec.management.identityProvider",
          )}
        />
      </div>
      {mode === "externalKeycloak" && !secrets && (
        <p className="text-sm text-muted">{READING_SECRETS}</p>
      )}
      {mode === "externalKeycloak" && secrets && (
        <div className="flex flex-col gap-2">
          <ExternalKeycloakFields
            key={generation}
            value={externalKeycloakDraftOf(
              management?.identityProvider?.externalKeycloak ?? undefined,
            )}
            secrets={secrets}
            namespaceIsNew={false}
            onChange={setExternal}
          />
          <FieldErrors errors={errorsAt(edit.errors, EXTERNAL)} />
        </div>
      )}
      {mode === "keycloak" && (
        <UrlFields fields={fields.filter((f) => f.key === "url.keycloak")} />
      )}
      <section className="flex min-w-0 flex-col gap-4">
        <h3 className="font-heading text-base font-semibold">
          First administrator
        </h3>
        {crossed && <p className="text-sm text-muted">{FIRST_ADMIN_STAYS}</p>}
        {mode === "oidc" ? (
          (["claimName", "claimValue"] as const).map((name) => {
            const label = name === "claimName" ? "Claim name" : "Claim value";
            const lock = lockOf(`admin.${name}`);
            return lock ? (
              <LockedField
                key={name}
                label={label}
                value={text(admin[name])}
                reason={lockLabel(lock)}
              />
            ) : (
              <ClaimTextField
                key={name}
                label={label}
                value={text(admin[name])}
                required
                placeholder={name === "claimName" ? "sub" : undefined}
                errors={errors(name)}
                onChange={setAdmin(name)}
              />
            );
          })
        ) : mode !== undefined ? (
          <>
            <ClaimTextField
              label="Username"
              value={text(admin.username)}
              required
              note={
                crossed ? undefined : "A new username creates a second user."
              }
              errors={errors("username")}
              onChange={setAdmin("username")}
            />
            <ClaimTextField
              label="Email"
              value={text(admin.email)}
              errors={errors("email")}
              onChange={setAdmin("email")}
            />
            <div className="flex max-w-md flex-col gap-1">
              <span className="text-sm font-medium text-text">
                Password Secret (optional)
              </span>
              {secrets ? (
                <SecretRefPicker
                  key={generation}
                  label="Password Secret"
                  secrets={secrets}
                  defaultValue={admin.passwordSecretRef ?? undefined}
                  onChange={(secret) =>
                    secret
                      ? edit.set(
                          [...ADMIN, { field: "passwordSecretRef" }],
                          secret,
                        )
                      : edit.reset([...ADMIN, { field: "passwordSecretRef" }])
                  }
                />
              ) : (
                <p className="text-sm text-muted">{READING_SECRETS}</p>
              )}
              <FieldErrors errors={errors("passwordSecretRef")} />
            </div>
          </>
        ) : null}
      </section>
    </div>
  );
}
