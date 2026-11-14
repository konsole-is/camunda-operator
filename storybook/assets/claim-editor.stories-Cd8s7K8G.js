import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{a as r,t as i}from"./hooks-Cuq9NGnJ.js";import{a,h as o}from"./labels-a4ptf10d.js";import{s}from"./iframe-DYHsYrxr.js";import{n as c,t as l}from"./link-DOLMGIR2.js";import{n as u,t as d}from"./checkbox-Bkv1pnZ7.js";import{r as f,t as p}from"./button-lmY3Hw21.js";import{n as m,t as h}from"./input-DEWa4Ywe.js";import{n as ee,t as te}from"./createLucideIcon-B55dJ-eE.js";import{l as ne,n as re,r as ie,t as ae,y as oe}from"./release-B_s19T_Z.js";import{c as se,t as ce}from"./merge-B877RDyT.js";import{m as le,p as ue,s as de,t as fe}from"./policy-UrtrXbln.js";import{a as pe,i as me}from"./draft-W6Wspu-a.js";import{C as he,t as g}from"./fields-D4ttTZ0d.js";import{n as _}from"./labels-LUZ4_yzX.js";import{f as v,p as y,t as b}from"./session-_5lMhwPU.js";import{a as ge,i as _e}from"./backup-caution-BPAYddiT.js";import{a as ve,i as ye,n as be,r as xe,t as Se}from"./claim-tab-DQnKIh48.js";import{i as Ce,n as we}from"./deletion-choice-CnaYvh35.js";import{n as x,r as Te,t as Ee}from"./native-select-DLYyprlG.js";import{n as S,t as C}from"./locked-field-Bczcj0Gm.js";import{n as De,r as Oe}from"./secret-ref-picker-UNokba01.js";import{c as ke,i as Ae,l as je,r as Me}from"./settings-editor-CQS1L21Y.js";import{a as Ne,i as w,n as Pe,o as Fe,r as T,s as Ie}from"./cluster-edit-BeM-IT2l.js";import{n as Le,t as Re}from"./selection-settings-editor-CsMIPQIp.js";var ze,Be;function Ve(){return(Ve=e((()=>{ee(),ze={name:`pencil`,size:24,node:[[`path`,{d:`M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z`,key:`1a8usu`}],[`path`,{d:`m15 5 4 4`,key:`1mk7zo`}]]},ze.node,Be=te(ze)})))()}function He(){return(He=e((()=>{})))()}function Ue({context:e}){let t=w(),{session:n}=t,{generation:r,own:i}=je(t.pending);if(!n)return null;let a=v(n).spec?.attachments,o=a?.optimize,s=a?.backupSchedule,l=e.managementAuthConfigs.map(e=>e.metadata?.name??``).sort(oe),u=e.storage?.spec.type===`elasticsearch`?l.length===0?`No ManagementAuthConfig exists. Your admin creates one first.`:void 0:`Optimize needs Elasticsearch as secondary storage.`,d=e.backupStorage.kind===`none`?`Needs a backup storage option.`:void 0,f=y(n,A),p=e=>Me(t.errors,e).map(e=>e.message),m=e=>{let n=he(``,e);return t.errors.filter(e=>e.path===n).map(e=>e.message)};return(0,E.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,E.jsx)(We,{label:`Attach Optimize`,checked:o!==void 0,off:o===void 0?u:void 0,lock:y(n,k),errors:m(k),onChange:e=>e?t.set(k,{managementAuthConfigRef:l[0]}):t.reset(k),children:o&&(0,E.jsxs)(E.Fragment,{children:[(0,E.jsx)(Ge,{label:`ManagementAuthConfig`,errors:p(Xe),children:e=>(0,E.jsx)(Ee,{...e,value:o.managementAuthConfigRef,onChange:e=>t.set(Xe,e.target.value),children:[...new Set([o.managementAuthConfigRef,...l])].sort(oe).map(e=>(0,E.jsx)(x,{value:e,children:e},e))})}),f?(0,E.jsx)(C,{label:`External URL`,value:o.externalUrl??e.optimize?.spec.externalUrl??`Not set`,reason:_(f)}):(0,E.jsx)(Ge,{label:`External URL`,errors:p(A),children:e=>(0,E.jsx)(h,{...e,type:`url`,spellCheck:!1,placeholder:`https://optimize.example.com`,value:o.externalUrl??``,onChange:e=>e.target.value===``?t.reset(A):t.set(A,e.target.value)})}),e.optimize&&(0,E.jsx)(c,{href:`?tab=overview&component=optimize`,className:`self-start text-sm font-medium text-text underline underline-offset-4 hover:text-accent`,children:`Open Optimize`})]})}),(0,E.jsx)(We,{label:`Back up on a schedule`,checked:s!==void 0,off:s===void 0?d:void 0,lock:y(n,j),errors:m(j),onChange:e=>e?t.set(j,{schedule:et}):t.reset(j),children:s&&(0,E.jsxs)(E.Fragment,{children:[(0,E.jsx)(Ge,{label:`Schedule (cron, UTC)`,errors:p(Ze),children:e=>(0,E.jsx)(h,{...e,className:`max-w-56 font-mono`,spellCheck:!1,value:s.schedule,onChange:e=>t.set(Ze,e.target.value)})}),(0,E.jsxs)(`div`,{className:`flex flex-wrap gap-4`,children:[(0,E.jsx)(qe,{label:`Completed backups to keep`,steps:Qe,minimum:1,value:s.retained?.completed,errors:p(Qe),set:i(t.set),reset:i(t.reset)}),(0,E.jsx)(qe,{label:`Failed backups to keep`,steps:$e,minimum:0,value:s.retained?.failed,errors:p($e),set:i(t.set),reset:i(t.reset)})]},r)]})})]})}function We({label:e,checked:t,off:n,lock:r,errors:i,onChange:a,children:o}){let s=(0,D.useId)();return(0,E.jsxs)(`fieldset`,{className:`flex min-w-0 flex-col gap-3`,children:[(0,E.jsxs)(`div`,{className:`flex items-start gap-3`,children:[(0,E.jsx)(d,{id:s,className:`mt-0.5`,checked:t,disabled:n!==void 0||r!==void 0,"aria-describedby":n||r?`${s}-note`:void 0,onCheckedChange:e=>a(e===!0)}),(0,E.jsxs)(`div`,{className:`flex flex-col gap-0.5`,children:[(0,E.jsx)(`label`,{htmlFor:s,className:`text-sm font-medium text-text`,children:e}),(n||r)&&(0,E.jsx)(`p`,{id:`${s}-note`,className:`text-xs text-muted`,children:n??(r&&_(r))}),(0,E.jsx)(Ke,{errors:i})]})]}),t&&(0,E.jsx)(`div`,{className:`ml-7 flex flex-col gap-4 border-l border-divider pl-4`,children:o})]})}function Ge({label:e,errors:t,children:n}){let r=(0,D.useId)(),i=t.length>0;return(0,E.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1.5`,children:[(0,E.jsx)(`label`,{htmlFor:r,className:`text-sm font-medium text-text`,children:e}),n({id:r,...i&&{"aria-invalid":!0,"aria-describedby":`${r}-error`}}),(0,E.jsx)(Ke,{id:`${r}-error`,errors:t})]})}function Ke({id:e,errors:t}){return t.length===0?null:(0,E.jsx)(`p`,{id:e,className:`text-xs text-bad`,children:t.join(` `)})}function qe({label:e,steps:t,minimum:n,value:r,errors:i,set:a,reset:o}){let{invalid:s}=w(),{generation:c,own:l}=je(0,r);return(0,D.useEffect)(()=>()=>s(t,void 0),[s,t]),(0,E.jsx)(Je,{label:e,minimum:n,value:r,errors:i,onChange:l(r=>{let i=Ye(r,n);s(t,i&&`${e}: ${i}`),i||(r===``?o(t):a(t,Number(r)))})},c)}function Je({label:e,minimum:t,value:n,errors:r,onChange:i}){let a=(0,D.useId)(),[o,s]=(0,D.useState)(n===void 0?``:String(n)),c=Ye(o,t)??(r.join(` `)||void 0);return(0,E.jsxs)(`div`,{className:`flex flex-col gap-1.5`,children:[(0,E.jsx)(`label`,{htmlFor:a,className:`text-sm font-medium text-text`,children:e}),(0,E.jsx)(h,{id:a,type:`number`,min:t,step:1,className:`w-32`,value:o,"aria-invalid":c?!0:void 0,"aria-describedby":c?`${a}-error`:void 0,onChange:e=>{s(e.target.value),i(e.target.value)}}),c&&(0,E.jsx)(`p`,{id:`${a}-error`,className:`text-xs text-bad`,children:c})]})}function Ye(e,t){if(e===``)return;let n=Number(e);if(!Number.isInteger(n))return`Enter a whole number`;if(n<t)return`Enter ${t} or more`}var E,D,O,k,Xe,A,j,Ze,Qe,$e,et;function tt(){return(tt=e((()=>{E=n(),l(),D=t(),S(),u(),m(),Te(),g(),fe(),b(),Ae(),T(),ke(),O=e=>({field:e}),k=[O(`spec`),O(`attachments`),O(`optimize`)],Xe=[...k,O(`managementAuthConfigRef`)],A=[...k,O(`externalUrl`)],j=[O(`spec`),O(`attachments`),O(`backupSchedule`)],Ze=[...j,O(`schedule`)],Qe=[...j,O(`retained`),O(`completed`)],$e=[...j,O(`retained`),O(`failed`)],et=`0 2 * * *`,Ue.__docgenInfo={description:`AttachmentsPanel edits the attachments of the claim. Attach Optimize
sets spec.attachments.optimize to the first ManagementAuthConfig by
name, and turning it off removes the attachment with its settings. It
needs Elasticsearch as secondary storage and a ManagementAuthConfig.
While Optimize is on, the panel edits its ManagementAuthConfig and its
external URL, which stays locked while a section fills it. Back up on a
schedule sets DEFAULT_SCHEDULE and needs a backup storage option; while
it is on, the panel edits the cron and the counts to keep, where an
empty count removes the field. It renders nothing without a session.`,methods:[],displayName:`AttachmentsPanel`,props:{context:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``}}}})))()}function nt({view:e}){let{session:t,set:n,reset:r}=w(),i=(0,rt.useId)();if(!t)return null;let a=_({reason:`atCreation`}),o=y(t,N),s=e=>{!e&&t.claim.spec?.suspend===void 0?r(N):n(N,e)};return(0,M.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[(0,M.jsxs)(`div`,{className:`grid gap-4 sm:grid-cols-3`,children:[(0,M.jsx)(C,{label:`Name`,value:e.name,reason:a}),(0,M.jsx)(C,{label:`Namespace`,value:e.namespace,reason:a}),(0,M.jsx)(C,{label:`Environment`,value:e.environment??`No environment`,reason:a})]}),o?(0,M.jsx)(C,{label:`Suspend the cluster`,value:e.suspend?`Yes`:`No`,reason:_(o)}):(0,M.jsxs)(`div`,{className:`flex items-start gap-3`,children:[(0,M.jsx)(d,{id:i,className:`mt-0.5`,checked:e.suspend,"aria-describedby":`${i}-hint`,onCheckedChange:e=>s(e===!0)}),(0,M.jsxs)(`div`,{className:`flex flex-col gap-0.5`,children:[(0,M.jsx)(`label`,{htmlFor:i,className:`text-sm font-medium text-text`,children:`Suspend the cluster`}),(0,M.jsx)(`p`,{id:`${i}-hint`,className:`text-xs text-muted`,children:`Every workload goes to zero. The volumes stay.`})]})]})]})}var M,rt,N;function it(){return(it=e((()=>{M=n(),rt=t(),S(),u(),g(),b(),T(),N=[{field:`spec`},{field:`suspend`}],nt.__docgenInfo={description:`BasicsPanel edits the Basics of the claim: the name, the namespace, and
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`sectionLabel`,value:{name:`string`,required:!0}},{key:`optionLabel`,value:{name:`string`,required:!0}},{key:`outputsReady`,value:{name:`string`,required:!1}},{key:`ready`,value:{name:`string`,required:!1}}]}}],raw:`ClaimRequestRow[]`,required:!0}}]}},description:``}}}})))()}function at({context:e}){let t=w();if(!t.session)return null;let n=ot(e,t.session),r=n.policy===`Orphan`&&!n.keepAllowed,i=t.session.claim.spec?.deletionPolicy;return(0,P.jsxs)(`div`,{className:`flex flex-col gap-3`,children:[(0,P.jsx)(we,{subject:`cluster`,choice:n,onChange:e=>e===n.defaultPolicy&&!i?t.reset(st):t.set(st,e)}),r&&(0,P.jsx)(`p`,{className:`text-sm text-warn`,children:ct})]})}function ot(e,t){let n=v(t),r=n.spec?.plugins??[],i=e.registrations,a=e.environment,o=e.sections.map(e=>de(e,i,a,`CamundaClusterClaim`)),s=n.status?.resolvedPolicy;return{...ue(n.spec?.deletionPolicy,r,a,i,o,le(void 0,s)),policy:le(n.spec?.deletionPolicy,s)}}var P,st,ct;function lt(){return(lt=e((()=>{P=n(),Ce(),fe(),b(),T(),st=[{field:`spec`},{field:`deletionPolicy`}],ct=`The claim keeps its resources, but an option below cannot keep them. Choose Delete, or remove that option.`,at.__docgenInfo={description:`DeletionPanel edits the deletion policy of the claim. Choosing the
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``}}}})))()}function ut(e){let{cluster:t,claim:n,releases:r,backups:i}=e,a=e.environment,o=n.spec?.releaseRef||t.spec.releaseRef,s=t.spec.suspend?`The version checks run when the cluster resumes.`:t.status?.management?.version?void 0:`The version checks run when the cluster reports its version.`,c=r.filter(e=>{let t=e.metadata?.name??``;return t===o||ne(t,a)}).map(e=>{let n=e.metadata?.name??``,r=ie(t,e,a,i),c=s===void 0;return{name:n,version:e.spec.version,isDefault:a?.spec.releases?.default===n,isCurrent:n===o,...c&&r.skippedMinor&&{disabled:`Skips ${r.skippedMinor}. Move to an ${r.skippedMinor} release first.`},...c&&r.downgrade&&r.current&&{downgrade:{from:r.current,to:r.target}}}});return c.sort((e,t)=>dt(t.version,e.version)||e.name.localeCompare(t.name)),{options:c,...s&&{note:s}}}function dt(e,t){let n=re(e)??[-1,-1,-1],r=re(t)??[-1,-1,-1];return n[0]-r[0]||n[1]-r[1]||n[2]-r[2]}function ft(){return(ft=e((()=>{fe(),ae()})))()}function pt({context:e}){let t=w(),{session:n}=t;if(!n)return null;let r=n.claim.spec,i=v(n).spec??{},a=e=>{let n=he(``,e);return t.errors.filter(e=>e.path===n).map(e=>e.message)},o=e=>y(n,e),s=(e,n,i)=>!gt(r,e)&&(n===``||n===i)||n===``?t.reset(e):t.set(e,n),{options:c,note:l}=ut({cluster:e.cluster,claim:n.claim,releases:e.releases,environment:e.environment,backups:e.backups}),u=i.releaseRef||e.cluster.spec.releaseRef||``,d=c.find(e=>e.name===u),f=n=>{s(I,n,e.cluster.spec.releaseRef);let i=c.find(e=>e.name===n)?.downgrade?.to;i?t.set(z,i):r?.allowVersionDowngrade===void 0?t.reset(z):t.set(z,r.allowVersionDowngrade)},m=ht(e,r?.presetRef),h=i.presetRef??``,ee=e.environment?.spec.platformConfigRef,te=i.platformConfigRef??``,ne=t.pending===0&&Ne(e.cluster,n.claim);return(0,F.jsxs)(`div`,{className:`flex flex-col gap-5`,children:[(0,F.jsx)(_t,{label:`Release`,value:u,lock:o(I),errors:a(I),onChange:f,hint:(0,F.jsxs)(F.Fragment,{children:[d?.downgrade&&(0,F.jsxs)(`p`,{className:`text-xs text-warn`,children:[d.downgrade.from,` to `,d.downgrade.to,` is a downgrade. The review asks you to confirm it.`]}),l&&(0,F.jsx)(`p`,{className:`text-xs text-muted`,children:l})]}),children:c.map(e=>(0,F.jsx)(x,{value:e.name,disabled:e.disabled!==void 0&&!e.isCurrent,children:mt(e)},e.name))}),ne&&(0,F.jsxs)(`div`,{className:`flex flex-wrap items-center gap-x-3 gap-y-2 rounded-md border border-border bg-panel-2 px-3 py-2 text-sm text-text-2`,children:[(0,F.jsxs)(`p`,{className:`min-w-0 flex-1`,children:[`The cluster runs `,e.cluster.status?.management?.version,`, so allowVersionDowngrade can go.`]}),(0,F.jsx)(p,{size:`sm`,variant:`outline`,onClick:()=>{t.reset(z),t.review()},children:`Clear`})]}),(0,F.jsx)(_t,{label:`Preset`,value:h,lock:o(L),errors:a(L),onChange:e=>s(L,e),children:m.map(e=>(0,F.jsx)(x,{value:e,children:e},e))}),(0,F.jsxs)(_t,{label:`Platform config`,value:te,lock:o(R),errors:a(R),onChange:e=>s(R,e),children:[(0,F.jsx)(x,{value:``,children:ee?`Default of the environment`:`Not set`}),e.platformConfigs.map(e=>(0,F.jsx)(x,{value:e.metadata?.name,children:e.metadata?.name},e.metadata?.name))]})]})}function mt(e){let t=`${e.version} (${e.name})`;if(e.disabled)return`${t}: ${e.disabled}`;let n=[e.isDefault&&`Default`,e.downgrade&&`downgrade`].filter(Boolean);return n.length>0?`${t}, ${n.join(`, `)}`:t}function ht(e,t){let n=pe(e.presets,e.environment).map(e=>e.metadata.name??``);return t&&!n.includes(t)?[...n,t]:n}function gt(e,t){let n=t[1];return e&&`field`in n?e[n.field]:void 0}function _t({label:e,value:t,lock:n,errors:r,hint:i,onChange:a,children:o}){let s=(0,vt.useId)();return n?(0,F.jsx)(C,{label:e,value:t||`Not set`,reason:_(n)}):(0,F.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1.5`,children:[(0,F.jsx)(`label`,{htmlFor:s,className:`text-sm font-medium text-text`,children:e}),(0,F.jsx)(Ee,{id:s,value:t,"aria-invalid":r.length>0||void 0,"aria-describedby":r.length>0?`${s}-error`:void 0,onChange:e=>a(e.target.value),children:o}),i,r.length>0&&(0,F.jsx)(`p`,{id:`${s}-error`,className:`text-xs text-bad`,children:r.join(` `)})]})}var F,vt,I,L,R,z;function yt(){return(yt=e((()=>{F=n(),vt=t(),S(),f(),Te(),me(),g(),b(),T(),Fe(),ft(),I=[{field:`spec`},{field:`releaseRef`}],L=[{field:`spec`},{field:`presetRef`}],R=[{field:`spec`},{field:`platformConfigRef`}],z=[{field:`spec`},{field:`allowVersionDowngrade`}],pt.__docgenInfo={description:`ReleasePanel edits the release, the preset, and the platform config of
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``}}}})))()}function bt({context:e,section:t}){let{session:n,pending:i,set:a,reset:o,invalid:s,errors:c}=w(),{generation:l,own:u}=je(i),d=e.cluster.metadata?.namespace??``,f=Oe(r(`Secret`,{namespace:d},[]).items);if(!n)return null;let p=e.registrations.find(e=>e.metadata?.name===t.registration),m=v(n),h=m.status?.resolvedPolicy?.parameters?.[t.registration]??{},ee=(m.spec?.plugins??[]).find(e=>e.section===t.section&&e.registration===t.registration),te=Object.fromEntries(Object.entries(h).map(([e,t])=>[e,t.allowed])),ne=t.parameters.some(e=>!e.lock&&e.claimSteps);return(0,B.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[(0,B.jsx)(C,{label:`Option`,value:t.optionLabel,reason:_({reason:`chosenAtCreation`})}),ne?(0,B.jsx)(Re,{fields:t.parameters,schema:p?.spec.parameters,allowed:te,claimed:xt(ee?.parameters),secrets:f,errors:c,set:u(a),reset:u(o),invalid:s},l):(0,B.jsx)(`p`,{className:`text-sm text-muted`,children:`No parameters you can change.`})]})}function xt(e){return typeof e==`object`&&e&&!Array.isArray(e)?e:{}}var B;function St(){return(St=e((()=>{B=n(),S(),De(),g(),i(),b(),Le(),T(),ke(),bt.__docgenInfo={description:`SectionPanel edits one selection of the claim: its option is locked as
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``},section:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  id: SectionComponentId;
  section: string;
  sectionLabel: string;
  registration: string;
  optionLabel: string;
  parameters: ClusterField[];
}`,signature:{properties:[{key:`id`,value:{name:`SectionComponentId`,required:!0}},{key:`section`,value:{name:`string`,required:!0}},{key:`sectionLabel`,value:{name:`string`,required:!0}},{key:`registration`,value:{name:`string`,required:!0}},{key:`optionLabel`,value:{name:`string`,required:!0}},{key:`parameters`,value:{name:`Array`,elements:[{name:`ClusterField`}],raw:`ClusterField[]`,required:!0}}]}},description:``}}}})))()}function Ct({context:e,requested:t}){let n=w(),[r,i]=(0,H.useState)(t),[a,o]=(0,H.useState)(t),[s,c]=(0,H.useState)(t),[l,u]=(0,H.useState)();t!==a&&(o(t),i(t),c(t));let d=e.claim;if(!d)return null;let f=n.session,p=Tt(e,d,f);if(!f)return(0,V.jsx)(Se,{view:p,stored:d,now:e.now});let m={},h={};for(let t of wt(e,p)){let e=l?.panel===t.panel?l.on:void 0;t.panel===r?m[t.panel]=(0,V.jsx)(Et,{def:t,scroll:s===t.panel,focus:e===`panel`,onShown:()=>{c(void 0),u(void 0)},onDone:()=>{i(void 0),u({panel:t.panel,on:`button`})}}):h[t.panel]=(0,V.jsx)(Dt,{name:t.name,focus:e===`button`,onFocused:()=>u(void 0),onClick:()=>{i(t.panel),u({panel:t.panel,on:`panel`})}})}return(0,V.jsx)(Se,{view:p,stored:d,now:e.now,edit:{panels:m,actions:h}})}function wt(e,t){return[{panel:`basics`,label:`Basics`,name:`Basics`,render:()=>(0,V.jsx)(nt,{view:t})},{panel:`release`,label:`Release and preset`,name:`Release and preset`,render:()=>(0,V.jsx)(pt,{context:e})},...t.sections.map(t=>({panel:t.id,label:t.sectionLabel,name:`${t.sectionLabel}: ${t.optionLabel}`,render:()=>(0,V.jsx)(bt,{context:e,section:t})})),{panel:`attachments`,label:`Attachments`,name:`Attachments`,render:()=>(0,V.jsx)(Ue,{context:e})},{panel:`deletion`,label:`Deletion policy`,name:`Deletion policy`,render:()=>(0,V.jsx)(at,{context:e})}]}function Tt(e,t,n){if(!n)return ye({claim:t,cluster:e.cluster,merged:e.merged,environments:e.environments,sections:e.sections,registrations:e.registrations});let r=ge(Ie(e,n)).after,{releaseRef:i,platformConfigRef:a}=r.spec;return ye({claim:v(n),cluster:{...e.cluster,spec:{...e.cluster.spec,releaseRef:i,platformConfigRef:a}},merged:r,environments:e.environments,sections:e.sections,registrations:e.registrations})}function Et({def:e,scroll:t,focus:n,onShown:r,onDone:i}){let a=(0,H.useRef)(null);return(0,H.useEffect)(()=>{(t||n)&&(t&&a.current?.scrollIntoView({block:`start`}),n&&a.current?.focus(),r())},[t,n,r]),(0,V.jsx)(be,{ref:a,label:e.label,name:e.name,editing:!0,action:(0,V.jsx)(p,{size:`sm`,variant:`outline`,onClick:i,children:`Done`}),children:e.render()})}function Dt({name:e,focus:t,onFocused:n,onClick:r}){let i=(0,H.useRef)(null);return(0,H.useEffect)(()=>{t&&(i.current?.focus(),n())},[t,n]),(0,V.jsxs)(p,{ref:i,size:`sm`,variant:`ghost`,"aria-label":`Edit ${e}`,onClick:r,children:[(0,V.jsx)(Be,{"aria-hidden":!0}),`Edit`]})}var V,H;function Ot(){return(Ot=e((()=>{V=n(),Ve(),s(),H=t(),f(),b(),ve(),xe(),_e(),He(),tt(),it(),T(),lt(),yt(),St(),Ct.__docgenInfo={description:`ClaimEditorPanels is the Claim tab with an Edit button on each panel that
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``},requested:{required:!1,tsType:{name:`union`,raw:'"basics" | "release" | "attachments" | "deletion" | `section:${string}`',elements:[{name:`literal`,value:`"basics"`},{name:`literal`,value:`"release"`},{name:`literal`,value:`"attachments"`},{name:`literal`,value:`"deletion"`},{name:`literal`,value:"`section:${string}`"}]},description:``}}}})))()}function U({version:e=`8.9.3`,suspended:t=!1,claimSpec:n={},claimStatus:r={},over:i={}}){let o={namespace:`team-payments`,name:`orders`},s={presetRef:`medium`,releaseRef:`camunda-8-9-3`,zeebe:{replicas:3},...t&&{suspend:!0}},c={apiVersion:`core.camunda.io/v1`,kind:`CamundaCluster`,metadata:{...o,labels:{[a]:`orders`}},spec:s,status:e?{management:{version:e}}:{}},l={apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,metadata:{...o,resourceVersion:`41`},spec:{presetRef:`medium`,releaseRef:`camunda-8-9-3`,cluster:{zeebe:{replicas:3}},...t&&{suspend:!0},...n},status:{environment:`prod`,...r}},u={kind:`Environment`,metadata:{name:`prod`},spec:{displayName:`Production`,presets:{camundaCluster:[`medium`,`large`]},releases:{allowed:[`camunda-8-9-1`,`camunda-8-9-3`,`camunda-8-11-0`],default:`camunda-8-9-3`}}},d=[G(`8.9.1`),G(`8.9.3`),G(`8.11.0`)],f=[K(`medium`),K(`large`)];return{cluster:c,claim:l,merged:se(f[0].spec,d[1].spec,s),environment:u,environments:[u],presets:f,releases:d,platformConfigs:[],managementAuthConfigs:[],sections:[],registrations:[],requests:[],backups:[],restores:[],backupStorage:{kind:`none`,schedules:[]},schedules:[],fields:[],writeMode:`direct`,now:`2026-11-10T12:00:00Z`,...i}}var W,kt,At,jt,Mt,Nt,G,K,Pt,Ft,It,Lt,q,J,Y,X,Z,Q,$,Rt;function zt(){return(zt=e((()=>{W=n(),o(),ce(),Ot(),T(),{expect:kt,fn:At,userEvent:jt,within:Mt}=__STORYBOOK_MODULE_TEST__,Nt={preview:At(async()=>({objects:[],errors:[]})),apply:At(async()=>({ok:!0,mode:`direct`}))},G=e=>({kind:`CamundaRelease`,metadata:{name:`camunda-${e.replaceAll(`.`,`-`)}`},spec:{version:e}}),K=e=>({kind:`CamundaClusterPreset`,metadata:{name:e},spec:{cluster:{}}}),Pt={claimSpec:{plugins:[{section:`access`,registration:`gateway-access`,parameters:{hostPrefix:`orders`,sources:[`office`]}}]},claimStatus:{resolvedPolicy:{sections:{access:{allowed:[`gateway-access`]}},parameters:{"gateway-access":{hostPrefix:{claimEditable:!0,value:`camunda`},zone:{value:`payments.example.com`},sources:{claimEditable:!0,allowed:[{name:`office`,label:`Office`,value:`10.0.0.0/16`},{name:`vpn`,label:`VPN`,value:`10.8.0.0/16`}]}}}}},over:{sections:[{kind:`PluginSection`,metadata:{name:`access`},spec:{label:`Access`,selection:`One`,appliesTo:[`CamundaClusterClaim`]}}],registrations:[{kind:`PluginRegistration`,metadata:{name:`gateway-access`},spec:{label:`Gateway`,deletionPolicies:[`Delete`],parameters:{type:`object`,properties:{hostPrefix:{type:`string`,description:`The first label of each host name.`},zone:{type:`string`},sources:{type:`array`,items:{type:`string`}}}}}}]}},Ft={claimSpec:{attachments:{optimize:{managementAuthConfigRef:`keycloak`,externalUrl:`https://optimize.payments.example.com`},backupSchedule:{schedule:`0 2 * * *`,retained:{completed:7}}}},over:{storage:{kind:`SecondaryStorageConfig`,metadata:{namespace:`team-payments`,name:`orders-storage`},spec:{type:`elasticsearch`}},managementAuthConfigs:[`camunda`,`keycloak`].map(e=>({kind:`ManagementAuthConfig`,metadata:{name:e},spec:{}})),backupStorage:{kind:`elasticsearch`,bucket:{name:`backups`,location:`s3://acme-backups/orders`},repository:`camunda-backups`,schedules:[]}}},It=class e{static{this.CLOSED=2}addEventListener(){}close(){this.readyState=e.CLOSED}constructor(){this.readyState=0}},Lt={title:`cluster/edit/Claim editor`,component:Ct,args:{context:U({})},beforeEach:()=>{let e=globalThis.EventSource;return globalThis.EventSource=It,()=>{globalThis.EventSource=e}},decorators:[(e,{args:t})=>(0,W.jsx)(`div`,{className:`max-w-5xl`,children:(0,W.jsx)(Pe,{context:t.context,actions:Nt,children:(0,W.jsx)(e,{})},JSON.stringify(t.context))})]},q={name:`Read view with Edit buttons`},J={name:`Basics in edit`,args:{requested:`basics`},play:async({canvasElement:e})=>{let t=Mt(e).getByRole(`checkbox`,{name:`Suspend the cluster`});await jt.click(t),await kt(t).toBeChecked()}},Y={name:`Release with a downgrade`,args:{requested:`release`},play:async({canvasElement:e})=>{let t=Mt(e);await jt.selectOptions(t.getByRole(`combobox`,{name:`Release`}),`camunda-8-9-1`),await kt(t.getByText(/is a downgrade\./)).toBeVisible()}},X={name:`Release while suspended`,args:{context:U({version:``,suspended:!0}),requested:`release`}},Z={name:`A section with an editable parameter`,args:{context:U(Pt),requested:`section:access`}},Q={name:`Attachments in edit`,args:{context:U(Ft),requested:`attachments`}},$={name:`Deletion with Keep refused`,args:{context:U(Pt),requested:`deletion`}},Rt=[`ReadViewWithEditButtons`,`BasicsInEdit`,`ReleaseWithADowngrade`,`ReleaseWhileSuspended`,`ASectionWithAnEditableParameter`,`AttachmentsInEdit`,`DeletionWithKeepRefused`],q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
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
}`,...$.parameters?.docs?.source}}}})))()}zt();export{Z as ASectionWithAnEditableParameter,Q as AttachmentsInEdit,J as BasicsInEdit,$ as DeletionWithKeepRefused,q as ReadViewWithEditButtons,X as ReleaseWhileSuspended,Y as ReleaseWithADowngrade,Rt as __namedExportsOrder,Lt as default};