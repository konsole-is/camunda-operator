import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{a as r,t as i}from"./hooks-Cuq9NGnJ.js";import{a,h as o}from"./labels-a4ptf10d.js";import{n as s,t as c}from"./createLucideIcon-B55dJ-eE.js";import{l}from"./iframe-DoYLAcwb.js";import{n as u,t as d}from"./link-DOLMGIR2.js";import{n as f,t as p}from"./checkbox-BurB0-jk.js";import{r as m,t as h}from"./button-DF6UHiMI.js";import{n as ee,t as g}from"./input-BK5lfukz.js";import{l as te,n as ne,r as re,t as ie,y as ae}from"./release-B_s19T_Z.js";import{c as oe,t as se}from"./merge-B877RDyT.js";import{m as ce,p as le,s as ue,t as de}from"./policy-UrtrXbln.js";import{a as fe,i as pe}from"./draft-W6Wspu-a.js";import{C as me,t as _}from"./fields-D4ttTZ0d.js";import{n as v}from"./labels-LUZ4_yzX.js";import{f as y,p as b,t as x}from"./session-_5lMhwPU.js";import{a as he,i as ge}from"./backup-caution-BPAYddiT.js";import{a as _e,i as ve,n as ye,r as be,t as xe}from"./claim-tab-Bhg4FBGS.js";import{i as Se,n as Ce}from"./deletion-choice-CGKHPBat.js";import{n as S,r as we,t as Te}from"./native-select-BsKUlQL2.js";import{n as C,t as w}from"./locked-field-Bczcj0Gm.js";import{n as Ee,r as De}from"./secret-ref-picker-euOpgowN.js";import{c as Oe,i as ke,l as Ae,r as je}from"./settings-editor-CbHED4ut.js";import{a as Me,i as T,n as Ne,o as Pe,r as E,s as Fe}from"./cluster-edit-CD1m_yJY.js";import{n as Ie,t as Le}from"./selection-settings-editor-0BKhBJRW.js";var Re,ze;function Be(){return(Be=e((()=>{s(),Re={name:`pencil`,size:24,node:[[`path`,{d:`M21.174 6.812a1 1 0 0 0-3.986-3.987L3.842 16.174a2 2 0 0 0-.5.83l-1.321 4.352a.5.5 0 0 0 .623.622l4.353-1.32a2 2 0 0 0 .83-.497z`,key:`1a8usu`}],[`path`,{d:`m15 5 4 4`,key:`1mk7zo`}]]},Re.node,ze=c(Re)})))()}function Ve(){return(Ve=e((()=>{})))()}function He({context:e}){let t=T(),{session:n}=t,{generation:r,own:i}=Ae(t.pending);if(!n)return null;let a=y(n).spec?.attachments,o=a?.optimize,s=a?.backupSchedule,c=e.managementAuthConfigs.map(e=>e.metadata?.name??``).sort(ae),l=e.storage?.spec.type===`elasticsearch`?c.length===0?`No ManagementAuthConfig exists. Your admin creates one first.`:void 0:`Optimize needs Elasticsearch as secondary storage.`,d=e.backupStorage.kind===`none`?`Needs a backup storage option.`:void 0,f=b(n,j),p=e=>je(t.errors,e).map(e=>e.message),m=e=>{let n=me(``,e);return t.errors.filter(e=>e.path===n).map(e=>e.message)};return(0,D.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,D.jsx)(Ue,{label:`Attach Optimize`,checked:o!==void 0,off:o===void 0?l:void 0,lock:b(n,A),errors:m(A),onChange:e=>e?t.set(A,{managementAuthConfigRef:c[0]}):t.reset(A),children:o&&(0,D.jsxs)(D.Fragment,{children:[(0,D.jsx)(We,{label:`ManagementAuthConfig`,errors:p(Ye),children:e=>(0,D.jsx)(Te,{...e,value:o.managementAuthConfigRef,onChange:e=>t.set(Ye,e.target.value),children:[...new Set([o.managementAuthConfigRef,...c])].sort(ae).map(e=>(0,D.jsx)(S,{value:e,children:e},e))})}),f?(0,D.jsx)(w,{label:`External URL`,value:o.externalUrl??e.optimize?.spec.externalUrl??`Not set`,reason:v(f)}):(0,D.jsx)(We,{label:`External URL`,errors:p(j),children:e=>(0,D.jsx)(g,{...e,type:`url`,spellCheck:!1,placeholder:`https://optimize.example.com`,value:o.externalUrl??``,onChange:e=>e.target.value===``?t.reset(j):t.set(j,e.target.value)})}),e.optimize&&(0,D.jsx)(u,{href:`?tab=overview&component=optimize`,className:`self-start text-sm font-medium text-text underline underline-offset-4 hover:text-accent`,children:`Open Optimize`})]})}),(0,D.jsx)(Ue,{label:`Back up on a schedule`,checked:s!==void 0,off:s===void 0?d:void 0,lock:b(n,M),errors:m(M),onChange:e=>e?t.set(M,{schedule:$e}):t.reset(M),children:s&&(0,D.jsxs)(D.Fragment,{children:[(0,D.jsx)(We,{label:`Schedule (cron, UTC)`,errors:p(Xe),children:e=>(0,D.jsx)(g,{...e,className:`max-w-56 font-mono`,spellCheck:!1,value:s.schedule,onChange:e=>t.set(Xe,e.target.value)})}),(0,D.jsxs)(`div`,{className:`flex flex-wrap gap-4`,children:[(0,D.jsx)(Ke,{label:`Completed backups to keep`,steps:Ze,minimum:1,value:s.retained?.completed,errors:p(Ze),set:i(t.set),reset:i(t.reset)}),(0,D.jsx)(Ke,{label:`Failed backups to keep`,steps:Qe,minimum:0,value:s.retained?.failed,errors:p(Qe),set:i(t.set),reset:i(t.reset)})]},r)]})})]})}function Ue({label:e,checked:t,off:n,lock:r,errors:i,onChange:a,children:o}){let s=(0,O.useId)();return(0,D.jsxs)(`fieldset`,{className:`flex min-w-0 flex-col gap-3`,children:[(0,D.jsxs)(`div`,{className:`flex items-start gap-3`,children:[(0,D.jsx)(p,{id:s,className:`mt-0.5`,checked:t,disabled:n!==void 0||r!==void 0,"aria-describedby":n||r?`${s}-note`:void 0,onCheckedChange:e=>a(e===!0)}),(0,D.jsxs)(`div`,{className:`flex flex-col gap-0.5`,children:[(0,D.jsx)(`label`,{htmlFor:s,className:`text-sm font-medium text-text`,children:e}),(n||r)&&(0,D.jsx)(`p`,{id:`${s}-note`,className:`text-xs text-muted`,children:n??(r&&v(r))}),(0,D.jsx)(Ge,{errors:i})]})]}),t&&(0,D.jsx)(`div`,{className:`ml-7 flex flex-col gap-4 border-l border-divider pl-4`,children:o})]})}function We({label:e,errors:t,children:n}){let r=(0,O.useId)(),i=t.length>0;return(0,D.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1.5`,children:[(0,D.jsx)(`label`,{htmlFor:r,className:`text-sm font-medium text-text`,children:e}),n({id:r,...i&&{"aria-invalid":!0,"aria-describedby":`${r}-error`}}),(0,D.jsx)(Ge,{id:`${r}-error`,errors:t})]})}function Ge({id:e,errors:t}){return t.length===0?null:(0,D.jsx)(`p`,{id:e,className:`text-xs text-bad`,children:t.join(` `)})}function Ke({label:e,steps:t,minimum:n,value:r,errors:i,set:a,reset:o}){let{invalid:s}=T(),{generation:c,own:l}=Ae(0,r);return(0,O.useEffect)(()=>()=>s(t,void 0),[s,t]),(0,D.jsx)(qe,{label:e,minimum:n,value:r,errors:i,onChange:l(r=>{let i=Je(r,n);s(t,i&&`${e}: ${i}`),i||(r===``?o(t):a(t,Number(r)))})},c)}function qe({label:e,minimum:t,value:n,errors:r,onChange:i}){let a=(0,O.useId)(),[o,s]=(0,O.useState)(n===void 0?``:String(n)),c=Je(o,t)??(r.join(` `)||void 0);return(0,D.jsxs)(`div`,{className:`flex flex-col gap-1.5`,children:[(0,D.jsx)(`label`,{htmlFor:a,className:`text-sm font-medium text-text`,children:e}),(0,D.jsx)(g,{id:a,type:`number`,min:t,step:1,className:`w-32`,value:o,"aria-invalid":c?!0:void 0,"aria-describedby":c?`${a}-error`:void 0,onChange:e=>{s(e.target.value),i(e.target.value)}}),c&&(0,D.jsx)(`p`,{id:`${a}-error`,className:`text-xs text-bad`,children:c})]})}function Je(e,t){if(e===``)return;let n=Number(e);if(!Number.isInteger(n))return`Enter a whole number`;if(n<t)return`Enter ${t} or more`}var D,O,k,A,Ye,j,M,Xe,Ze,Qe,$e;function et(){return(et=e((()=>{D=n(),d(),O=t(),C(),f(),ee(),we(),_(),de(),x(),ke(),E(),Oe(),k=e=>({field:e}),A=[k(`spec`),k(`attachments`),k(`optimize`)],Ye=[...A,k(`managementAuthConfigRef`)],j=[...A,k(`externalUrl`)],M=[k(`spec`),k(`attachments`),k(`backupSchedule`)],Xe=[...M,k(`schedule`)],Ze=[...M,k(`retained`),k(`completed`)],Qe=[...M,k(`retained`),k(`failed`)],$e=`0 2 * * *`,He.__docgenInfo={description:`AttachmentsPanel edits the attachments of the claim. Attach Optimize
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``}}}})))()}function tt({view:e}){let{session:t,set:n,reset:r}=T(),i=(0,nt.useId)();if(!t)return null;let a=v({reason:`atCreation`}),o=b(t,P),s=e=>{!e&&t.claim.spec?.suspend===void 0?r(P):n(P,e)};return(0,N.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[(0,N.jsxs)(`div`,{className:`grid gap-4 sm:grid-cols-3`,children:[(0,N.jsx)(w,{label:`Name`,value:e.name,reason:a}),(0,N.jsx)(w,{label:`Namespace`,value:e.namespace,reason:a}),(0,N.jsx)(w,{label:`Environment`,value:e.environment??`No environment`,reason:a})]}),o?(0,N.jsx)(w,{label:`Suspend the cluster`,value:e.suspend?`Yes`:`No`,reason:v(o)}):(0,N.jsxs)(`div`,{className:`flex items-start gap-3`,children:[(0,N.jsx)(p,{id:i,className:`mt-0.5`,checked:e.suspend,"aria-describedby":`${i}-hint`,onCheckedChange:e=>s(e===!0)}),(0,N.jsxs)(`div`,{className:`flex flex-col gap-0.5`,children:[(0,N.jsx)(`label`,{htmlFor:i,className:`text-sm font-medium text-text`,children:`Suspend the cluster`}),(0,N.jsx)(`p`,{id:`${i}-hint`,className:`text-xs text-muted`,children:`Every workload goes to zero. The volumes stay.`})]})]})]})}var N,nt,P;function rt(){return(rt=e((()=>{N=n(),nt=t(),C(),f(),_(),x(),E(),P=[{field:`spec`},{field:`suspend`}],tt.__docgenInfo={description:`BasicsPanel edits the Basics of the claim: the name, the namespace, and
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`sectionLabel`,value:{name:`string`,required:!0}},{key:`optionLabel`,value:{name:`string`,required:!0}},{key:`outputsReady`,value:{name:`string`,required:!1}},{key:`ready`,value:{name:`string`,required:!1}}]}}],raw:`ClaimRequestRow[]`,required:!0}}]}},description:``}}}})))()}function it({context:e}){let t=T();if(!t.session)return null;let n=at(e,t.session),r=n.policy===`Orphan`&&!n.keepAllowed,i=t.session.claim.spec?.deletionPolicy;return(0,F.jsxs)(`div`,{className:`flex flex-col gap-3`,children:[(0,F.jsx)(Ce,{subject:`cluster`,choice:n,onChange:e=>e===n.defaultPolicy&&!i?t.reset(ot):t.set(ot,e)}),r&&(0,F.jsx)(`p`,{className:`text-sm text-warn`,children:st})]})}function at(e,t){let n=y(t),r=n.spec?.plugins??[],i=e.registrations,a=e.environment,o=e.sections.map(e=>ue(e,i,a,`CamundaClusterClaim`)),s=n.status?.resolvedPolicy;return{...le(n.spec?.deletionPolicy,r,a,i,o,ce(void 0,s)),policy:ce(n.spec?.deletionPolicy,s)}}var F,ot,st;function ct(){return(ct=e((()=>{F=n(),Se(),de(),x(),E(),ot=[{field:`spec`},{field:`deletionPolicy`}],st=`The claim keeps its resources, but an option below cannot keep them. Choose Delete, or remove that option.`,it.__docgenInfo={description:`DeletionPanel edits the deletion policy of the claim. Choosing the
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``}}}})))()}function lt(e){let{cluster:t,claim:n,releases:r,backups:i}=e,a=e.environment,o=n.spec?.releaseRef||t.spec.releaseRef,s=t.spec.suspend?`The version checks run when the cluster resumes.`:t.status?.management?.version?void 0:`The version checks run when the cluster reports its version.`,c=r.filter(e=>{let t=e.metadata?.name??``;return t===o||te(t,a)}).map(e=>{let n=e.metadata?.name??``,r=re(t,e,a,i),c=s===void 0;return{name:n,version:e.spec.version,isDefault:a?.spec.releases?.default===n,isCurrent:n===o,...c&&r.skippedMinor&&{disabled:`Skips ${r.skippedMinor}. Move to an ${r.skippedMinor} release first.`},...c&&r.downgrade&&r.current&&{downgrade:{from:r.current,to:r.target}}}});return c.sort((e,t)=>ut(t.version,e.version)||e.name.localeCompare(t.name)),{options:c,...s&&{note:s}}}function ut(e,t){let n=ne(e)??[-1,-1,-1],r=ne(t)??[-1,-1,-1];return n[0]-r[0]||n[1]-r[1]||n[2]-r[2]}function dt(){return(dt=e((()=>{de(),ie()})))()}function ft({context:e}){let t=T(),{session:n}=t;if(!n)return null;let r=n.claim.spec,i=y(n).spec??{},a=e=>{let n=me(``,e);return t.errors.filter(e=>e.path===n).map(e=>e.message)},o=e=>b(n,e),s=(e,n,i)=>!ht(r,e)&&(n===``||n===i)||n===``?t.reset(e):t.set(e,n),{options:c,note:l}=lt({cluster:e.cluster,claim:n.claim,releases:e.releases,environment:e.environment,backups:e.backups}),u=i.releaseRef||e.cluster.spec.releaseRef||``,d=c.find(e=>e.name===u),f=n=>{s(L,n,e.cluster.spec.releaseRef);let i=c.find(e=>e.name===n)?.downgrade?.to;i?t.set(B,i):r?.allowVersionDowngrade===void 0?t.reset(B):t.set(B,r.allowVersionDowngrade)},p=mt(e,r?.presetRef),m=i.presetRef??``,ee=e.environment?.spec.platformConfigRef,g=i.platformConfigRef??``,te=t.pending===0&&Me(e.cluster,n.claim);return(0,I.jsxs)(`div`,{className:`flex flex-col gap-5`,children:[(0,I.jsx)(gt,{label:`Release`,value:u,lock:o(L),errors:a(L),onChange:f,hint:(0,I.jsxs)(I.Fragment,{children:[d?.downgrade&&(0,I.jsxs)(`p`,{className:`text-xs text-warn`,children:[d.downgrade.from,` to `,d.downgrade.to,` is a downgrade. The review asks you to confirm it.`]}),l&&(0,I.jsx)(`p`,{className:`text-xs text-muted`,children:l})]}),children:c.map(e=>(0,I.jsx)(S,{value:e.name,disabled:e.disabled!==void 0&&!e.isCurrent,children:pt(e)},e.name))}),te&&(0,I.jsxs)(`div`,{className:`flex flex-wrap items-center gap-x-3 gap-y-2 rounded-md border border-border bg-panel-2 px-3 py-2 text-sm text-text-2`,children:[(0,I.jsxs)(`p`,{className:`min-w-0 flex-1`,children:[`The cluster runs `,e.cluster.status?.management?.version,`, so allowVersionDowngrade can go.`]}),(0,I.jsx)(h,{size:`sm`,variant:`outline`,onClick:()=>{t.reset(B),t.review()},children:`Clear`})]}),(0,I.jsx)(gt,{label:`Preset`,value:m,lock:o(R),errors:a(R),onChange:e=>s(R,e),children:p.map(e=>(0,I.jsx)(S,{value:e,children:e},e))}),(0,I.jsxs)(gt,{label:`Platform config`,value:g,lock:o(z),errors:a(z),onChange:e=>s(z,e),children:[(0,I.jsx)(S,{value:``,children:ee?`Default of the environment`:`Not set`}),e.platformConfigs.map(e=>(0,I.jsx)(S,{value:e.metadata?.name,children:e.metadata?.name},e.metadata?.name))]})]})}function pt(e){let t=`${e.version} (${e.name})`;if(e.disabled)return`${t}: ${e.disabled}`;let n=[e.isDefault&&`Default`,e.downgrade&&`downgrade`].filter(Boolean);return n.length>0?`${t}, ${n.join(`, `)}`:t}function mt(e,t){let n=fe(e.presets,e.environment).map(e=>e.metadata.name??``);return t&&!n.includes(t)?[...n,t]:n}function ht(e,t){let n=t[1];return e&&`field`in n?e[n.field]:void 0}function gt({label:e,value:t,lock:n,errors:r,hint:i,onChange:a,children:o}){let s=(0,_t.useId)();return n?(0,I.jsx)(w,{label:e,value:t||`Not set`,reason:v(n)}):(0,I.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1.5`,children:[(0,I.jsx)(`label`,{htmlFor:s,className:`text-sm font-medium text-text`,children:e}),(0,I.jsx)(Te,{id:s,value:t,"aria-invalid":r.length>0||void 0,"aria-describedby":r.length>0?`${s}-error`:void 0,onChange:e=>a(e.target.value),children:o}),i,r.length>0&&(0,I.jsx)(`p`,{id:`${s}-error`,className:`text-xs text-bad`,children:r.join(` `)})]})}var I,_t,L,R,z,B;function vt(){return(vt=e((()=>{I=n(),_t=t(),C(),m(),we(),pe(),_(),x(),E(),Pe(),dt(),L=[{field:`spec`},{field:`releaseRef`}],R=[{field:`spec`},{field:`presetRef`}],z=[{field:`spec`},{field:`platformConfigRef`}],B=[{field:`spec`},{field:`allowVersionDowngrade`}],ft.__docgenInfo={description:`ReleasePanel edits the release, the preset, and the platform config of
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``}}}})))()}function yt({context:e,section:t}){let{session:n,pending:i,set:a,reset:o,invalid:s,errors:c}=T(),{generation:l,own:u}=Ae(i),d=e.cluster.metadata?.namespace??``,f=De(r(`Secret`,{namespace:d},[]).items);if(!n)return null;let p=e.registrations.find(e=>e.metadata?.name===t.registration),m=y(n),h=m.status?.resolvedPolicy?.parameters?.[t.registration]??{},ee=(m.spec?.plugins??[]).find(e=>e.section===t.section&&e.registration===t.registration),g=Object.fromEntries(Object.entries(h).map(([e,t])=>[e,t.allowed])),te=t.parameters.some(e=>!e.lock&&e.claimSteps);return(0,V.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[(0,V.jsx)(w,{label:`Option`,value:t.optionLabel,reason:v({reason:`chosenAtCreation`})}),te?(0,V.jsx)(Le,{fields:t.parameters,schema:p?.spec.parameters,allowed:g,claimed:bt(ee?.parameters),secrets:f,errors:c,set:u(a),reset:u(o),invalid:s},l):(0,V.jsx)(`p`,{className:`text-sm text-muted`,children:`No parameters you can change.`})]})}function bt(e){return typeof e==`object`&&e&&!Array.isArray(e)?e:{}}var V;function xt(){return(xt=e((()=>{V=n(),C(),Ee(),_(),i(),x(),Ie(),E(),Oe(),yt.__docgenInfo={description:`SectionPanel edits one selection of the claim: its option is locked as
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
}`,signature:{properties:[{key:`id`,value:{name:`SectionComponentId`,required:!0}},{key:`section`,value:{name:`string`,required:!0}},{key:`sectionLabel`,value:{name:`string`,required:!0}},{key:`registration`,value:{name:`string`,required:!0}},{key:`optionLabel`,value:{name:`string`,required:!0}},{key:`parameters`,value:{name:`Array`,elements:[{name:`ClusterField`}],raw:`ClusterField[]`,required:!0}}]}},description:``}}}})))()}function St({context:e,requested:t}){let n=T(),[r,i]=(0,U.useState)(t),[a,o]=(0,U.useState)(t),[s,c]=(0,U.useState)(t),[l,u]=(0,U.useState)();t!==a&&(o(t),i(t),c(t));let d=e.claim;if(!d)return null;let f=n.session,p=wt(e,d,f);if(!f)return(0,H.jsx)(xe,{view:p,stored:d,now:e.now});let m={},h={};for(let t of Ct(e,p)){let e=l?.panel===t.panel?l.on:void 0;t.panel===r?m[t.panel]=(0,H.jsx)(Tt,{def:t,scroll:s===t.panel,focus:e===`panel`,onShown:()=>{c(void 0),u(void 0)},onDone:()=>{i(void 0),u({panel:t.panel,on:`button`})}}):h[t.panel]=(0,H.jsx)(Et,{name:t.name,focus:e===`button`,onFocused:()=>u(void 0),onClick:()=>{i(t.panel),u({panel:t.panel,on:`panel`})}})}return(0,H.jsx)(xe,{view:p,stored:d,now:e.now,edit:{panels:m,actions:h}})}function Ct(e,t){return[{panel:`basics`,label:`Basics`,name:`Basics`,render:()=>(0,H.jsx)(tt,{view:t})},{panel:`release`,label:`Release and preset`,name:`Release and preset`,render:()=>(0,H.jsx)(ft,{context:e})},...t.sections.map(t=>({panel:t.id,label:t.sectionLabel,name:`${t.sectionLabel}: ${t.optionLabel}`,render:()=>(0,H.jsx)(yt,{context:e,section:t})})),{panel:`attachments`,label:`Attachments`,name:`Attachments`,render:()=>(0,H.jsx)(He,{context:e})},{panel:`deletion`,label:`Deletion policy`,name:`Deletion policy`,render:()=>(0,H.jsx)(it,{context:e})}]}function wt(e,t,n){if(!n)return ve({claim:t,cluster:e.cluster,merged:e.merged,environments:e.environments,sections:e.sections,registrations:e.registrations});let r=he(Fe(e,n)).after,{releaseRef:i,platformConfigRef:a}=r.spec;return ve({claim:y(n),cluster:{...e.cluster,spec:{...e.cluster.spec,releaseRef:i,platformConfigRef:a}},merged:r,environments:e.environments,sections:e.sections,registrations:e.registrations})}function Tt({def:e,scroll:t,focus:n,onShown:r,onDone:i}){let a=(0,U.useRef)(null);return(0,U.useEffect)(()=>{(t||n)&&(t&&a.current?.scrollIntoView({block:`start`}),n&&a.current?.focus(),r())},[t,n,r]),(0,H.jsx)(ye,{ref:a,label:e.label,name:e.name,editing:!0,action:(0,H.jsx)(h,{size:`sm`,variant:`outline`,onClick:i,children:`Done`}),children:e.render()})}function Et({name:e,focus:t,onFocused:n,onClick:r}){let i=(0,U.useRef)(null);return(0,U.useEffect)(()=>{t&&(i.current?.focus(),n())},[t,n]),(0,H.jsxs)(h,{ref:i,size:`sm`,variant:`ghost`,"aria-label":`Edit ${e}`,onClick:r,children:[(0,H.jsx)(ze,{"aria-hidden":!0}),`Edit`]})}var H,U;function Dt(){return(Dt=e((()=>{H=n(),Be(),l(),U=t(),m(),x(),_e(),be(),ge(),Ve(),et(),rt(),E(),ct(),vt(),xt(),St.__docgenInfo={description:`ClaimEditorPanels is the Claim tab with an Edit button on each panel that
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``},requested:{required:!1,tsType:{name:`union`,raw:'"basics" | "release" | "attachments" | "deletion" | `section:${string}`',elements:[{name:`literal`,value:`"basics"`},{name:`literal`,value:`"release"`},{name:`literal`,value:`"attachments"`},{name:`literal`,value:`"deletion"`},{name:`literal`,value:"`section:${string}`"}]},description:``}}}})))()}function W({version:e=`8.9.3`,suspended:t=!1,claimSpec:n={},claimStatus:r={},over:i={}}){let o={namespace:`team-payments`,name:`orders`},s={presetRef:`medium`,releaseRef:`camunda-8-9-3`,zeebe:{replicas:3},...t&&{suspend:!0}},c={apiVersion:`core.camunda.io/v1`,kind:`CamundaCluster`,metadata:{...o,labels:{[a]:`orders`}},spec:s,status:e?{management:{version:e}}:{}},l={apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,metadata:{...o,resourceVersion:`41`},spec:{presetRef:`medium`,releaseRef:`camunda-8-9-3`,cluster:{zeebe:{replicas:3}},...t&&{suspend:!0},...n},status:{environment:`prod`,...r}},u={kind:`Environment`,metadata:{name:`prod`},spec:{displayName:`Production`,presets:{camundaCluster:[`medium`,`large`]},releases:{allowed:[`camunda-8-9-1`,`camunda-8-9-3`,`camunda-8-11-0`],default:`camunda-8-9-3`}}},d=[K(`8.9.1`),K(`8.9.3`),K(`8.11.0`)],f=[Nt(`medium`),Nt(`large`)];return{cluster:c,claim:l,merged:oe(f[0].spec,d[1].spec,s),environment:u,environments:[u],presets:f,releases:d,platformConfigs:[],managementAuthConfigs:[],sections:[],registrations:[],requests:[],backups:[],restores:[],backupStorage:{kind:`none`,schedules:[]},schedules:[],fields:[],writeMode:`direct`,now:`2026-11-10T12:00:00Z`,...i}}var G,Ot,kt,At,jt,Mt,K,Nt,Pt,Ft,It,Lt,q,J,Y,X,Z,Q,$,Rt;function zt(){return(zt=e((()=>{G=n(),o(),se(),Dt(),E(),{expect:Ot,fn:kt,userEvent:At,within:jt}=__STORYBOOK_MODULE_TEST__,Mt={preview:kt(async()=>({objects:[],errors:[]})),apply:kt(async()=>({ok:!0,mode:`direct`}))},K=e=>({kind:`CamundaRelease`,metadata:{name:`camunda-${e.replaceAll(`.`,`-`)}`},spec:{version:e}}),Nt=e=>({kind:`CamundaClusterPreset`,metadata:{name:e},spec:{cluster:{}}}),Pt={claimSpec:{plugins:[{section:`access`,registration:`gateway-access`,parameters:{hostPrefix:`orders`,sources:[`office`]}}]},claimStatus:{resolvedPolicy:{sections:{access:{allowed:[`gateway-access`]}},parameters:{"gateway-access":{hostPrefix:{claimEditable:!0,value:`camunda`},zone:{value:`payments.example.com`},sources:{claimEditable:!0,allowed:[{name:`office`,label:`Office`,value:`10.0.0.0/16`},{name:`vpn`,label:`VPN`,value:`10.8.0.0/16`}]}}}}},over:{sections:[{kind:`PluginSection`,metadata:{name:`access`},spec:{label:`Access`,selection:`One`,appliesTo:[`CamundaClusterClaim`]}}],registrations:[{kind:`PluginRegistration`,metadata:{name:`gateway-access`},spec:{label:`Gateway`,deletionPolicies:[`Delete`],parameters:{type:`object`,properties:{hostPrefix:{type:`string`,description:`The first label of each host name.`},zone:{type:`string`},sources:{type:`array`,items:{type:`string`}}}}}}]}},Ft={claimSpec:{attachments:{optimize:{managementAuthConfigRef:`keycloak`,externalUrl:`https://optimize.payments.example.com`},backupSchedule:{schedule:`0 2 * * *`,retained:{completed:7}}}},over:{storage:{kind:`SecondaryStorageConfig`,metadata:{namespace:`team-payments`,name:`orders-storage`},spec:{type:`elasticsearch`}},managementAuthConfigs:[`camunda`,`keycloak`].map(e=>({kind:`ManagementAuthConfig`,metadata:{name:e},spec:{}})),backupStorage:{kind:`elasticsearch`,bucket:{name:`backups`,location:`s3://acme-backups/orders`},repository:`camunda-backups`,schedules:[]}}},It=class e{static{this.CLOSED=2}addEventListener(){}close(){this.readyState=e.CLOSED}constructor(){this.readyState=0}},Lt={title:`cluster/edit/Claim editor`,component:St,args:{context:W({})},beforeEach:()=>{let e=globalThis.EventSource;return globalThis.EventSource=It,()=>{globalThis.EventSource=e}},decorators:[(e,{args:t})=>(0,G.jsx)(`div`,{className:`max-w-5xl`,children:(0,G.jsx)(Ne,{context:t.context,actions:Mt,children:(0,G.jsx)(e,{})},JSON.stringify(t.context))})]},q={name:`Read view with Edit buttons`},J={name:`Basics in edit`,args:{requested:`basics`},play:async({canvasElement:e})=>{let t=jt(e).getByRole(`checkbox`,{name:`Suspend the cluster`});await At.click(t),await Ot(t).toBeChecked()}},Y={name:`Release with a downgrade`,args:{requested:`release`},play:async({canvasElement:e})=>{let t=jt(e);await At.selectOptions(t.getByRole(`combobox`,{name:`Release`}),`camunda-8-9-1`),await Ot(t.getByText(/is a downgrade\./)).toBeVisible()}},X={name:`Release while suspended`,args:{context:W({version:``,suspended:!0}),requested:`release`}},Z={name:`A section with an editable parameter`,args:{context:W(Pt),requested:`section:access`}},Q={name:`Attachments in edit`,args:{context:W(Ft),requested:`attachments`}},$={name:`Deletion with Keep refused`,args:{context:W(Pt),requested:`deletion`}},Rt=[`ReadViewWithEditButtons`,`BasicsInEdit`,`ReleaseWithADowngrade`,`ReleaseWhileSuspended`,`ASectionWithAnEditableParameter`,`AttachmentsInEdit`,`DeletionWithKeepRefused`],q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
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