import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{n as r,t as i}from"./link-DOLMGIR2.js";import{a,c as o,n as s,o as c,r as l,s as u,t as d}from"./dialog-HNjn3AGC.js";import{n as f,t as p}from"./input-tSe8UiRP.js";import{n as m,r as h,t as g}from"./button-D-7ck_wu.js";import{n as _,t as v}from"./inline-confirm-DWc7C2D_.js";import{r as y,t as ee}from"./typed-confirm-Crh-mlvh.js";import{o as b,s as x}from"./units-B_VbZtdu.js";import{_ as S,b as C,c as w,d as T,f as te,g as ne,h as re,i as E,l as D,m as ie,n as ae,o as O,p as oe,r as se,s as ce,t as le,u as ue,v as de,y as fe}from"./backup-actions-C_Ya9GDt.js";function k({context:e,apply:t,show:n}){let i=(0,L.useId)(),a=ae(e),o=T(e),[s,c]=(0,L.useState)(),[l,u]=(0,L.useTransition)(),d=s!==void 0,f=d?E(s,e.backupNames):[],h=()=>{if(s===void 0||a.kind===void 0)return;let r=se(e,a.kind,s);u(async()=>{let i=await t(P(e),r).catch(F);n(i,`Backup ${s} created.`),i.ok&&c(void 0)})};return(0,I.jsxs)(`div`,{className:`flex flex-col items-end gap-3`,children:[(0,I.jsxs)(`div`,{className:`flex flex-wrap items-center justify-end gap-x-3 gap-y-2`,children:[(0,I.jsx)(`p`,{className:`text-xs text-muted`,children:le}),(0,I.jsx)(r,{href:`?tab=claim&section=attachments`,className:m({variant:`outline`,size:`sm`}),children:`Change schedule`}),o&&(0,I.jsx)(me,{context:e,state:o,reasonId:`${i}-point-reason`,apply:t,show:n}),!d&&(0,I.jsx)(g,{size:`sm`,disabled:a.disabled!==void 0,"aria-describedby":a.disabled?`${i}-reason`:void 0,onClick:()=>c(ue(e.cluster,e.now)),children:`Back up now`})]}),o&&`disabled`in o&&(0,I.jsx)(`p`,{id:`${i}-point-reason`,className:`max-w-md text-right text-xs text-muted`,children:o.disabled}),a.disabled&&(0,I.jsx)(`p`,{id:`${i}-reason`,className:`max-w-md text-right text-xs text-muted`,children:a.disabled}),d&&(0,I.jsxs)(`form`,{"aria-label":`Back up now`,className:`flex w-full max-w-md flex-col gap-3 rounded-md border border-border bg-panel-2/50 p-3 text-sm`,onSubmit:e=>{e.preventDefault(),h()},children:[(0,I.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,I.jsx)(`label`,{htmlFor:`${i}-name`,className:`font-medium`,children:`Name`}),(0,I.jsx)(p,{id:`${i}-name`,value:s,autoFocus:!0,autoComplete:`off`,spellCheck:!1,"aria-invalid":f.length>0||void 0,"aria-describedby":f.length>0?`${i}-errors`:void 0,onChange:e=>c(e.target.value)}),f.length>0&&(0,I.jsx)(`ul`,{id:`${i}-errors`,className:`flex flex-col gap-0.5 text-xs text-bad`,children:f.map(e=>(0,I.jsx)(`li`,{children:e},e))})]}),a.waits&&(0,I.jsx)(`p`,{className:`text-xs text-info`,children:a.waits}),(0,I.jsxs)(`div`,{className:`flex flex-wrap gap-2`,children:[(0,I.jsx)(g,{type:`submit`,size:`sm`,disabled:f.length>0||l||a.disabled!==void 0,"aria-describedby":a.disabled?`${i}-reason`:void 0,children:`Start backup`}),(0,I.jsx)(g,{type:`button`,size:`sm`,variant:`ghost`,onClick:()=>c(void 0),children:`Cancel`})]})]})]})}function A({context:e,row:t,apply:n,show:r}){return(0,I.jsxs)(`div`,{className:`flex flex-wrap items-start gap-2`,children:[t.phase===`Completed`&&(0,I.jsx)(pe,{context:e,row:t,apply:n,show:r}),(0,I.jsx)(N,{context:e,row:t,apply:n,show:r})]})}function pe({context:e,row:t,apply:n,show:r}){let i=re(e,t);return(0,I.jsx)(j,{trigger:`Restore`,title:`Restore ${e.cluster}`,description:`From backup ${t.name}.`,block:i,context:e,write:n=>ne(e,t,n),what:`restore`,apply:n,show:r,children:(0,I.jsx)(M,{context:e,...S(e,t)})})}function me({context:e,state:t,reasonId:n,apply:r,show:i}){let[a,o]=(0,L.useState)(``),s=(0,L.useId)(),c=`intervals`in t?t.intervals:[],l=fe(a),u=ie(l,c,e.now),d=a===``?void 0:u;return(0,I.jsx)(j,{trigger:`Restore to a point in time`,title:`Restore ${e.cluster} to a point in time`,description:`Choose a time in UTC inside the restore window.`,block:`disabled`in t?t.disabled:void 0,reasonId:n,size:`sm`,context:e,write:t=>te(e,t,l),what:`pitr`,ready:u===void 0,onOpen:()=>o(``),apply:r,show:i,children:(0,I.jsxs)(I.Fragment,{children:[(0,I.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,I.jsx)(`p`,{className:`font-medium text-text`,children:`Restore window`}),(0,I.jsx)(`ul`,{className:`flex list-disc flex-col gap-0.5 pl-5 text-text-2`,children:c.map(e=>(0,I.jsx)(`li`,{children:he(e)},e.from))})]}),(0,I.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,I.jsx)(`label`,{htmlFor:`${s}-at`,className:`font-medium`,children:`Time (UTC)`}),(0,I.jsx)(p,{id:`${s}-at`,type:`datetime-local`,step:1,value:a,"aria-invalid":d!==void 0||void 0,"aria-describedby":d?`${s}-error`:void 0,onChange:e=>o(e.target.value)}),d&&(0,I.jsx)(`p`,{id:`${s}-error`,className:`text-xs text-bad`,children:d})]}),u===void 0&&l!==void 0&&(0,I.jsx)(M,{context:e,...oe(e,x(l))})]})})}function j({trigger:e,title:t,description:n,block:r,reasonId:i,size:o,context:f,write:p,what:m,ready:h=!0,onOpen:_,apply:v,show:y,children:b}){let x=(0,L.useId)(),[S,C]=(0,L.useState)(!1),[w,T]=(0,L.useTransition)();if(r!==void 0){let t=i??`${x}-reason`;return(0,I.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,I.jsx)(g,{variant:`outline`,size:o,disabled:!0,"aria-describedby":t,className:`self-start`,children:e}),i===void 0&&(0,I.jsx)(`p`,{id:t,className:`max-w-56 text-xs text-muted`,children:r})]})}return(0,I.jsxs)(d,{open:S,onOpenChange:e=>{e&&_?.(),C(e)},children:[(0,I.jsx)(u,{asChild:!0,children:(0,I.jsx)(g,{variant:`outline`,size:o,children:e})}),(0,I.jsxs)(s,{className:`max-h-[85vh] overflow-y-auto sm:max-w-lg`,children:[(0,I.jsxs)(a,{children:[(0,I.jsx)(c,{children:t}),(0,I.jsx)(l,{children:n})]}),(0,I.jsxs)(`div`,{className:`flex flex-col gap-4 text-sm`,children:[b,h&&(0,I.jsx)(ee,{name:f.cluster,confirmLabel:`Restore`,pending:w,onConfirm:()=>{let e=de(f,m),t=p(e);T(async()=>{let n=await v(P(f),t).catch(F);C(!1),y(n,`Restore ${e} created.`)})}})]})]})]})}function M({context:e,lead:t,lines:n}){let r=e.running?.what===`Backup`?C(e.running,`restore`):void 0;return(0,I.jsxs)(`div`,{className:`flex flex-col gap-2`,children:[(0,I.jsx)(`p`,{className:`font-medium text-text`,children:t}),(0,I.jsx)(`ul`,{className:`flex list-disc flex-col gap-1 pl-5 text-text-2`,children:n.map(e=>(0,I.jsx)(`li`,{children:e},e))}),r&&(0,I.jsx)(`p`,{className:`text-xs text-info`,children:r})]})}function he(e){let t=e.to?x(e.to):`now`,n=`${x(e.from)} to ${t}`;return e.failing?`${n}. Uploads are failing after this point.`:n}function N({context:e,row:t,apply:n,show:r}){let i=(0,L.useId)(),[,a]=(0,L.useTransition)(),o=(0,L.useRef)(!1),s=ce(e,t);if(s!==void 0)return(0,I.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,I.jsx)(g,{variant:`outline`,disabled:!0,"aria-describedby":`${i}-reason`,className:`self-start`,children:`Delete`}),(0,I.jsx)(`p`,{id:`${i}-reason`,className:`max-w-56 text-xs text-muted`,children:s})]});let{goes:c,stays:l}=w(t);return(0,I.jsx)(v,{triggerLabel:`Delete`,confirmLabel:`Delete backup`,goes:c,stays:l,onConfirm:()=>{if(o.current)return;o.current=!0;let i=O(e.namespace,t);a(async()=>{let a=await n(P(e),i).catch(F);o.current=!1,r(a,`Backup ${t.name} deleted.`)})}})}function P(e){return{namespace:e.namespace,name:e.cluster}}function F(e){return{ok:!1,errors:[{code:`Unavailable`,message:String(e)}]}}var I,L;function R(){return(R=e((()=>{I=n(),i(),L=t(),_(),y(),b(),h(),o(),f(),D(),k.__docgenInfo={description:`BackupHeaderActions holds Back up now, Restore to a point in time when
the cluster has it, Change schedule, and the note that the writes skip
Git.`,methods:[],displayName:`BackupHeaderActions`,props:{context:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  namespace: string;
  cluster: string;
  storage: BackupStorage;
  suspension?: Suspension;
  running?: { what: "Backup" | "Restore"; name: string; resumeFailed?: true };
  restoring?: string;
  backupNames: string[];
  restoreNames: string[];
  readingRestores: { name: string; kind: BackupKind; backup: string }[];
  clusterVersion?: string;
  retentionWindow?: string;
  changesOff?: string;
  now: string;
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`cluster`,value:{name:`string`,required:!0}},{key:`storage`,value:{name:`union`,raw:`| { kind: "none"; schedules: ScheduleView[] }
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`suspension`,value:{name:`union`,raw:`"held" | "requested" | "StorageAlreadyAttached" | "WaitingForHandover"`,elements:[{name:`literal`,value:`"held"`},{name:`literal`,value:`"requested"`},{name:`literal`,value:`"StorageAlreadyAttached"`},{name:`literal`,value:`"WaitingForHandover"`}],required:!1}},{key:`running`,value:{name:`signature`,type:`object`,raw:`{ what: "Backup" | "Restore"; name: string; resumeFailed?: true }`,signature:{properties:[{key:`what`,value:{name:`union`,raw:`"Backup" | "Restore"`,elements:[{name:`literal`,value:`"Backup"`},{name:`literal`,value:`"Restore"`}],required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`resumeFailed`,value:{name:`literal`,value:`true`,required:!1}}]},required:!1}},{key:`restoring`,value:{name:`string`,required:!1}},{key:`backupNames`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}},{key:`restoreNames`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}},{key:`readingRestores`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ name: string; kind: BackupKind; backup: string }`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`kind`,value:{name:`BackupKind`,required:!0}},{key:`backup`,value:{name:`string`,required:!0}}]}}],raw:`{ name: string; kind: BackupKind; backup: string }[]`,required:!0}},{key:`clusterVersion`,value:{name:`string`,required:!1}},{key:`retentionWindow`,value:{name:`string`,required:!1}},{key:`changesOff`,value:{name:`string`,required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``},apply:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(
  cluster: { namespace: string; name: string },
  change: Change,
) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`signature`,type:`object`,raw:`{ namespace: string; name: string }`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}}]}},name:`cluster`},{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}}},description:``},show:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(result: WriteResult, message: string) => void`,signature:{arguments:[{type:{name:`WriteResult`},name:`result`},{type:{name:`string`},name:`message`}],return:{name:`void`}}},description:``}}},A.__docgenInfo={description:`BackupRowActions holds Restore, on a Completed backup only, and Delete.`,methods:[],displayName:`BackupRowActions`,props:{context:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  namespace: string;
  cluster: string;
  storage: BackupStorage;
  suspension?: Suspension;
  running?: { what: "Backup" | "Restore"; name: string; resumeFailed?: true };
  restoring?: string;
  backupNames: string[];
  restoreNames: string[];
  readingRestores: { name: string; kind: BackupKind; backup: string }[];
  clusterVersion?: string;
  retentionWindow?: string;
  changesOff?: string;
  now: string;
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`cluster`,value:{name:`string`,required:!0}},{key:`storage`,value:{name:`union`,raw:`| { kind: "none"; schedules: ScheduleView[] }
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`suspension`,value:{name:`union`,raw:`"held" | "requested" | "StorageAlreadyAttached" | "WaitingForHandover"`,elements:[{name:`literal`,value:`"held"`},{name:`literal`,value:`"requested"`},{name:`literal`,value:`"StorageAlreadyAttached"`},{name:`literal`,value:`"WaitingForHandover"`}],required:!1}},{key:`running`,value:{name:`signature`,type:`object`,raw:`{ what: "Backup" | "Restore"; name: string; resumeFailed?: true }`,signature:{properties:[{key:`what`,value:{name:`union`,raw:`"Backup" | "Restore"`,elements:[{name:`literal`,value:`"Backup"`},{name:`literal`,value:`"Restore"`}],required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`resumeFailed`,value:{name:`literal`,value:`true`,required:!1}}]},required:!1}},{key:`restoring`,value:{name:`string`,required:!1}},{key:`backupNames`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}},{key:`restoreNames`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}},{key:`readingRestores`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ name: string; kind: BackupKind; backup: string }`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`kind`,value:{name:`BackupKind`,required:!0}},{key:`backup`,value:{name:`string`,required:!0}}]}}],raw:`{ name: string; kind: BackupKind; backup: string }[]`,required:!0}},{key:`clusterVersion`,value:{name:`string`,required:!1}},{key:`retentionWindow`,value:{name:`string`,required:!1}},{key:`changesOff`,value:{name:`string`,required:!1}},{key:`now`,value:{name:`string`,required:!0}}]}},description:``},row:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  kind: BackupKind;
  name: string;
  uid?: string;
  origin: "Scheduled" | "Manual";
  started: string;
  finished: string;
  duration: string;
  phase: string;
  tone: StatusTone;
  step: string;
  version: string;
  details: Detail[];
}`,signature:{properties:[{key:`kind`,value:{name:`BackupKind`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`uid`,value:{name:`string`,required:!1}},{key:`origin`,value:{name:`union`,raw:`"Scheduled" | "Manual"`,elements:[{name:`literal`,value:`"Scheduled"`},{name:`literal`,value:`"Manual"`}],required:!0}},{key:`started`,value:{name:`string`,required:!0}},{key:`finished`,value:{name:`string`,required:!0}},{key:`duration`,value:{name:`string`,required:!0}},{key:`phase`,value:{name:`string`,required:!0}},{key:`tone`,value:{name:`StatusTone`,required:!0}},{key:`step`,value:{name:`string`,required:!0}},{key:`version`,value:{name:`string`,required:!0}},{key:`details`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ label: string; value: string }`,signature:{properties:[{key:`label`,value:{name:`string`,required:!0}},{key:`value`,value:{name:`string`,required:!0}}]}}],raw:`Detail[]`,required:!0}}]}},description:``},apply:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(
  cluster: { namespace: string; name: string },
  change: Change,
) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`signature`,type:`object`,raw:`{ namespace: string; name: string }`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}}]}},name:`cluster`},{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}}},description:``},show:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(result: WriteResult, message: string) => void`,signature:{arguments:[{type:{name:`WriteResult`},name:`result`},{type:{name:`string`},name:`message`}],return:{name:`void`}}},description:``}}}})))()}function z(e,t={}){return{namespace:`team-a`,cluster:`orders`,storage:U[e],backupNames:[],restoreNames:[],readingRestores:[],clusterVersion:`8.9.4`,now:`2026-11-10T14:30:05Z`,...t}}function B(e){return(0,V.jsx)(`table`,{className:`text-sm`,children:(0,V.jsx)(`tbody`,{children:(0,V.jsxs)(`tr`,{children:[(0,V.jsx)(`td`,{className:`pr-4 font-medium`,children:e.row.name}),(0,V.jsx)(`td`,{className:`min-w-56 whitespace-normal`,children:(0,V.jsx)(A,{...e})})]})})})}var V,H,U,W,G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{V=n(),R(),{fn:H}=__STORYBOOK_MODULE_TEST__,U={elasticsearch:{kind:`elasticsearch`,bucket:{name:`backups`,location:`s3://acme-backups/orders`},repository:`camunda-backups`,schedules:[]},rdbms:{kind:`rdbms`,bucket:{name:`backups`,location:`s3://acme-backups/orders`},schedules:[{name:`nightly`,cron:`0 2 * * *`,completed:7,failed:3}]},archived:{kind:`rdbms`,bucket:{name:`backups`,location:`s3://acme-backups/orders`},database:{archive:{server:`orders-db`,state:{tone:`ok`,label:`Archiving`},window:[{from:`2026-11-04T02:00:00Z`,to:`2026-11-07T18:20:00Z`,failing:!0},{from:`2026-11-08T02:00:00Z`}]}},schedules:[]},none:{kind:`none`,schedules:[]}},W={kind:`LogicalBackupRDBMS`,name:`nightly-20261110`,origin:`Scheduled`,started:`2026-11-10 02:00 UTC`,finished:`2026-11-10 02:09 UTC`,duration:`9m 12s`,phase:`Completed`,tone:`ok`,step:`-`,version:`8.9.3`,details:[]},G={title:`cluster/BackupActions`,component:k,args:{context:z(`elasticsearch`),apply:H(async()=>({ok:!0,mode:`direct`})),show:H()},decorators:[e=>(0,V.jsx)(`div`,{className:`max-w-4xl`,children:(0,V.jsx)(e,{})})]},K={name:`Back up now on Elasticsearch`},q={name:`Back up now while a restore runs`,args:{context:z(`rdbms`,{suspension:`held`,running:{what:`Restore`,name:`orders-restore-1`},restoring:`orders-restore-1`})}},J={name:`Back up now without backup storage`,args:{context:z(`none`)}},Y={name:`Restore a completed PostgreSQL backup`,args:{context:z(`rdbms`)},render:e=>(0,V.jsx)(B,{...e,row:W})},X={name:`Restore of a backup that recorded no version`,args:{context:z(`rdbms`)},render:e=>(0,V.jsx)(B,{...e,row:{...W,version:`-`}})},Z={name:`Point in time on PostgreSQL with recovery by the operator`,args:{context:z(`archived`)}},Q=[`BackUpNowOnElasticsearch`,`BackUpNowWhileARestoreRuns`,`BackUpNowWithoutBackupStorage`,`RestoreACompletedPostgreSQLBackup`,`RestoreOfABackupThatRecordedNoVersion`,`PointInTimeOnPostgreSQLWithRecoveryByTheOperator`],K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
  name: "Back up now on Elasticsearch"
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  name: "Back up now while a restore runs",
  args: {
    context: context("rdbms", {
      suspension: "held",
      running: {
        what: "Restore",
        name: "orders-restore-1"
      },
      restoring: "orders-restore-1"
    })
  }
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "Back up now without backup storage",
  args: {
    context: context("none")
  }
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "Restore a completed PostgreSQL backup",
  args: {
    context: context("rdbms")
  },
  render: args => <RowCell {...args} row={scheduled} />
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "Restore of a backup that recorded no version",
  args: {
    context: context("rdbms")
  },
  render: args => <RowCell {...args} row={{
    ...scheduled,
    version: "-"
  }} />
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "Point in time on PostgreSQL with recovery by the operator",
  args: {
    context: context("archived")
  }
}`,...Z.parameters?.docs?.source}}}})))()}$();export{K as BackUpNowOnElasticsearch,q as BackUpNowWhileARestoreRuns,J as BackUpNowWithoutBackupStorage,Z as PointInTimeOnPostgreSQLWithRecoveryByTheOperator,Y as RestoreACompletedPostgreSQLBackup,X as RestoreOfABackupThatRecordedNoVersion,Q as __namedExportsOrder,G as default};