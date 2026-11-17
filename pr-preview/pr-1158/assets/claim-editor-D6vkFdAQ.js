import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{a as r,t as i}from"./hooks-Bys_naRE.js";import{n as a,t as o}from"./createLucideIcon-B55dJ-eE.js";import{l as s}from"./iframe-aMeaDM-O.js";import{n as c}from"./json-CPrrxgTM.js";import{t as l,u}from"./policy-vB0zORS5.js";import{a as d,o as f}from"./draft-D2PSPGd3.js";import{v as p}from"./schema-eTMNNukr.js";import{t as m}from"./fields-gfHAo22I.js";import{n as h}from"./labels-DYL500th.js";import{at as g,it as _,t as v}from"./session-CkZc4fR5.js";import{n as y,t as b}from"./attachment-fields-D-ruccNM.js";import{n as x,t as S}from"./locked-field-BEGQxrUK.js";import{n as ee,t as te}from"./checkbox-DJ8TlJcr.js";import{n as C,r as ne,t as re}from"./native-select-CJ9qjdUU.js";import{c as ie,l as ae}from"./settings-editor-CuFB_u1T.js";import{r as oe,t as w}from"./button-t9SAe2aN.js";import{n as se,r as ce}from"./secret-ref-picker-ydGqlv8K.js";import{a as le,n as ue}from"./deletion-choice-N4Jxp3T1.js";import{c as de,n as fe,o as pe,s as me,t as he}from"./claim-tab-DJYlmQm3.js";import{i as ge,r as _e}from"./impacts-Cg1TPmB2.js";import{a as ve,i as ye,n as T,o as be,r as E}from"./cluster-edit-C62qlpGp.js";import{i as xe,n as Se,r as Ce}from"./release-options-CUChB9ZF.js";import{n as we,t as Te}from"./selection-settings-editor-DXgNiBJU.js";var D,O;function k(){return(k=e((()=>{a(),D={name:`pencil`,size:24,node:[[`path`,{d:`M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z`,key:`1a8usu`}],[`path`,{d:`m15 5 4 4`,key:`1mk7zo`}]]},D.node,O=o(D)})))()}function A(){return(A=e((()=>{})))()}function j({context:e}){let t=E(),{session:n}=t;return n?(0,M.jsx)(b,{value:_(n).spec?.attachments,set:t.set,reset:t.reset,invalid:t.invalid,context:{secondaryStorage:e.storage?.spec.type,backupStorage:e.backupStorage.kind!==`none`,managementAuthConfigs:e.managementAuthConfigs.map(e=>e.metadata?.name??``).sort(c),lock:e=>g(n,e),errors:t.errors,pending:t.pending,lockedUrl:e.optimize?.spec.externalUrl,optimizeHref:e.optimize?`?tab=overview&component=optimize`:void 0}}):null}var M;function N(){return(N=e((()=>{M=n(),l(),v(),y(),T(),j.__docgenInfo={description:`AttachmentsPanel edits the attachments of the claim through
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`ProcessCardId | SectionComponentId`,elements:[{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}]},{name:`SectionComponentId`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``}}}})))()}function P({view:e}){let{session:t,set:n,reset:r}=E(),i=(0,I.useId)();if(!t)return null;let a=h({reason:`atCreation`}),o=g(t,L),s=e=>{!e&&t.claim.spec?.suspend===void 0?r(L):n(L,e)};return(0,F.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[(0,F.jsxs)(`div`,{className:`grid gap-4 sm:grid-cols-3`,children:[(0,F.jsx)(S,{label:`Name`,value:e.name,reason:a}),(0,F.jsx)(S,{label:`Namespace`,value:e.namespace,reason:a}),(0,F.jsx)(S,{label:`Environment`,value:e.environment??`No environment`,reason:a})]}),o?(0,F.jsx)(S,{label:`Suspend the cluster`,value:e.suspend?`Yes`:`No`,reason:h(o)}):(0,F.jsxs)(`div`,{className:`flex items-start gap-3`,children:[(0,F.jsx)(te,{id:i,className:`mt-0.5`,checked:e.suspend,"aria-describedby":`${i}-hint`,onCheckedChange:e=>s(e===!0)}),(0,F.jsxs)(`div`,{className:`flex flex-col gap-0.5`,children:[(0,F.jsx)(`label`,{htmlFor:i,className:`text-sm font-medium text-text`,children:`Suspend the cluster`}),(0,F.jsx)(`p`,{id:`${i}-hint`,className:`text-xs text-muted`,children:`Every workload goes to zero. The volumes stay.`})]})]})]})}var F,I,L;function R(){return(R=e((()=>{F=n(),I=t(),x(),ee(),m(),v(),T(),L=[{field:`spec`},{field:`suspend`}],P.__docgenInfo={description:`BasicsPanel edits the Basics of the claim: the name, the namespace, and
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`sectionLabel`,value:{name:`string`,required:!0}},{key:`optionLabel`,value:{name:`string`,required:!0}},{key:`outputsReady`,value:{name:`string`,required:!1}},{key:`ready`,value:{name:`string`,required:!1}}]}}],raw:`ClaimRequestRow[]`,required:!0}}]}},description:``}}}})))()}function Ee({context:e}){let t=E();if(!t.session)return null;let n=_(t.session),r=u({claim:{...n,spec:{...n.spec,plugins:n.spec?.plugins}},environment:e.environment,sections:e.sections,registrations:e.registrations,kind:`CamundaClusterClaim`}),i=r.policy===`Orphan`&&!r.keepAllowed,a=t.session.claim.spec?.deletionPolicy;return(0,z.jsxs)(`div`,{className:`flex flex-col gap-3`,children:[(0,z.jsx)(ue,{subject:`cluster`,choice:r,onChange:e=>e===r.defaultPolicy&&!a?t.reset(B):t.set(B,e)}),i&&(0,z.jsx)(`p`,{className:`text-sm text-warn`,children:`The claim keeps its resources, but an option below cannot keep them. Choose Delete, or remove that option.`})]})}var z,B;function De(){return(De=e((()=>{z=n(),le(),l(),v(),T(),B=[{field:`spec`},{field:`deletionPolicy`}],Ee.__docgenInfo={description:`DeletionPanel edits the deletion policy of the claim. Choosing the
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`ProcessCardId | SectionComponentId`,elements:[{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}]},{name:`SectionComponentId`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``}}}})))()}function Oe({context:e}){let t=E(),{session:n}=t;if(!n)return null;let r=n.claim.spec,i=_(n).spec??{},a=e=>{let n=p(``,e);return t.errors.filter(e=>e.path===n).map(e=>e.message)},o=e=>g(n,e),s=(e,n,i)=>!Ae(r,e)&&(n===``||n===i)||n===``?t.reset(e):t.set(e,n),{options:c,note:l}=Ce({cluster:e.cluster,claim:n.claim,releases:e.releases,environment:e.environment,backups:e.backups}),u=i.releaseRef||e.cluster.spec.releaseRef||``,d=c.find(e=>e.name===u),f=n=>{s(U,n,e.cluster.spec.releaseRef);let i=c.find(e=>e.name===n)?.downgrade?.to;i?t.set(K,i):r?.allowVersionDowngrade===void 0?t.reset(K):t.set(K,r.allowVersionDowngrade)},m=ke(e,r?.presetRef),h=i.presetRef??``,v=e.environment?.spec.platformConfigRef,y=i.platformConfigRef??``,b=t.pending===0&&ye(e.cluster,n.claim);return(0,H.jsxs)(`div`,{className:`flex flex-col gap-5`,children:[(0,H.jsx)(V,{label:`Release`,value:u,lock:o(U),errors:a(U),onChange:f,hint:(0,H.jsxs)(H.Fragment,{children:[d?.downgrade&&(0,H.jsxs)(`p`,{className:`text-xs text-warn`,children:[d.downgrade.from,` to `,d.downgrade.to,` is a downgrade. The review asks you to confirm it.`]}),l&&(0,H.jsx)(`p`,{className:`text-xs text-muted`,children:l})]}),children:c.map(e=>(0,H.jsx)(C,{value:e.name,disabled:e.disabled!==void 0&&!e.isCurrent,children:xe(e,e.isDefault)},e.name))}),b&&(0,H.jsxs)(`div`,{className:`flex flex-wrap items-center gap-x-3 gap-y-2 rounded-md border border-border bg-panel-2 px-3 py-2 text-sm text-text-2`,children:[(0,H.jsxs)(`p`,{className:`min-w-0 flex-1`,children:[`The cluster runs `,e.cluster.status?.management?.version,`, so allowVersionDowngrade can go.`]}),(0,H.jsx)(w,{size:`sm`,variant:`outline`,onClick:()=>{t.reset(K),t.review()},children:`Clear`})]}),(0,H.jsx)(V,{label:`Preset`,value:h,lock:o(W),errors:a(W),onChange:e=>s(W,e),children:m.map(e=>(0,H.jsx)(C,{value:e,children:e},e))}),(0,H.jsxs)(V,{label:`Platform config`,value:y,lock:o(G),errors:a(G),onChange:e=>s(G,e),children:[(0,H.jsx)(C,{value:``,children:v?`Default of the environment`:`Not set`}),e.platformConfigs.map(e=>(0,H.jsx)(C,{value:e.metadata?.name,children:e.metadata?.name},e.metadata?.name))]})]})}function ke(e,t){let n=f(e.presets,e.environment).map(e=>e.metadata.name??``);return t&&!n.includes(t)?[...n,t]:n}function Ae(e,t){let n=t[1];return e&&`field`in n?e[n.field]:void 0}function V({label:e,value:t,lock:n,errors:r,hint:i,onChange:a,children:o}){let s=(0,je.useId)();return n?(0,H.jsx)(S,{label:e,value:t||`Not set`,reason:h(n)}):(0,H.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1.5`,children:[(0,H.jsx)(`label`,{htmlFor:s,className:`text-sm font-medium text-text`,children:e}),(0,H.jsx)(re,{id:s,value:t,"aria-invalid":r.length>0||void 0,"aria-describedby":r.length>0?`${s}-error`:void 0,onChange:e=>a(e.target.value),children:o}),i,r.length>0&&(0,H.jsx)(`p`,{id:`${s}-error`,className:`text-xs text-bad`,children:r.join(` `)})]})}var H,je,U,W,G,K;function Me(){return(Me=e((()=>{H=n(),je=t(),x(),oe(),ne(),d(),m(),v(),T(),ve(),Se(),U=[{field:`spec`},{field:`releaseRef`}],W=[{field:`spec`},{field:`presetRef`}],G=[{field:`spec`},{field:`platformConfigRef`}],K=[{field:`spec`},{field:`allowVersionDowngrade`}],Oe.__docgenInfo={description:`ReleasePanel edits the release, the preset, and the platform config of
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`ProcessCardId | SectionComponentId`,elements:[{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}]},{name:`SectionComponentId`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``}}}})))()}function Ne({context:e,section:t}){let{session:n,pending:i,set:a,reset:o,invalid:s,errors:c}=E(),{generation:l,own:u}=ae(i),d=e.cluster.metadata?.namespace??``,f=ce(r(`Secret`,{namespace:d},[]).items);if(!n)return null;let p=e.registrations.find(e=>e.metadata?.name===t.registration),m=_(n),g=m.status?.resolvedPolicy?.parameters?.[t.registration]??{},v=(m.spec?.plugins??[]).find(e=>e.section===t.section&&e.registration===t.registration),y=Object.fromEntries(Object.entries(g).map(([e,t])=>[e,t.allowed])),b=t.parameters.some(e=>!e.lock&&e.claimSteps);return(0,q.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[(0,q.jsx)(S,{label:`Option`,value:t.optionLabel,reason:h({reason:`chosenAtCreation`})}),b?(0,q.jsx)(Te,{fields:t.parameters,schema:p?.spec.parameters,allowed:y,claimed:Pe(v?.parameters),secrets:f,errors:c,set:u(a),reset:u(o),invalid:s},l):(0,q.jsx)(`p`,{className:`text-sm text-muted`,children:`No parameters you can change.`})]})}function Pe(e){return typeof e==`object`&&e&&!Array.isArray(e)?e:{}}var q;function Fe(){return(Fe=e((()=>{q=n(),x(),se(),m(),i(),v(),we(),T(),ie(),Ne.__docgenInfo={description:`SectionPanel edits one selection of the claim: its option is locked as
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
}`,signature:{properties:[{key:`id`,value:{name:`SectionComponentId`,required:!0}},{key:`section`,value:{name:`string`,required:!0}},{key:`sectionLabel`,value:{name:`string`,required:!0}},{key:`registration`,value:{name:`string`,required:!0}},{key:`optionLabel`,value:{name:`string`,required:!0}},{key:`parameters`,value:{name:`Array`,elements:[{name:`ClusterField`}],raw:`ClusterField[]`,required:!0}}]}},description:``}}}})))()}function J({context:e,requested:t}){let n=E(),[r,i]=(0,Q.useState)(t),[a,o]=(0,Q.useState)(t),[s,c]=(0,Q.useState)(t),[l,u]=(0,Q.useState)();t!==a&&(o(t),i(t),c(t));let d=e.claim;if(!d)return null;let f=n.session,p=Le(e,d,f);if(!f)return(0,Z.jsx)(he,{view:p,stored:d,now:e.now});let m={},h={};for(let t of Ie(e,p)){let e=l?.panel===t.panel?l.on:void 0;t.panel===r?m[t.panel]=(0,Z.jsx)(Y,{def:t,scroll:s===t.panel,focus:e===`panel`,onShown:()=>{c(void 0),u(void 0)},onDone:()=>{i(void 0),u({panel:t.panel,on:`button`})}}):h[t.panel]=(0,Z.jsx)(X,{name:t.name,focus:e===`button`,onFocused:()=>u(void 0),onClick:()=>{i(t.panel),u({panel:t.panel,on:`panel`})}})}return(0,Z.jsx)(he,{view:p,stored:d,now:e.now,edit:{panels:m,actions:h}})}function Ie(e,t){return[{panel:`basics`,label:`Basics`,name:`Basics`,render:()=>(0,Z.jsx)(P,{view:t})},{panel:`release`,label:`Release and preset`,name:`Release and preset`,render:()=>(0,Z.jsx)(Oe,{context:e})},...t.sections.map(t=>({panel:t.id,label:t.sectionLabel,name:`${t.sectionLabel}: ${t.optionLabel}`,render:()=>(0,Z.jsx)(Ne,{context:e,section:t})})),{panel:`attachments`,label:`Attachments`,name:`Attachments`,render:()=>(0,Z.jsx)(j,{context:e})},{panel:`deletion`,label:`Deletion policy`,name:`Deletion policy`,render:()=>(0,Z.jsx)(Ee,{context:e})}]}function Le(e,t,n){return n?Re(e,n):me({claim:t,cluster:e.cluster,merged:e.merged,environments:e.environments,sections:e.sections,registrations:e.registrations})}function Re(e,t){let n=ge(be(e,t)).after,{releaseRef:r,platformConfigRef:i}=n.spec;return me({claim:_(t),cluster:{...e.cluster,spec:{...e.cluster.spec,releaseRef:r,platformConfigRef:i}},merged:n,environments:e.environments,sections:e.sections,registrations:e.registrations})}function Y({def:e,scroll:t,focus:n,onShown:r,onDone:i}){let a=(0,Q.useRef)(null);return(0,Q.useEffect)(()=>{(t||n)&&(t&&a.current?.scrollIntoView({block:`start`}),n&&a.current?.focus(),r())},[t,n,r]),(0,Z.jsx)(fe,{ref:a,label:e.label,name:e.name,editing:!0,action:(0,Z.jsx)(w,{size:`sm`,variant:`outline`,onClick:i,children:`Done`}),children:e.render()})}function X({name:e,focus:t,onFocused:n,onClick:r}){let i=(0,Q.useRef)(null);return(0,Q.useEffect)(()=>{t&&(i.current?.focus(),n())},[t,n]),(0,Z.jsxs)(w,{ref:i,size:`sm`,variant:`ghost`,"aria-label":`Edit ${e}`,onClick:r,children:[(0,Z.jsx)(O,{"aria-hidden":!0}),`Edit`]})}var Z,Q;function $(){return($=e((()=>{Z=n(),k(),s(),Q=t(),oe(),v(),de(),pe(),_e(),A(),N(),R(),T(),De(),Me(),Fe(),J.__docgenInfo={description:`ClaimEditorPanels is the Claim tab with an Edit button on each panel that
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`ProcessCardId | SectionComponentId`,elements:[{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}]},{name:`SectionComponentId`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``},requested:{required:!1,tsType:{name:`union`,raw:'"basics" | "release" | "attachments" | "deletion" | `section:${string}`',elements:[{name:`literal`,value:`"basics"`},{name:`literal`,value:`"release"`},{name:`literal`,value:`"attachments"`},{name:`literal`,value:`"deletion"`},{name:`literal`,value:"`section:${string}`"}]},description:``}}},Y.__docgenInfo={description:`EditPanel is a panel of a Claim tab in edit, with a Done button. It
scrolls into view when scroll is set, takes the focus when focus is set,
and then calls onShown.`,methods:[],displayName:`EditPanel`,props:{def:{required:!0,tsType:{name:`Pick`,elements:[{name:`signature`,type:`object`,raw:`{
  panel: ClaimPanel;
  label: string;
  name: string;
  render: () => ReactNode;
}`,signature:{properties:[{key:`panel`,value:{name:`union`,raw:'"basics" | "release" | "attachments" | "deletion" | `section:${string}`',elements:[{name:`literal`,value:`"basics"`},{name:`literal`,value:`"release"`},{name:`literal`,value:`"attachments"`},{name:`literal`,value:`"deletion"`},{name:`literal`,value:"`section:${string}`"}],required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`render`,value:{name:`signature`,type:`function`,raw:`() => ReactNode`,signature:{arguments:[],return:{name:`ReactNode`}},required:!0}}]}},{name:`union`,raw:`"label" | "name" | "render"`,elements:[{name:`literal`,value:`"label"`},{name:`literal`,value:`"name"`},{name:`literal`,value:`"render"`}]}],raw:`Pick<EditDef, "label" | "name" | "render">`},description:``},scroll:{required:!0,tsType:{name:`boolean`},description:``},focus:{required:!0,tsType:{name:`boolean`},description:``},onShown:{required:!0,tsType:{name:`signature`,type:`function`,raw:`() => void`,signature:{arguments:[],return:{name:`void`}}},description:``},onDone:{required:!0,tsType:{name:`signature`,type:`function`,raw:`() => void`,signature:{arguments:[],return:{name:`void`}}},description:``}}},X.__docgenInfo={description:`EditButton opens the editor of the panel name. It takes the focus when focus is set, and then calls onFocused.`,methods:[],displayName:`EditButton`,props:{name:{required:!0,tsType:{name:`string`},description:``},focus:{required:!0,tsType:{name:`boolean`},description:``},onFocused:{required:!0,tsType:{name:`signature`,type:`function`,raw:`() => void`,signature:{arguments:[],return:{name:`void`}}},description:``},onClick:{required:!0,tsType:{name:`signature`,type:`function`,raw:`() => void`,signature:{arguments:[],return:{name:`void`}}},description:``}}}})))()}export{$ as i,X as n,Y as r,J as t};