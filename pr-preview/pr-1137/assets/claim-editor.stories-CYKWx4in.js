import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{a as r,t as i}from"./hooks-BZtkPEQF.js";import{a,b as o}from"./labels-CoLYcJ9C.js";import{n as s,t as c}from"./createLucideIcon-B55dJ-eE.js";import{l}from"./iframe-CE3Rt7mq.js";import{S as u}from"./release-CzPobo-q.js";import{_ as d,t as f}from"./merge-CyPKz5z-.js";import{t as p,u as m}from"./policy-mKx7yP_H.js";import{a as h,o as g}from"./draft-Cua5vuJR.js";import{v as ee}from"./schema-C3XnYfaK.js";import{t as _}from"./fields-m3rennQl.js";import{n as v}from"./labels-DYL500th.js";import{n as te,t as ne}from"./attachment-fields-DDNkg9Ae.js";import{K as y,q as b,t as x}from"./session-BHIfl8Uq.js";import{n as S,t as C}from"./locked-field-BEGQxrUK.js";import{n as re,t as ie}from"./checkbox-ZYSSjKOs.js";import{n as w,r as ae,t as oe}from"./native-select-C1RzJW3T.js";import{c as se,l as ce}from"./settings-editor-QZFsPhEO.js";import{r as le,t as T}from"./button-Dn1nGbY0.js";import{n as ue,r as de}from"./secret-ref-picker-BpYRCEO2.js";import{i as fe,r as pe}from"./impacts-Cdq5zNiK.js";import{a as me,n as he,o as ge,s as _e,t as ve}from"./claim-tab-BFIUunmV.js";import{a as ye,n as be}from"./deletion-choice-Ce0VdU57.js";import{a as xe,i as Se,n as E,o as Ce,r as D,t as we}from"./cluster-edit-_871O4vs.js";import{i as Te,n as Ee,r as De}from"./release-options-Ir4ExF8l.js";import{n as Oe,t as ke}from"./selection-settings-editor-v3veHzn7.js";var O,Ae;function je(){return(je=e((()=>{s(),O={name:`pencil`,size:24,node:[[`path`,{d:`M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z`,key:`1a8usu`}],[`path`,{d:`m15 5 4 4`,key:`1mk7zo`}]]},O.node,Ae=c(O)})))()}function Me(){return(Me=e((()=>{})))()}function Ne({context:e}){let t=D(),{session:n}=t;return n?(0,Pe.jsx)(ne,{value:y(n).spec?.attachments,set:t.set,reset:t.reset,invalid:t.invalid,context:{secondaryStorage:e.storage?.spec.type,backupStorage:e.backupStorage.kind!==`none`,managementAuthConfigs:e.managementAuthConfigs.map(e=>e.metadata?.name??``).sort(u),lock:e=>b(n,e),errors:t.errors,pending:t.pending,lockedUrl:e.optimize?.spec.externalUrl,optimizeHref:e.optimize?`?tab=overview&component=optimize`:void 0}}):null}var Pe;function Fe(){return(Fe=e((()=>{Pe=n(),p(),x(),te(),E(),Ne.__docgenInfo={description:`AttachmentsPanel edits the attachments of the claim through
AttachmentFields, with the claim after the pending changes of the session
and the rules of the cluster in context. A locked external URL with no
claim value shows the URL of the running Optimize. It renders nothing
without a session.`,methods:[],displayName:`AttachmentsPanel`,props:{context:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  cluster: Resource<"CamundaCluster">;
  claim?: Resource<"CamundaClusterClaim">;
  merged: Merged<CamundaClusterSpec>;
  environment?: Resource<"Environment">;
  environments: readonly Resource<"Environment">[];
  presets: readonly Resource<"CamundaClusterPreset">[];
  releases: readonly Resource<"CamundaRelease">[];
  platformConfigs: readonly Resource<"CamundaPlatformConfig">[];
  managementAuthConfigs: readonly Resource<"ManagementAuthConfig">[];
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  requests: readonly Resource<"PluginRequest">[];
  storage?: Resource<"SecondaryStorageConfig">;
  backups: readonly BackupObject[];
  restores: readonly RestoreObject[];
  backupStorage: BackupStorage;
  schedules: readonly Resource<"BackupSchedule">[];
  optimize?: Resource<"CamundaOptimize">;
  fields: readonly ClusterField[];
  writeMode?: WriteMode;
  openId?: CardId;
  now: string;
}`,signature:{properties:[{key:`cluster`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaCluster"`}],raw:`Resource<"CamundaCluster">`,required:!0}},{key:`claim`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaClusterClaim"`}],raw:`Resource<"CamundaClusterClaim">`,required:!1}},{key:`merged`,value:{name:`Merged`,elements:[{name:`CamundaClusterSpec`}],raw:`Merged<CamundaClusterSpec>`,required:!0}},{key:`environment`,value:{name:`Resource`,elements:[{name:`literal`,value:`"Environment"`}],raw:`Resource<"Environment">`,required:!1}},{key:`environments`,value:{name:`unknown`,required:!0}},{key:`presets`,value:{name:`unknown`,required:!0}},{key:`releases`,value:{name:`unknown`,required:!0}},{key:`platformConfigs`,value:{name:`unknown`,required:!0}},{key:`managementAuthConfigs`,value:{name:`unknown`,required:!0}},{key:`sections`,value:{name:`unknown`,required:!0}},{key:`registrations`,value:{name:`unknown`,required:!0}},{key:`requests`,value:{name:`unknown`,required:!0}},{key:`storage`,value:{name:`Resource`,elements:[{name:`literal`,value:`"SecondaryStorageConfig"`}],raw:`Resource<"SecondaryStorageConfig">`,required:!1}},{key:`backups`,value:{name:`unknown`,required:!0}},{key:`restores`,value:{name:`unknown`,required:!0}},{key:`backupStorage`,value:{name:`union`,raw:`| { kind: "none"; schedules: ScheduleView[] }
| {
    kind: "elasticsearch" | "rdbms" | "unknown";
    missing?: string;
    bucket: BucketView;
    repository?: string;
    database?: DatabaseView;
    schedules: ScheduleView[];
  }`,elements:[{name:`signature`,type:`object`,raw:`{ kind: "none"; schedules: ScheduleView[] }`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"none"`,required:!0}},{key:`schedules`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  cron: string;
  completed: number;
  failed: number;
  lastBackup?: string;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{
  kind: "elasticsearch" | "rdbms" | "unknown";
  missing?: string;
  bucket: BucketView;
  repository?: string;
  database?: DatabaseView;
  schedules: ScheduleView[];
}`,signature:{properties:[{key:`kind`,value:{name:`union`,raw:`"elasticsearch" | "rdbms" | "unknown"`,elements:[{name:`literal`,value:`"elasticsearch"`},{name:`literal`,value:`"rdbms"`},{name:`literal`,value:`"unknown"`}],required:!0}},{key:`missing`,value:{name:`string`,required:!1}},{key:`bucket`,value:{name:`signature`,type:`object`,raw:`{ name: string; location?: string }`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`location`,value:{name:`string`,required:!1}}]},required:!1}},{key:`repository`,value:{name:`string`,required:!1}},{key:`database`,value:{name:`signature`,type:`object`,raw:`{
  missing?: string;
  serverConfig?: string;
  endpoint?: string;
  pitr?: { enabled: boolean; recovery?: string; retentionDays?: number };
  archive?: ArchiveView;
}`,signature:{properties:[{key:`missing`,value:{name:`string`,required:!1}},{key:`serverConfig`,value:{name:`string`,required:!1}},{key:`endpoint`,value:{name:`string`,required:!1}},{key:`pitr`,value:{name:`signature`,type:`object`,raw:`{ enabled: boolean; recovery?: string; retentionDays?: number }`,signature:{properties:[{key:`enabled`,value:{name:`boolean`,required:!0}},{key:`recovery`,value:{name:`string`,required:!1}},{key:`retentionDays`,value:{name:`number`,required:!1}}]},required:!1}},{key:`archive`,value:{name:`signature`,type:`object`,raw:`{
  server: string;
  state: { tone: StatusTone; label: string; message?: string };
  bucket?: BucketView;
  window?: RestoreInterval[];
}`,signature:{properties:[{key:`server`,value:{name:`string`,required:!0}},{key:`state`,value:{name:`signature`,type:`object`,raw:`{ tone: StatusTone; label: string; message?: string }`,signature:{properties:[{key:`tone`,value:{name:`StatusTone`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`message`,value:{name:`string`,required:!1}}]},required:!0}},{key:`bucket`,value:{name:`signature`,type:`object`,raw:`{ name: string; location?: string }`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`location`,value:{name:`string`,required:!1}}]},required:!1}},{key:`window`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ from: string; to?: string; failing?: true }`,signature:{properties:[{key:`from`,value:{name:`string`,required:!0}},{key:`to`,value:{name:`string`,required:!1}},{key:`failing`,value:{name:`literal`,value:`true`,required:!1}}]}}],raw:`RestoreInterval[]`,required:!1}}]},required:!1}}]},required:!1}},{key:`schedules`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  cron: string;
  completed: number;
  failed: number;
  lastBackup?: string;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`ProcessCardId | SectionComponentId`,elements:[{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}]},{name:`SectionComponentId`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``}}}})))()}function Ie({view:e}){let{session:t,set:n,reset:r}=D(),i=(0,Le.useId)();if(!t)return null;let a=v({reason:`atCreation`}),o=b(t,A),s=e=>{!e&&t.claim.spec?.suspend===void 0?r(A):n(A,e)};return(0,k.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[(0,k.jsxs)(`div`,{className:`grid gap-4 sm:grid-cols-3`,children:[(0,k.jsx)(C,{label:`Name`,value:e.name,reason:a}),(0,k.jsx)(C,{label:`Namespace`,value:e.namespace,reason:a}),(0,k.jsx)(C,{label:`Environment`,value:e.environment??`No environment`,reason:a})]}),o?(0,k.jsx)(C,{label:`Suspend the cluster`,value:e.suspend?`Yes`:`No`,reason:v(o)}):(0,k.jsxs)(`div`,{className:`flex items-start gap-3`,children:[(0,k.jsx)(ie,{id:i,className:`mt-0.5`,checked:e.suspend,"aria-describedby":`${i}-hint`,onCheckedChange:e=>s(e===!0)}),(0,k.jsxs)(`div`,{className:`flex flex-col gap-0.5`,children:[(0,k.jsx)(`label`,{htmlFor:i,className:`text-sm font-medium text-text`,children:`Suspend the cluster`}),(0,k.jsx)(`p`,{id:`${i}-hint`,className:`text-xs text-muted`,children:`Every workload goes to zero. The volumes stay.`})]})]})]})}var k,Le,A;function Re(){return(Re=e((()=>{k=n(),Le=t(),S(),re(),_(),x(),E(),A=[{field:`spec`},{field:`suspend`}],Ie.__docgenInfo={description:`BasicsPanel edits the Basics of the claim: the name, the namespace, and
the environment are locked as set at creation, and Suspend is a toggle.
view is the claim after the pending changes. It renders nothing without
a session.`,methods:[],displayName:`BasicsPanel`,props:{view:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  name: string;
  namespace: string;
  environment?: string;
  suspend: boolean;
  preset?: string;
  release: ClaimRef;
  platformConfig: ClaimRef;
  deletionPolicy: ClaimRef;
  sections: ClaimSection[];
  optimize?: { externalUrl?: string; managementAuthConfigRef: string };
  backupSchedule?: { schedule: string; completed?: number; failed?: number };
  conditions: ConditionRow[];
  requests: ClaimRequestRow[];
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`namespace`,value:{name:`string`,required:!0}},{key:`environment`,value:{name:`string`,required:!1}},{key:`suspend`,value:{name:`boolean`,required:!0}},{key:`preset`,value:{name:`string`,required:!1}},{key:`release`,value:{name:`signature`,type:`object`,raw:`{ value?: string; fromEnvironment: boolean }`,signature:{properties:[{key:`value`,value:{name:`string`,required:!1}},{key:`fromEnvironment`,value:{name:`boolean`,required:!0}}]},required:!0}},{key:`platformConfig`,value:{name:`signature`,type:`object`,raw:`{ value?: string; fromEnvironment: boolean }`,signature:{properties:[{key:`value`,value:{name:`string`,required:!1}},{key:`fromEnvironment`,value:{name:`boolean`,required:!0}}]},required:!0}},{key:`deletionPolicy`,value:{name:`signature`,type:`object`,raw:`{ value?: string; fromEnvironment: boolean }`,signature:{properties:[{key:`value`,value:{name:`string`,required:!1}},{key:`fromEnvironment`,value:{name:`boolean`,required:!0}}]},required:!0}},{key:`sections`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  id: SectionComponentId;
  section: string;
  sectionLabel: string;
  registration: string;
  optionLabel: string;
  parameters: ClusterField[];
}`,signature:{properties:[{key:`id`,value:{name:`SectionComponentId`,required:!0}},{key:`section`,value:{name:`string`,required:!0}},{key:`sectionLabel`,value:{name:`string`,required:!0}},{key:`registration`,value:{name:`string`,required:!0}},{key:`optionLabel`,value:{name:`string`,required:!0}},{key:`parameters`,value:{name:`Array`,elements:[{name:`ClusterField`}],raw:`ClusterField[]`,required:!0}}]}}],raw:`ClaimSection[]`,required:!0}},{key:`optimize`,value:{name:`signature`,type:`object`,raw:`{ externalUrl?: string; managementAuthConfigRef: string }`,signature:{properties:[{key:`externalUrl`,value:{name:`string`,required:!1}},{key:`managementAuthConfigRef`,value:{name:`string`,required:!0}}]},required:!1}},{key:`backupSchedule`,value:{name:`signature`,type:`object`,raw:`{ schedule: string; completed?: number; failed?: number }`,signature:{properties:[{key:`schedule`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!1}},{key:`failed`,value:{name:`number`,required:!1}}]},required:!1}},{key:`conditions`,value:{name:`Array`,elements:[{name:`ConditionRow`}],raw:`ConditionRow[]`,required:!0}},{key:`requests`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  sectionLabel: string;
  optionLabel: string;
  outputsReady?: string;
  ready?: string;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`sectionLabel`,value:{name:`string`,required:!0}},{key:`optionLabel`,value:{name:`string`,required:!0}},{key:`outputsReady`,value:{name:`string`,required:!1}},{key:`ready`,value:{name:`string`,required:!1}}]}}],raw:`ClaimRequestRow[]`,required:!0}}]}},description:``}}}})))()}function ze({context:e}){let t=D();if(!t.session)return null;let n=y(t.session),r=m({claim:{...n,spec:{...n.spec,plugins:n.spec?.plugins}},environment:e.environment,sections:e.sections,registrations:e.registrations,kind:`CamundaClusterClaim`}),i=r.policy===`Orphan`&&!r.keepAllowed,a=t.session.claim.spec?.deletionPolicy;return(0,j.jsxs)(`div`,{className:`flex flex-col gap-3`,children:[(0,j.jsx)(be,{subject:`cluster`,choice:r,onChange:e=>e===r.defaultPolicy&&!a?t.reset(M):t.set(M,e)}),i&&(0,j.jsx)(`p`,{className:`text-sm text-warn`,children:`The claim keeps its resources, but an option below cannot keep them. Choose Delete, or remove that option.`})]})}var j,M;function Be(){return(Be=e((()=>{j=n(),ye(),p(),x(),E(),M=[{field:`spec`},{field:`deletionPolicy`}],ze.__docgenInfo={description:`DeletionPanel edits the deletion policy of the claim. Choosing the
default while the claim sets no policy removes the change. While the
claim keeps its resources and an option cannot, the panel shows Keep as
chosen. It renders nothing without a session.`,methods:[],displayName:`DeletionPanel`,props:{context:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  cluster: Resource<"CamundaCluster">;
  claim?: Resource<"CamundaClusterClaim">;
  merged: Merged<CamundaClusterSpec>;
  environment?: Resource<"Environment">;
  environments: readonly Resource<"Environment">[];
  presets: readonly Resource<"CamundaClusterPreset">[];
  releases: readonly Resource<"CamundaRelease">[];
  platformConfigs: readonly Resource<"CamundaPlatformConfig">[];
  managementAuthConfigs: readonly Resource<"ManagementAuthConfig">[];
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  requests: readonly Resource<"PluginRequest">[];
  storage?: Resource<"SecondaryStorageConfig">;
  backups: readonly BackupObject[];
  restores: readonly RestoreObject[];
  backupStorage: BackupStorage;
  schedules: readonly Resource<"BackupSchedule">[];
  optimize?: Resource<"CamundaOptimize">;
  fields: readonly ClusterField[];
  writeMode?: WriteMode;
  openId?: CardId;
  now: string;
}`,signature:{properties:[{key:`cluster`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaCluster"`}],raw:`Resource<"CamundaCluster">`,required:!0}},{key:`claim`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaClusterClaim"`}],raw:`Resource<"CamundaClusterClaim">`,required:!1}},{key:`merged`,value:{name:`Merged`,elements:[{name:`CamundaClusterSpec`}],raw:`Merged<CamundaClusterSpec>`,required:!0}},{key:`environment`,value:{name:`Resource`,elements:[{name:`literal`,value:`"Environment"`}],raw:`Resource<"Environment">`,required:!1}},{key:`environments`,value:{name:`unknown`,required:!0}},{key:`presets`,value:{name:`unknown`,required:!0}},{key:`releases`,value:{name:`unknown`,required:!0}},{key:`platformConfigs`,value:{name:`unknown`,required:!0}},{key:`managementAuthConfigs`,value:{name:`unknown`,required:!0}},{key:`sections`,value:{name:`unknown`,required:!0}},{key:`registrations`,value:{name:`unknown`,required:!0}},{key:`requests`,value:{name:`unknown`,required:!0}},{key:`storage`,value:{name:`Resource`,elements:[{name:`literal`,value:`"SecondaryStorageConfig"`}],raw:`Resource<"SecondaryStorageConfig">`,required:!1}},{key:`backups`,value:{name:`unknown`,required:!0}},{key:`restores`,value:{name:`unknown`,required:!0}},{key:`backupStorage`,value:{name:`union`,raw:`| { kind: "none"; schedules: ScheduleView[] }
| {
    kind: "elasticsearch" | "rdbms" | "unknown";
    missing?: string;
    bucket: BucketView;
    repository?: string;
    database?: DatabaseView;
    schedules: ScheduleView[];
  }`,elements:[{name:`signature`,type:`object`,raw:`{ kind: "none"; schedules: ScheduleView[] }`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"none"`,required:!0}},{key:`schedules`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  cron: string;
  completed: number;
  failed: number;
  lastBackup?: string;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{
  kind: "elasticsearch" | "rdbms" | "unknown";
  missing?: string;
  bucket: BucketView;
  repository?: string;
  database?: DatabaseView;
  schedules: ScheduleView[];
}`,signature:{properties:[{key:`kind`,value:{name:`union`,raw:`"elasticsearch" | "rdbms" | "unknown"`,elements:[{name:`literal`,value:`"elasticsearch"`},{name:`literal`,value:`"rdbms"`},{name:`literal`,value:`"unknown"`}],required:!0}},{key:`missing`,value:{name:`string`,required:!1}},{key:`bucket`,value:{name:`signature`,type:`object`,raw:`{ name: string; location?: string }`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`location`,value:{name:`string`,required:!1}}]},required:!1}},{key:`repository`,value:{name:`string`,required:!1}},{key:`database`,value:{name:`signature`,type:`object`,raw:`{
  missing?: string;
  serverConfig?: string;
  endpoint?: string;
  pitr?: { enabled: boolean; recovery?: string; retentionDays?: number };
  archive?: ArchiveView;
}`,signature:{properties:[{key:`missing`,value:{name:`string`,required:!1}},{key:`serverConfig`,value:{name:`string`,required:!1}},{key:`endpoint`,value:{name:`string`,required:!1}},{key:`pitr`,value:{name:`signature`,type:`object`,raw:`{ enabled: boolean; recovery?: string; retentionDays?: number }`,signature:{properties:[{key:`enabled`,value:{name:`boolean`,required:!0}},{key:`recovery`,value:{name:`string`,required:!1}},{key:`retentionDays`,value:{name:`number`,required:!1}}]},required:!1}},{key:`archive`,value:{name:`signature`,type:`object`,raw:`{
  server: string;
  state: { tone: StatusTone; label: string; message?: string };
  bucket?: BucketView;
  window?: RestoreInterval[];
}`,signature:{properties:[{key:`server`,value:{name:`string`,required:!0}},{key:`state`,value:{name:`signature`,type:`object`,raw:`{ tone: StatusTone; label: string; message?: string }`,signature:{properties:[{key:`tone`,value:{name:`StatusTone`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`message`,value:{name:`string`,required:!1}}]},required:!0}},{key:`bucket`,value:{name:`signature`,type:`object`,raw:`{ name: string; location?: string }`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`location`,value:{name:`string`,required:!1}}]},required:!1}},{key:`window`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ from: string; to?: string; failing?: true }`,signature:{properties:[{key:`from`,value:{name:`string`,required:!0}},{key:`to`,value:{name:`string`,required:!1}},{key:`failing`,value:{name:`literal`,value:`true`,required:!1}}]}}],raw:`RestoreInterval[]`,required:!1}}]},required:!1}}]},required:!1}},{key:`schedules`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  cron: string;
  completed: number;
  failed: number;
  lastBackup?: string;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`ProcessCardId | SectionComponentId`,elements:[{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}]},{name:`SectionComponentId`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``}}}})))()}function Ve({context:e}){let t=D(),{session:n}=t;if(!n)return null;let r=n.claim.spec,i=y(n).spec??{},a=e=>{let n=ee(``,e);return t.errors.filter(e=>e.path===n).map(e=>e.message)},o=e=>b(n,e),s=(e,n,i)=>!Ue(r,e)&&(n===``||n===i)||n===``?t.reset(e):t.set(e,n),{options:c,note:l}=De({cluster:e.cluster,claim:n.claim,releases:e.releases,environment:e.environment,backups:e.backups}),u=i.releaseRef||e.cluster.spec.releaseRef||``,d=c.find(e=>e.name===u),f=n=>{s(F,n,e.cluster.spec.releaseRef);let i=c.find(e=>e.name===n)?.downgrade?.to;i?t.set(R,i):r?.allowVersionDowngrade===void 0?t.reset(R):t.set(R,r.allowVersionDowngrade)},p=He(e,r?.presetRef),m=i.presetRef??``,h=e.environment?.spec.platformConfigRef,g=i.platformConfigRef??``,_=t.pending===0&&Se(e.cluster,n.claim);return(0,P.jsxs)(`div`,{className:`flex flex-col gap-5`,children:[(0,P.jsx)(N,{label:`Release`,value:u,lock:o(F),errors:a(F),onChange:f,hint:(0,P.jsxs)(P.Fragment,{children:[d?.downgrade&&(0,P.jsxs)(`p`,{className:`text-xs text-warn`,children:[d.downgrade.from,` to `,d.downgrade.to,` is a downgrade. The review asks you to confirm it.`]}),l&&(0,P.jsx)(`p`,{className:`text-xs text-muted`,children:l})]}),children:c.map(e=>(0,P.jsx)(w,{value:e.name,disabled:e.disabled!==void 0&&!e.isCurrent,children:Te(e,e.isDefault)},e.name))}),_&&(0,P.jsxs)(`div`,{className:`flex flex-wrap items-center gap-x-3 gap-y-2 rounded-md border border-border bg-panel-2 px-3 py-2 text-sm text-text-2`,children:[(0,P.jsxs)(`p`,{className:`min-w-0 flex-1`,children:[`The cluster runs `,e.cluster.status?.management?.version,`, so allowVersionDowngrade can go.`]}),(0,P.jsx)(T,{size:`sm`,variant:`outline`,onClick:()=>{t.reset(R),t.review()},children:`Clear`})]}),(0,P.jsx)(N,{label:`Preset`,value:m,lock:o(I),errors:a(I),onChange:e=>s(I,e),children:p.map(e=>(0,P.jsx)(w,{value:e,children:e},e))}),(0,P.jsxs)(N,{label:`Platform config`,value:g,lock:o(L),errors:a(L),onChange:e=>s(L,e),children:[(0,P.jsx)(w,{value:``,children:h?`Default of the environment`:`Not set`}),e.platformConfigs.map(e=>(0,P.jsx)(w,{value:e.metadata?.name,children:e.metadata?.name},e.metadata?.name))]})]})}function He(e,t){let n=g(e.presets,e.environment).map(e=>e.metadata.name??``);return t&&!n.includes(t)?[...n,t]:n}function Ue(e,t){let n=t[1];return e&&`field`in n?e[n.field]:void 0}function N({label:e,value:t,lock:n,errors:r,hint:i,onChange:a,children:o}){let s=(0,We.useId)();return n?(0,P.jsx)(C,{label:e,value:t||`Not set`,reason:v(n)}):(0,P.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1.5`,children:[(0,P.jsx)(`label`,{htmlFor:s,className:`text-sm font-medium text-text`,children:e}),(0,P.jsx)(oe,{id:s,value:t,"aria-invalid":r.length>0||void 0,"aria-describedby":r.length>0?`${s}-error`:void 0,onChange:e=>a(e.target.value),children:o}),i,r.length>0&&(0,P.jsx)(`p`,{id:`${s}-error`,className:`text-xs text-bad`,children:r.join(` `)})]})}var P,We,F,I,L,R;function Ge(){return(Ge=e((()=>{P=n(),We=t(),S(),le(),ae(),h(),_(),x(),E(),xe(),Ee(),F=[{field:`spec`},{field:`releaseRef`}],I=[{field:`spec`},{field:`presetRef`}],L=[{field:`spec`},{field:`platformConfigRef`}],R=[{field:`spec`},{field:`allowVersionDowngrade`}],Ve.__docgenInfo={description:`ReleasePanel edits the release, the preset, and the platform config of
the claim. Once downgradeDone and nothing is pending, Clear removes
allowVersionDowngrade and opens the review. It renders nothing without a
session.`,methods:[],displayName:`ReleasePanel`,props:{context:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  cluster: Resource<"CamundaCluster">;
  claim?: Resource<"CamundaClusterClaim">;
  merged: Merged<CamundaClusterSpec>;
  environment?: Resource<"Environment">;
  environments: readonly Resource<"Environment">[];
  presets: readonly Resource<"CamundaClusterPreset">[];
  releases: readonly Resource<"CamundaRelease">[];
  platformConfigs: readonly Resource<"CamundaPlatformConfig">[];
  managementAuthConfigs: readonly Resource<"ManagementAuthConfig">[];
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  requests: readonly Resource<"PluginRequest">[];
  storage?: Resource<"SecondaryStorageConfig">;
  backups: readonly BackupObject[];
  restores: readonly RestoreObject[];
  backupStorage: BackupStorage;
  schedules: readonly Resource<"BackupSchedule">[];
  optimize?: Resource<"CamundaOptimize">;
  fields: readonly ClusterField[];
  writeMode?: WriteMode;
  openId?: CardId;
  now: string;
}`,signature:{properties:[{key:`cluster`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaCluster"`}],raw:`Resource<"CamundaCluster">`,required:!0}},{key:`claim`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaClusterClaim"`}],raw:`Resource<"CamundaClusterClaim">`,required:!1}},{key:`merged`,value:{name:`Merged`,elements:[{name:`CamundaClusterSpec`}],raw:`Merged<CamundaClusterSpec>`,required:!0}},{key:`environment`,value:{name:`Resource`,elements:[{name:`literal`,value:`"Environment"`}],raw:`Resource<"Environment">`,required:!1}},{key:`environments`,value:{name:`unknown`,required:!0}},{key:`presets`,value:{name:`unknown`,required:!0}},{key:`releases`,value:{name:`unknown`,required:!0}},{key:`platformConfigs`,value:{name:`unknown`,required:!0}},{key:`managementAuthConfigs`,value:{name:`unknown`,required:!0}},{key:`sections`,value:{name:`unknown`,required:!0}},{key:`registrations`,value:{name:`unknown`,required:!0}},{key:`requests`,value:{name:`unknown`,required:!0}},{key:`storage`,value:{name:`Resource`,elements:[{name:`literal`,value:`"SecondaryStorageConfig"`}],raw:`Resource<"SecondaryStorageConfig">`,required:!1}},{key:`backups`,value:{name:`unknown`,required:!0}},{key:`restores`,value:{name:`unknown`,required:!0}},{key:`backupStorage`,value:{name:`union`,raw:`| { kind: "none"; schedules: ScheduleView[] }
| {
    kind: "elasticsearch" | "rdbms" | "unknown";
    missing?: string;
    bucket: BucketView;
    repository?: string;
    database?: DatabaseView;
    schedules: ScheduleView[];
  }`,elements:[{name:`signature`,type:`object`,raw:`{ kind: "none"; schedules: ScheduleView[] }`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"none"`,required:!0}},{key:`schedules`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  cron: string;
  completed: number;
  failed: number;
  lastBackup?: string;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{
  kind: "elasticsearch" | "rdbms" | "unknown";
  missing?: string;
  bucket: BucketView;
  repository?: string;
  database?: DatabaseView;
  schedules: ScheduleView[];
}`,signature:{properties:[{key:`kind`,value:{name:`union`,raw:`"elasticsearch" | "rdbms" | "unknown"`,elements:[{name:`literal`,value:`"elasticsearch"`},{name:`literal`,value:`"rdbms"`},{name:`literal`,value:`"unknown"`}],required:!0}},{key:`missing`,value:{name:`string`,required:!1}},{key:`bucket`,value:{name:`signature`,type:`object`,raw:`{ name: string; location?: string }`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`location`,value:{name:`string`,required:!1}}]},required:!1}},{key:`repository`,value:{name:`string`,required:!1}},{key:`database`,value:{name:`signature`,type:`object`,raw:`{
  missing?: string;
  serverConfig?: string;
  endpoint?: string;
  pitr?: { enabled: boolean; recovery?: string; retentionDays?: number };
  archive?: ArchiveView;
}`,signature:{properties:[{key:`missing`,value:{name:`string`,required:!1}},{key:`serverConfig`,value:{name:`string`,required:!1}},{key:`endpoint`,value:{name:`string`,required:!1}},{key:`pitr`,value:{name:`signature`,type:`object`,raw:`{ enabled: boolean; recovery?: string; retentionDays?: number }`,signature:{properties:[{key:`enabled`,value:{name:`boolean`,required:!0}},{key:`recovery`,value:{name:`string`,required:!1}},{key:`retentionDays`,value:{name:`number`,required:!1}}]},required:!1}},{key:`archive`,value:{name:`signature`,type:`object`,raw:`{
  server: string;
  state: { tone: StatusTone; label: string; message?: string };
  bucket?: BucketView;
  window?: RestoreInterval[];
}`,signature:{properties:[{key:`server`,value:{name:`string`,required:!0}},{key:`state`,value:{name:`signature`,type:`object`,raw:`{ tone: StatusTone; label: string; message?: string }`,signature:{properties:[{key:`tone`,value:{name:`StatusTone`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`message`,value:{name:`string`,required:!1}}]},required:!0}},{key:`bucket`,value:{name:`signature`,type:`object`,raw:`{ name: string; location?: string }`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`location`,value:{name:`string`,required:!1}}]},required:!1}},{key:`window`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ from: string; to?: string; failing?: true }`,signature:{properties:[{key:`from`,value:{name:`string`,required:!0}},{key:`to`,value:{name:`string`,required:!1}},{key:`failing`,value:{name:`literal`,value:`true`,required:!1}}]}}],raw:`RestoreInterval[]`,required:!1}}]},required:!1}}]},required:!1}},{key:`schedules`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  cron: string;
  completed: number;
  failed: number;
  lastBackup?: string;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`ProcessCardId | SectionComponentId`,elements:[{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}]},{name:`SectionComponentId`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``}}}})))()}function Ke({context:e,section:t}){let{session:n,pending:i,set:a,reset:o,invalid:s,errors:c}=D(),{generation:l,own:u}=ce(i),d=e.cluster.metadata?.namespace??``,f=de(r(`Secret`,{namespace:d},[]).items);if(!n)return null;let p=e.registrations.find(e=>e.metadata?.name===t.registration),m=y(n),h=m.status?.resolvedPolicy?.parameters?.[t.registration]??{},g=(m.spec?.plugins??[]).find(e=>e.section===t.section&&e.registration===t.registration),ee=Object.fromEntries(Object.entries(h).map(([e,t])=>[e,t.allowed])),_=t.parameters.some(e=>!e.lock&&e.claimSteps);return(0,z.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[(0,z.jsx)(C,{label:`Option`,value:t.optionLabel,reason:v({reason:`chosenAtCreation`})}),_?(0,z.jsx)(ke,{fields:t.parameters,schema:p?.spec.parameters,allowed:ee,claimed:qe(g?.parameters),secrets:f,errors:c,set:u(a),reset:u(o),invalid:s},l):(0,z.jsx)(`p`,{className:`text-sm text-muted`,children:`No parameters you can change.`})]})}function qe(e){return typeof e==`object`&&e&&!Array.isArray(e)?e:{}}var z;function Je(){return(Je=e((()=>{z=n(),S(),ue(),_(),i(),x(),Oe(),E(),se(),Ke.__docgenInfo={description:`SectionPanel edits one selection of the claim: its option is locked as
chosen at creation, and the parameters that the claim may set have
editors. section comes from the claim after the pending changes. It
renders nothing without a session.`,methods:[],displayName:`SectionPanel`,props:{context:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  cluster: Resource<"CamundaCluster">;
  claim?: Resource<"CamundaClusterClaim">;
  merged: Merged<CamundaClusterSpec>;
  environment?: Resource<"Environment">;
  environments: readonly Resource<"Environment">[];
  presets: readonly Resource<"CamundaClusterPreset">[];
  releases: readonly Resource<"CamundaRelease">[];
  platformConfigs: readonly Resource<"CamundaPlatformConfig">[];
  managementAuthConfigs: readonly Resource<"ManagementAuthConfig">[];
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  requests: readonly Resource<"PluginRequest">[];
  storage?: Resource<"SecondaryStorageConfig">;
  backups: readonly BackupObject[];
  restores: readonly RestoreObject[];
  backupStorage: BackupStorage;
  schedules: readonly Resource<"BackupSchedule">[];
  optimize?: Resource<"CamundaOptimize">;
  fields: readonly ClusterField[];
  writeMode?: WriteMode;
  openId?: CardId;
  now: string;
}`,signature:{properties:[{key:`cluster`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaCluster"`}],raw:`Resource<"CamundaCluster">`,required:!0}},{key:`claim`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaClusterClaim"`}],raw:`Resource<"CamundaClusterClaim">`,required:!1}},{key:`merged`,value:{name:`Merged`,elements:[{name:`CamundaClusterSpec`}],raw:`Merged<CamundaClusterSpec>`,required:!0}},{key:`environment`,value:{name:`Resource`,elements:[{name:`literal`,value:`"Environment"`}],raw:`Resource<"Environment">`,required:!1}},{key:`environments`,value:{name:`unknown`,required:!0}},{key:`presets`,value:{name:`unknown`,required:!0}},{key:`releases`,value:{name:`unknown`,required:!0}},{key:`platformConfigs`,value:{name:`unknown`,required:!0}},{key:`managementAuthConfigs`,value:{name:`unknown`,required:!0}},{key:`sections`,value:{name:`unknown`,required:!0}},{key:`registrations`,value:{name:`unknown`,required:!0}},{key:`requests`,value:{name:`unknown`,required:!0}},{key:`storage`,value:{name:`Resource`,elements:[{name:`literal`,value:`"SecondaryStorageConfig"`}],raw:`Resource<"SecondaryStorageConfig">`,required:!1}},{key:`backups`,value:{name:`unknown`,required:!0}},{key:`restores`,value:{name:`unknown`,required:!0}},{key:`backupStorage`,value:{name:`union`,raw:`| { kind: "none"; schedules: ScheduleView[] }
| {
    kind: "elasticsearch" | "rdbms" | "unknown";
    missing?: string;
    bucket: BucketView;
    repository?: string;
    database?: DatabaseView;
    schedules: ScheduleView[];
  }`,elements:[{name:`signature`,type:`object`,raw:`{ kind: "none"; schedules: ScheduleView[] }`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"none"`,required:!0}},{key:`schedules`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  cron: string;
  completed: number;
  failed: number;
  lastBackup?: string;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{
  kind: "elasticsearch" | "rdbms" | "unknown";
  missing?: string;
  bucket: BucketView;
  repository?: string;
  database?: DatabaseView;
  schedules: ScheduleView[];
}`,signature:{properties:[{key:`kind`,value:{name:`union`,raw:`"elasticsearch" | "rdbms" | "unknown"`,elements:[{name:`literal`,value:`"elasticsearch"`},{name:`literal`,value:`"rdbms"`},{name:`literal`,value:`"unknown"`}],required:!0}},{key:`missing`,value:{name:`string`,required:!1}},{key:`bucket`,value:{name:`signature`,type:`object`,raw:`{ name: string; location?: string }`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`location`,value:{name:`string`,required:!1}}]},required:!1}},{key:`repository`,value:{name:`string`,required:!1}},{key:`database`,value:{name:`signature`,type:`object`,raw:`{
  missing?: string;
  serverConfig?: string;
  endpoint?: string;
  pitr?: { enabled: boolean; recovery?: string; retentionDays?: number };
  archive?: ArchiveView;
}`,signature:{properties:[{key:`missing`,value:{name:`string`,required:!1}},{key:`serverConfig`,value:{name:`string`,required:!1}},{key:`endpoint`,value:{name:`string`,required:!1}},{key:`pitr`,value:{name:`signature`,type:`object`,raw:`{ enabled: boolean; recovery?: string; retentionDays?: number }`,signature:{properties:[{key:`enabled`,value:{name:`boolean`,required:!0}},{key:`recovery`,value:{name:`string`,required:!1}},{key:`retentionDays`,value:{name:`number`,required:!1}}]},required:!1}},{key:`archive`,value:{name:`signature`,type:`object`,raw:`{
  server: string;
  state: { tone: StatusTone; label: string; message?: string };
  bucket?: BucketView;
  window?: RestoreInterval[];
}`,signature:{properties:[{key:`server`,value:{name:`string`,required:!0}},{key:`state`,value:{name:`signature`,type:`object`,raw:`{ tone: StatusTone; label: string; message?: string }`,signature:{properties:[{key:`tone`,value:{name:`StatusTone`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`message`,value:{name:`string`,required:!1}}]},required:!0}},{key:`bucket`,value:{name:`signature`,type:`object`,raw:`{ name: string; location?: string }`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`location`,value:{name:`string`,required:!1}}]},required:!1}},{key:`window`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ from: string; to?: string; failing?: true }`,signature:{properties:[{key:`from`,value:{name:`string`,required:!0}},{key:`to`,value:{name:`string`,required:!1}},{key:`failing`,value:{name:`literal`,value:`true`,required:!1}}]}}],raw:`RestoreInterval[]`,required:!1}}]},required:!1}}]},required:!1}},{key:`schedules`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  cron: string;
  completed: number;
  failed: number;
  lastBackup?: string;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`ProcessCardId | SectionComponentId`,elements:[{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}]},{name:`SectionComponentId`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``},section:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  id: SectionComponentId;
  section: string;
  sectionLabel: string;
  registration: string;
  optionLabel: string;
  parameters: ClusterField[];
}`,signature:{properties:[{key:`id`,value:{name:`SectionComponentId`,required:!0}},{key:`section`,value:{name:`string`,required:!0}},{key:`sectionLabel`,value:{name:`string`,required:!0}},{key:`registration`,value:{name:`string`,required:!0}},{key:`optionLabel`,value:{name:`string`,required:!0}},{key:`parameters`,value:{name:`Array`,elements:[{name:`ClusterField`}],raw:`ClusterField[]`,required:!0}}]}},description:``}}}})))()}function Ye({context:e,requested:t}){let n=D(),[r,i]=(0,V.useState)(t),[a,o]=(0,V.useState)(t),[s,c]=(0,V.useState)(t),[l,u]=(0,V.useState)();t!==a&&(o(t),i(t),c(t));let d=e.claim;if(!d)return null;let f=n.session,p=Ze(e,d,f);if(!f)return(0,B.jsx)(ve,{view:p,stored:d,now:e.now});let m={},h={};for(let t of Xe(e,p)){let e=l?.panel===t.panel?l.on:void 0;t.panel===r?m[t.panel]=(0,B.jsx)(Qe,{def:t,scroll:s===t.panel,focus:e===`panel`,onShown:()=>{c(void 0),u(void 0)},onDone:()=>{i(void 0),u({panel:t.panel,on:`button`})}}):h[t.panel]=(0,B.jsx)($e,{name:t.name,focus:e===`button`,onFocused:()=>u(void 0),onClick:()=>{i(t.panel),u({panel:t.panel,on:`panel`})}})}return(0,B.jsx)(ve,{view:p,stored:d,now:e.now,edit:{panels:m,actions:h}})}function Xe(e,t){return[{panel:`basics`,label:`Basics`,name:`Basics`,render:()=>(0,B.jsx)(Ie,{view:t})},{panel:`release`,label:`Release and preset`,name:`Release and preset`,render:()=>(0,B.jsx)(Ve,{context:e})},...t.sections.map(t=>({panel:t.id,label:t.sectionLabel,name:`${t.sectionLabel}: ${t.optionLabel}`,render:()=>(0,B.jsx)(Ke,{context:e,section:t})})),{panel:`attachments`,label:`Attachments`,name:`Attachments`,render:()=>(0,B.jsx)(Ne,{context:e})},{panel:`deletion`,label:`Deletion policy`,name:`Deletion policy`,render:()=>(0,B.jsx)(ze,{context:e})}]}function Ze(e,t,n){if(!n)return ge({claim:t,cluster:e.cluster,merged:e.merged,environments:e.environments,sections:e.sections,registrations:e.registrations});let r=fe(Ce(e,n)).after,{releaseRef:i,platformConfigRef:a}=r.spec;return ge({claim:y(n),cluster:{...e.cluster,spec:{...e.cluster.spec,releaseRef:i,platformConfigRef:a}},merged:r,environments:e.environments,sections:e.sections,registrations:e.registrations})}function Qe({def:e,scroll:t,focus:n,onShown:r,onDone:i}){let a=(0,V.useRef)(null);return(0,V.useEffect)(()=>{(t||n)&&(t&&a.current?.scrollIntoView({block:`start`}),n&&a.current?.focus(),r())},[t,n,r]),(0,B.jsx)(he,{ref:a,label:e.label,name:e.name,editing:!0,action:(0,B.jsx)(T,{size:`sm`,variant:`outline`,onClick:i,children:`Done`}),children:e.render()})}function $e({name:e,focus:t,onFocused:n,onClick:r}){let i=(0,V.useRef)(null);return(0,V.useEffect)(()=>{t&&(i.current?.focus(),n())},[t,n]),(0,B.jsxs)(T,{ref:i,size:`sm`,variant:`ghost`,"aria-label":`Edit ${e}`,onClick:r,children:[(0,B.jsx)(Ae,{"aria-hidden":!0}),`Edit`]})}var B,V;function et(){return(et=e((()=>{B=n(),je(),l(),V=t(),le(),x(),_e(),me(),pe(),Me(),Fe(),Re(),E(),Be(),Ge(),Je(),Ye.__docgenInfo={description:`ClaimEditorPanels is the Claim tab with an Edit button on each panel that
has an editor. One panel is in edit at a time, in place of its read
panel. Done closes it and keeps its pending changes. The read view shows
the claim after the pending changes. requested opens its panel and
scrolls it into view, on mount and on each change. Without a session, it
shows the read view with no Edit button. It renders nothing without the
claim.`,methods:[],displayName:`ClaimEditorPanels`,props:{context:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  cluster: Resource<"CamundaCluster">;
  claim?: Resource<"CamundaClusterClaim">;
  merged: Merged<CamundaClusterSpec>;
  environment?: Resource<"Environment">;
  environments: readonly Resource<"Environment">[];
  presets: readonly Resource<"CamundaClusterPreset">[];
  releases: readonly Resource<"CamundaRelease">[];
  platformConfigs: readonly Resource<"CamundaPlatformConfig">[];
  managementAuthConfigs: readonly Resource<"ManagementAuthConfig">[];
  sections: readonly Resource<"PluginSection">[];
  registrations: readonly Resource<"PluginRegistration">[];
  requests: readonly Resource<"PluginRequest">[];
  storage?: Resource<"SecondaryStorageConfig">;
  backups: readonly BackupObject[];
  restores: readonly RestoreObject[];
  backupStorage: BackupStorage;
  schedules: readonly Resource<"BackupSchedule">[];
  optimize?: Resource<"CamundaOptimize">;
  fields: readonly ClusterField[];
  writeMode?: WriteMode;
  openId?: CardId;
  now: string;
}`,signature:{properties:[{key:`cluster`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaCluster"`}],raw:`Resource<"CamundaCluster">`,required:!0}},{key:`claim`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaClusterClaim"`}],raw:`Resource<"CamundaClusterClaim">`,required:!1}},{key:`merged`,value:{name:`Merged`,elements:[{name:`CamundaClusterSpec`}],raw:`Merged<CamundaClusterSpec>`,required:!0}},{key:`environment`,value:{name:`Resource`,elements:[{name:`literal`,value:`"Environment"`}],raw:`Resource<"Environment">`,required:!1}},{key:`environments`,value:{name:`unknown`,required:!0}},{key:`presets`,value:{name:`unknown`,required:!0}},{key:`releases`,value:{name:`unknown`,required:!0}},{key:`platformConfigs`,value:{name:`unknown`,required:!0}},{key:`managementAuthConfigs`,value:{name:`unknown`,required:!0}},{key:`sections`,value:{name:`unknown`,required:!0}},{key:`registrations`,value:{name:`unknown`,required:!0}},{key:`requests`,value:{name:`unknown`,required:!0}},{key:`storage`,value:{name:`Resource`,elements:[{name:`literal`,value:`"SecondaryStorageConfig"`}],raw:`Resource<"SecondaryStorageConfig">`,required:!1}},{key:`backups`,value:{name:`unknown`,required:!0}},{key:`restores`,value:{name:`unknown`,required:!0}},{key:`backupStorage`,value:{name:`union`,raw:`| { kind: "none"; schedules: ScheduleView[] }
| {
    kind: "elasticsearch" | "rdbms" | "unknown";
    missing?: string;
    bucket: BucketView;
    repository?: string;
    database?: DatabaseView;
    schedules: ScheduleView[];
  }`,elements:[{name:`signature`,type:`object`,raw:`{ kind: "none"; schedules: ScheduleView[] }`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"none"`,required:!0}},{key:`schedules`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  cron: string;
  completed: number;
  failed: number;
  lastBackup?: string;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{
  kind: "elasticsearch" | "rdbms" | "unknown";
  missing?: string;
  bucket: BucketView;
  repository?: string;
  database?: DatabaseView;
  schedules: ScheduleView[];
}`,signature:{properties:[{key:`kind`,value:{name:`union`,raw:`"elasticsearch" | "rdbms" | "unknown"`,elements:[{name:`literal`,value:`"elasticsearch"`},{name:`literal`,value:`"rdbms"`},{name:`literal`,value:`"unknown"`}],required:!0}},{key:`missing`,value:{name:`string`,required:!1}},{key:`bucket`,value:{name:`signature`,type:`object`,raw:`{ name: string; location?: string }`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`location`,value:{name:`string`,required:!1}}]},required:!1}},{key:`repository`,value:{name:`string`,required:!1}},{key:`database`,value:{name:`signature`,type:`object`,raw:`{
  missing?: string;
  serverConfig?: string;
  endpoint?: string;
  pitr?: { enabled: boolean; recovery?: string; retentionDays?: number };
  archive?: ArchiveView;
}`,signature:{properties:[{key:`missing`,value:{name:`string`,required:!1}},{key:`serverConfig`,value:{name:`string`,required:!1}},{key:`endpoint`,value:{name:`string`,required:!1}},{key:`pitr`,value:{name:`signature`,type:`object`,raw:`{ enabled: boolean; recovery?: string; retentionDays?: number }`,signature:{properties:[{key:`enabled`,value:{name:`boolean`,required:!0}},{key:`recovery`,value:{name:`string`,required:!1}},{key:`retentionDays`,value:{name:`number`,required:!1}}]},required:!1}},{key:`archive`,value:{name:`signature`,type:`object`,raw:`{
  server: string;
  state: { tone: StatusTone; label: string; message?: string };
  bucket?: BucketView;
  window?: RestoreInterval[];
}`,signature:{properties:[{key:`server`,value:{name:`string`,required:!0}},{key:`state`,value:{name:`signature`,type:`object`,raw:`{ tone: StatusTone; label: string; message?: string }`,signature:{properties:[{key:`tone`,value:{name:`StatusTone`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`message`,value:{name:`string`,required:!1}}]},required:!0}},{key:`bucket`,value:{name:`signature`,type:`object`,raw:`{ name: string; location?: string }`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`location`,value:{name:`string`,required:!1}}]},required:!1}},{key:`window`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ from: string; to?: string; failing?: true }`,signature:{properties:[{key:`from`,value:{name:`string`,required:!0}},{key:`to`,value:{name:`string`,required:!1}},{key:`failing`,value:{name:`literal`,value:`true`,required:!1}}]}}],raw:`RestoreInterval[]`,required:!1}}]},required:!1}}]},required:!1}},{key:`schedules`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  cron: string;
  completed: number;
  failed: number;
  lastBackup?: string;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`ProcessCardId | SectionComponentId`,elements:[{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}]},{name:`SectionComponentId`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``},requested:{required:!1,tsType:{name:`union`,raw:'"basics" | "release" | "attachments" | "deletion" | `section:${string}`',elements:[{name:`literal`,value:`"basics"`},{name:`literal`,value:`"release"`},{name:`literal`,value:`"attachments"`},{name:`literal`,value:`"deletion"`},{name:`literal`,value:"`section:${string}`"}]},description:``}}}})))()}function H({version:e=`8.9.3`,suspended:t=!1,claimSpec:n={},claimStatus:r={},over:i={}}){let o={namespace:`team-payments`,name:`orders`},s={presetRef:`medium`,releaseRef:`camunda-8-9-3`,zeebe:{replicas:3},...t&&{suspend:!0}},c={apiVersion:`core.camunda.io/v1`,kind:`CamundaCluster`,metadata:{...o,labels:{[a]:`orders`}},spec:s,status:e?{management:{version:e}}:{}},l={apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,metadata:{...o,resourceVersion:`41`},spec:{presetRef:`medium`,releaseRef:`camunda-8-9-3`,cluster:{zeebe:{replicas:3}},...t&&{suspend:!0},...n},status:{environment:`prod`,...r}},u={kind:`Environment`,metadata:{name:`prod`},spec:{displayName:`Production`,presets:{camundaCluster:[`medium`,`large`]},releases:{allowed:[`camunda-8-9-1`,`camunda-8-9-3`,`camunda-8-11-0`],default:`camunda-8-9-3`}}},f=[G(`8.9.1`),G(`8.9.3`),G(`8.11.0`)],p=[K(`medium`),K(`large`)];return{cluster:c,claim:l,merged:d(p[0].spec,f[1].spec,s),environment:u,environments:[u],presets:p,releases:f,platformConfigs:[],managementAuthConfigs:[],sections:[],registrations:[],requests:[],backups:[],restores:[],backupStorage:{kind:`none`,schedules:[]},schedules:[],fields:[],writeMode:`direct`,now:`2026-11-10T12:00:00Z`,...i}}var U,W,tt,nt,rt,it,G,K,at,ot,st,ct,q,J,Y,X,Z,Q,$,lt;function ut(){return(ut=e((()=>{U=n(),o(),f(),et(),E(),{expect:W,fn:tt,userEvent:nt,within:rt}=__STORYBOOK_MODULE_TEST__,it={preview:tt(async()=>({objects:[],errors:[]})),apply:tt(async()=>({ok:!0,mode:`direct`}))},G=e=>({kind:`CamundaRelease`,metadata:{name:`camunda-${e.replaceAll(`.`,`-`)}`},spec:{version:e}}),K=e=>({kind:`CamundaClusterPreset`,metadata:{name:e},spec:{cluster:{}}}),at={claimSpec:{plugins:[{section:`access`,registration:`gateway-access`,parameters:{hostPrefix:`orders`,sources:[`office`]}}]},claimStatus:{resolvedPolicy:{sections:{access:{allowed:[`gateway-access`]}},parameters:{"gateway-access":{hostPrefix:{claimEditable:!0,value:`camunda`},zone:{value:`payments.example.com`},sources:{claimEditable:!0,allowed:[{name:`office`,label:`Office`,value:`10.0.0.0/16`},{name:`vpn`,label:`VPN`,value:`10.8.0.0/16`}]}}}}},over:{sections:[{kind:`PluginSection`,metadata:{name:`access`},spec:{label:`Access`,selection:`One`,appliesTo:[`CamundaClusterClaim`]}}],registrations:[{kind:`PluginRegistration`,metadata:{name:`gateway-access`},spec:{label:`Gateway`,deletionPolicies:[`Delete`],parameters:{type:`object`,properties:{hostPrefix:{type:`string`,description:`The first label of each host name.`},zone:{type:`string`},sources:{type:`array`,items:{type:`string`}}}}}}]}},ot={claimSpec:{attachments:{optimize:{managementAuthConfigRef:`keycloak`,externalUrl:`https://optimize.payments.example.com`},backupSchedule:{schedule:`0 2 * * *`,retained:{completed:7}}}},over:{storage:{kind:`SecondaryStorageConfig`,metadata:{namespace:`team-payments`,name:`orders-storage`},spec:{type:`elasticsearch`}},managementAuthConfigs:[`camunda`,`keycloak`].map(e=>({kind:`ManagementAuthConfig`,metadata:{name:e},spec:{}})),backupStorage:{kind:`elasticsearch`,bucket:{name:`backups`,location:`s3://acme-backups/orders`},repository:`camunda-backups`,schedules:[]}}},st=class e{static{this.CLOSED=2}addEventListener(){}close(){this.readyState=e.CLOSED}constructor(){this.readyState=0}},ct={title:`cluster/edit/Claim editor`,component:Ye,args:{context:H({})},beforeEach:()=>{let e=globalThis.EventSource;return globalThis.EventSource=st,()=>{globalThis.EventSource=e}},decorators:[(e,{args:t})=>(0,U.jsx)(`div`,{className:`max-w-5xl`,children:(0,U.jsx)(we,{context:t.context,actions:it,children:(0,U.jsx)(e,{})},JSON.stringify(t.context))})]},q={name:`Read view with Edit buttons`},J={name:`Basics in edit`,args:{requested:`basics`},play:async({canvasElement:e})=>{let t=rt(e).getByRole(`checkbox`,{name:`Suspend the cluster`});await nt.click(t),await W(t).toBeChecked()}},Y={name:`Release with a downgrade`,args:{requested:`release`},play:async({canvasElement:e})=>{let t=rt(e);await nt.selectOptions(t.getByRole(`combobox`,{name:`Release`}),`camunda-8-9-1`),await W(t.getByText(/is a downgrade\./)).toBeVisible()}},X={name:`Release while suspended`,args:{context:H({version:``,suspended:!0}),requested:`release`}},Z={name:`A section with an editable parameter`,args:{context:H(at),requested:`section:access`}},Q={name:`Attachments in edit`,args:{context:H(ot),requested:`attachments`}},$={name:`Deletion with Keep refused`,args:{context:H(at),requested:`deletion`}},lt=[`ReadViewWithEditButtons`,`BasicsInEdit`,`ReleaseWithADowngrade`,`ReleaseWhileSuspended`,`ASectionWithAnEditableParameter`,`AttachmentsInEdit`,`DeletionWithKeepRefused`],q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  name: "Read view with Edit buttons"
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "Basics in edit",
  args: {
    requested: "basics"
  },
  play: async ({
    canvasElement
  }) => {
    const suspend = within(canvasElement).getByRole("checkbox", {
      name: "Suspend the cluster"
    });
    await userEvent.click(suspend);
    await expect(suspend).toBeChecked();
  }
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "Release with a downgrade",
  args: {
    requested: "release"
  },
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    await userEvent.selectOptions(canvas.getByRole("combobox", {
      name: "Release"
    }), "camunda-8-9-1");
    await expect(canvas.getByText(/is a downgrade\\./)).toBeVisible();
  }
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "Release while suspended",
  args: {
    context: contextOf({
      version: "",
      suspended: true
    }),
    requested: "release"
  }
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "A section with an editable parameter",
  args: {
    context: contextOf(ACCESS),
    requested: "section:access"
  }
}`,...Z.parameters?.docs?.source}}},Q.parameters={...Q.parameters,docs:{...Q.parameters?.docs,source:{originalSource:`{
  name: "Attachments in edit",
  args: {
    context: contextOf(ATTACHED),
    requested: "attachments"
  }
}`,...Q.parameters?.docs?.source}}},$.parameters={...$.parameters,docs:{...$.parameters?.docs,source:{originalSource:`{
  name: "Deletion with Keep refused",
  args: {
    context: contextOf(ACCESS),
    requested: "deletion"
  }
}`,...$.parameters?.docs?.source}}}})))()}ut();export{Z as ASectionWithAnEditableParameter,Q as AttachmentsInEdit,J as BasicsInEdit,$ as DeletionWithKeepRefused,q as ReadViewWithEditButtons,X as ReleaseWhileSuspended,Y as ReleaseWithADowngrade,lt as __namedExportsOrder,ct as default};