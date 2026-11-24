import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{n as r,t as i}from"./link-DOLMGIR2.js";import{a,c as o,n as s,o as c,r as l,s as u,t as d}from"./dialog-BCFYs4iF.js";import{n as f,t as p}from"./checkbox-CD0LgU-T.js";import{n as m,t as h}from"./input-BkHum75w.js";import{n as g,r as _,t as v}from"./button-H_M1e76Z.js";import{n as y,t as b}from"./inline-confirm-Bq3pCpAr.js";import{r as x,t as ee}from"./typed-confirm-8owe9eUt.js";import{o as S,s as C}from"./units-B_VbZtdu.js";import{_ as te,b as ne,c as re,d as ie,f as ae,g as oe,h as se,i as ce,l as le,m as ue,n as de,o as fe,p as pe,r as me,s as he,t as ge,u as _e,v as ve,y as ye}from"./backup-actions-Bhgoqu7W.js";function w({context:e,apply:t,show:n}){let i=(0,j.useId)(),a=de(e),o=ie(e),[s,c]=(0,j.useState)(),[l,u]=(0,j.useTransition)(),d=s!==void 0,f=d?ce(s,e.backupNames):[],p=()=>{if(s===void 0||a.kind===void 0)return;let r=me(e,a.kind,s);u(async()=>{let i=await t(O(e),r).catch(k);n(i,`Backup ${s} created.`),i.ok&&c(void 0)})};return(0,A.jsxs)(`div`,{className:`flex flex-col items-end gap-3`,children:[(0,A.jsxs)(`div`,{className:`flex flex-wrap items-center justify-end gap-x-3 gap-y-2`,children:[(0,A.jsx)(`p`,{className:`text-xs text-muted`,children:ge}),(0,A.jsx)(r,{href:`?tab=claim&section=attachments`,className:g({variant:`outline`,size:`sm`}),children:`Change schedule`}),o&&(0,A.jsx)(xe,{context:e,state:o,reasonId:`${i}-point-reason`,apply:t,show:n}),!d&&(0,A.jsx)(v,{size:`sm`,disabled:a.disabled!==void 0,"aria-describedby":a.disabled?`${i}-reason`:void 0,onClick:()=>c(_e(e.cluster,e.now)),children:`Back up now`})]}),o&&`disabled`in o&&(0,A.jsx)(`p`,{id:`${i}-point-reason`,className:`max-w-md text-right text-xs text-muted`,children:o.disabled}),a.disabled&&(0,A.jsx)(`p`,{id:`${i}-reason`,className:`max-w-md text-right text-xs text-muted`,children:a.disabled}),d&&(0,A.jsxs)(`form`,{"aria-label":`Back up now`,className:`flex w-full max-w-md flex-col gap-3 rounded-md border border-border bg-panel-2/50 p-3 text-sm`,onSubmit:e=>{e.preventDefault(),p()},children:[(0,A.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,A.jsx)(`label`,{htmlFor:`${i}-name`,className:`font-medium`,children:`Name`}),(0,A.jsx)(h,{id:`${i}-name`,value:s,autoFocus:!0,autoComplete:`off`,spellCheck:!1,"aria-invalid":f.length>0||void 0,"aria-describedby":f.length>0?`${i}-errors`:void 0,onChange:e=>c(e.target.value)}),f.length>0&&(0,A.jsx)(`ul`,{id:`${i}-errors`,className:`flex flex-col gap-0.5 text-xs text-bad`,children:f.map(e=>(0,A.jsx)(`li`,{children:e},e))})]}),a.waits&&(0,A.jsx)(`p`,{className:`text-xs text-info`,children:a.waits}),(0,A.jsxs)(`div`,{className:`flex flex-wrap gap-2`,children:[(0,A.jsx)(v,{type:`submit`,size:`sm`,disabled:f.length>0||l||a.disabled!==void 0,"aria-describedby":a.disabled?`${i}-reason`:void 0,children:`Start backup`}),(0,A.jsx)(v,{type:`button`,size:`sm`,variant:`ghost`,onClick:()=>c(void 0),children:`Cancel`})]})]})]})}function T({context:e,row:t,apply:n,show:r}){return(0,A.jsxs)(`div`,{className:`flex flex-wrap items-start gap-2`,children:[t.phase===`Completed`&&(0,A.jsx)(be,{context:e,row:t,apply:n,show:r}),(0,A.jsx)(Ce,{context:e,row:t,apply:n,show:r})]})}function be({context:e,row:t,apply:n,show:r}){let i=se(e,t);return(0,A.jsx)(E,{trigger:`Restore`,title:`Restore ${e.cluster}`,description:`From backup ${t.name}.`,block:i,context:e,write:n=>oe(e,t,n),what:`restore`,apply:n,show:r,children:(0,A.jsx)(D,{context:e,...te(e,t)})})}function xe({context:e,state:t,reasonId:n,apply:r,show:i}){let[a,o]=(0,j.useState)(``),[s,c]=(0,j.useState)(!1),l=(0,j.useId)(),u=`intervals`in t?t.intervals:[],d=`intervals`in t?t.manual:void 0,f=ye(a),m=ue(f,u,e.now),g=a===``?void 0:m;return(0,A.jsx)(E,{trigger:`Restore to a point in time`,title:`Restore ${e.cluster} to a point in time`,description:`Choose a time in UTC inside the restore window.`,block:`disabled`in t?t.disabled:void 0,reasonId:n,size:`sm`,context:e,write:t=>ae(e,t,f),what:`pitr`,ready:m===void 0&&(d===void 0||s),onOpen:()=>{o(``),c(!1)},apply:r,show:i,children:(0,A.jsxs)(A.Fragment,{children:[(0,A.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,A.jsx)(`p`,{className:`font-medium text-text`,children:`Restore window`}),(0,A.jsx)(`ul`,{className:`flex list-disc flex-col gap-0.5 pl-5 text-text-2`,children:u.map(e=>(0,A.jsx)(`li`,{children:Se(e)},e.from))})]}),(0,A.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,A.jsx)(`label`,{htmlFor:`${l}-at`,className:`font-medium`,children:`Time (UTC)`}),(0,A.jsx)(h,{id:`${l}-at`,type:`datetime-local`,step:1,value:a,"aria-invalid":g!==void 0||void 0,"aria-describedby":g?`${l}-error`:void 0,onChange:e=>{o(e.target.value),c(!1)}}),g&&(0,A.jsx)(`p`,{id:`${l}-error`,className:`text-xs text-bad`,children:g})]}),m===void 0&&f!==void 0&&(0,A.jsxs)(A.Fragment,{children:[(0,A.jsx)(D,{context:e,...pe(e,C(f),d)}),d&&(0,A.jsxs)(`div`,{className:`flex items-center gap-2`,children:[(0,A.jsx)(p,{id:`${l}-rolled`,checked:s,onCheckedChange:e=>c(e===!0)}),(0,A.jsx)(`label`,{htmlFor:`${l}-rolled`,className:`font-medium text-text`,children:`I rolled the database server back to this time`})]})]})]})})}function E({trigger:e,title:t,description:n,block:r,reasonId:i,size:o,context:f,write:p,what:m,ready:h=!0,onOpen:g,apply:_,show:y,children:b}){let x=(0,j.useId)(),[S,C]=(0,j.useState)(!1),[te,ne]=(0,j.useTransition)();if(r!==void 0){let t=i??`${x}-reason`;return(0,A.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,A.jsx)(v,{variant:`outline`,size:o,disabled:!0,"aria-describedby":t,className:`self-start`,children:e}),i===void 0&&(0,A.jsx)(`p`,{id:t,className:`max-w-56 text-xs text-muted`,children:r})]})}return(0,A.jsxs)(d,{open:S,onOpenChange:e=>{e&&g?.(),C(e)},children:[(0,A.jsx)(u,{asChild:!0,children:(0,A.jsx)(v,{variant:`outline`,size:o,children:e})}),(0,A.jsxs)(s,{className:`max-h-[85vh] overflow-y-auto sm:max-w-lg`,children:[(0,A.jsxs)(a,{children:[(0,A.jsx)(c,{children:t}),(0,A.jsx)(l,{children:n})]}),(0,A.jsxs)(`div`,{className:`flex flex-col gap-4 text-sm`,children:[b,h&&(0,A.jsx)(ee,{name:f.cluster,confirmLabel:`Restore`,pending:te,onConfirm:()=>{let e=ve(f,m),t=p(e);ne(async()=>{let n=await _(O(f),t).catch(k);C(!1),y(n,`Restore ${e} created.`)})}})]})]})]})}function D({context:e,lead:t,lines:n}){let r=e.running?.what===`Backup`?ne(e.running,`restore`):void 0;return(0,A.jsxs)(`div`,{className:`flex flex-col gap-2`,children:[(0,A.jsx)(`p`,{className:`font-medium text-text`,children:t}),(0,A.jsx)(`ul`,{className:`flex list-disc flex-col gap-1 pl-5 text-text-2`,children:n.map(e=>(0,A.jsx)(`li`,{children:e},e))}),r&&(0,A.jsx)(`p`,{className:`text-xs text-info`,children:r})]})}function Se(e){let t=e.to?C(e.to):`now`,n=`${C(e.from)} to ${t}`;return e.failing?`${n}. Uploads are failing after this point.`:n}function Ce({context:e,row:t,apply:n,show:r}){let i=(0,j.useId)(),[,a]=(0,j.useTransition)(),o=(0,j.useRef)(!1),s=he(e,t);if(s!==void 0)return(0,A.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,A.jsx)(v,{variant:`outline`,disabled:!0,"aria-describedby":`${i}-reason`,className:`self-start`,children:`Delete`}),(0,A.jsx)(`p`,{id:`${i}-reason`,className:`max-w-56 text-xs text-muted`,children:s})]});let{goes:c,stays:l}=re(t);return(0,A.jsx)(b,{triggerLabel:`Delete`,confirmLabel:`Delete backup`,goes:c,stays:l,onConfirm:()=>{if(o.current)return;o.current=!0;let i=fe(e.namespace,t);a(async()=>{let a=await n(O(e),i).catch(k);o.current=!1,r(a,`Backup ${t.name} deleted.`)})}})}function O(e){return{namespace:e.namespace,name:e.cluster}}function k(e){return{ok:!1,errors:[{code:`Unavailable`,message:String(e)}]}}var A,j;function M(){return(M=e((()=>{A=n(),i(),j=t(),y(),x(),S(),_(),f(),o(),m(),le(),w.__docgenInfo={description:`BackupHeaderActions holds Back up now, Restore to a point in time when
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
) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`signature`,type:`object`,raw:`{ namespace: string; name: string }`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}}]}},name:`cluster`},{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}}},description:``},show:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(result: WriteResult, message: string) => void`,signature:{arguments:[{type:{name:`WriteResult`},name:`result`},{type:{name:`string`},name:`message`}],return:{name:`void`}}},description:``}}},T.__docgenInfo={description:`BackupRowActions holds Restore, on a Completed backup only, and Delete.`,methods:[],displayName:`BackupRowActions`,props:{context:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
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
) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`signature`,type:`object`,raw:`{ namespace: string; name: string }`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}}]}},name:`cluster`},{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}}},description:``},show:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(result: WriteResult, message: string) => void`,signature:{arguments:[{type:{name:`WriteResult`},name:`result`},{type:{name:`string`},name:`message`}],return:{name:`void`}}},description:``}}}})))()}function N(e,t={}){return{namespace:`team-a`,cluster:`orders`,storage:H[e],backupNames:[],restoreNames:[],readingRestores:[],clusterVersion:`8.9.4`,now:`2026-11-10T14:30:05Z`,...t}}function P(e){return(0,F.jsx)(`table`,{className:`text-sm`,children:(0,F.jsx)(`tbody`,{children:(0,F.jsxs)(`tr`,{children:[(0,F.jsx)(`td`,{className:`pr-4 font-medium`,children:e.row.name}),(0,F.jsx)(`td`,{className:`min-w-56 whitespace-normal`,children:(0,F.jsx)(T,{...e})})]})})})}var F,I,L,R,z,B,V,H,U,W,G,K,q,J,Y,X,Z,Q,we;function $(){return($=e((()=>{F=n(),M(),{expect:I,fireEvent:L,fn:R,userEvent:z,waitFor:B,within:V}=__STORYBOOK_MODULE_TEST__,H={elasticsearch:{kind:`elasticsearch`,bucket:{name:`backups`,location:`s3://acme-backups/orders`},repository:`camunda-backups`,schedules:[]},rdbms:{kind:`rdbms`,bucket:{name:`backups`,location:`s3://acme-backups/orders`},schedules:[{name:`nightly`,cron:`0 2 * * *`,completed:7,failed:3}]},archived:{kind:`rdbms`,bucket:{name:`backups`,location:`s3://acme-backups/orders`},database:{archive:{server:`orders-db`,state:{tone:`ok`,label:`Archiving`},window:[{from:`2026-11-04T02:00:00Z`,to:`2026-11-07T18:20:00Z`,failing:!0},{from:`2026-11-08T02:00:00Z`}]}},schedules:[]},manual:{kind:`rdbms`,bucket:{name:`backups`,location:`s3://acme-backups/orders`},database:{serverConfig:`orders-pg`,endpoint:`orders-pg.abc123.eu-west-1.rds.amazonaws.com:5432`,pitr:{enabled:!0,recovery:`external`,retentionDays:3}},schedules:[]},manualWithoutPitr:{kind:`rdbms`,bucket:{name:`backups`,location:`s3://acme-backups/orders`},database:{serverConfig:`orders-pg`,endpoint:`orders-pg.abc123.eu-west-1.rds.amazonaws.com:5432`,pitr:{enabled:!1,recovery:`external`}},schedules:[]},none:{kind:`none`,schedules:[]}},U={kind:`LogicalBackupRDBMS`,name:`nightly-20261110`,origin:`Scheduled`,started:`2026-11-10 02:00 UTC`,finished:`2026-11-10 02:09 UTC`,duration:`9m 12s`,phase:`Completed`,tone:`ok`,step:`-`,version:`8.9.3`,details:[]},W={title:`cluster/BackupActions`,component:w,args:{context:N(`elasticsearch`),apply:R(async()=>({ok:!0,mode:`direct`})),show:R()},decorators:[e=>(0,F.jsx)(`div`,{className:`max-w-4xl`,children:(0,F.jsx)(e,{})})]},G={name:`Back up now on Elasticsearch`},K={name:`Back up now while a restore runs`,args:{context:N(`rdbms`,{suspension:`held`,running:{what:`Restore`,name:`orders-restore-1`},restoring:`orders-restore-1`})}},q={name:`Back up now without backup storage`,args:{context:N(`none`)}},J={name:`Restore a completed PostgreSQL backup`,args:{context:N(`rdbms`)},render:e=>(0,F.jsx)(P,{...e,row:U})},Y={name:`Restore of a backup that recorded no version`,args:{context:N(`rdbms`)},render:e=>(0,F.jsx)(P,{...e,row:{...U,version:`-`}})},X={name:`Point in time on PostgreSQL with recovery by the operator`,args:{context:N(`archived`)}},Z={name:`Point in time after a rollback outside the operator`,args:{context:N(`manual`)},play:async({canvas:e,canvasElement:t})=>{await z.click(e.getByRole(`button`,{name:`Restore to a point in time`}));let n=V(t.ownerDocument.body).getByRole(`dialog`,{name:`Restore orders to a point in time`});L.change(V(n).getByLabelText(`Time (UTC)`),{target:{value:`2026-11-09T09:30:15`}}),await B(()=>I(V(n).getByRole(`checkbox`,{name:`I rolled the database server back to this time`})).toBeVisible()),await I(V(n).queryByLabelText(`Type orders to confirm`)).toBeNull()}},Q={name:`Point in time on a server without point-in-time recovery`,args:{context:N(`manualWithoutPitr`)}},we=[`BackUpNowOnElasticsearch`,`BackUpNowWhileARestoreRuns`,`BackUpNowWithoutBackupStorage`,`RestoreACompletedPostgreSQLBackup`,`RestoreOfABackupThatRecordedNoVersion`,`PointInTimeOnPostgreSQLWithRecoveryByTheOperator`,`PointInTimeAfterARollbackOutsideTheOperator`,`PointInTimeOnAServerWithoutPointInTimeRecovery`],G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
  name: "Back up now on Elasticsearch"
}`,...G.parameters?.docs?.source}}},K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
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
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  name: "Back up now without backup storage",
  args: {
    context: context("none")
  }
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "Restore a completed PostgreSQL backup",
  args: {
    context: context("rdbms")
  },
  render: args => <RowCell {...args} row={scheduled} />
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "Restore of a backup that recorded no version",
  args: {
    context: context("rdbms")
  },
  render: args => <RowCell {...args} row={{
    ...scheduled,
    version: "-"
  }} />
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "Point in time on PostgreSQL with recovery by the operator",
  args: {
    context: context("archived")
  }
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "Point in time after a rollback outside the operator",
  args: {
    context: context("manual")
  },
  play: async ({
    canvas,
    canvasElement
  }) => {
    await userEvent.click(canvas.getByRole("button", {
      name: "Restore to a point in time"
    }));
    // The dialog renders in a portal outside the canvas.
    const dialog = within(canvasElement.ownerDocument.body).getByRole("dialog", {
      name: "Restore orders to a point in time"
    });
    fireEvent.change(within(dialog).getByLabelText("Time (UTC)"), {
      target: {
        value: "2026-11-09T09:30:15"
      }
    });
    // The dialog fades in, so the checkbox is not visible at once.
    await waitFor(() => expect(within(dialog).getByRole("checkbox", {
      name: "I rolled the database server back to this time"
    })).toBeVisible());
    await expect(within(dialog).queryByLabelText("Type orders to confirm")).toBeNull();
  }
}`,...Z.parameters?.docs?.source}}},Q.parameters={...Q.parameters,docs:{...Q.parameters?.docs,source:{originalSource:`{
  name: "Point in time on a server without point-in-time recovery",
  args: {
    context: context("manualWithoutPitr")
  }
}`,...Q.parameters?.docs?.source}}}})))()}$();export{G as BackUpNowOnElasticsearch,K as BackUpNowWhileARestoreRuns,q as BackUpNowWithoutBackupStorage,Z as PointInTimeAfterARollbackOutsideTheOperator,Q as PointInTimeOnAServerWithoutPointInTimeRecovery,X as PointInTimeOnPostgreSQLWithRecoveryByTheOperator,J as RestoreACompletedPostgreSQLBackup,Y as RestoreOfABackupThatRecordedNoVersion,we as __namedExportsOrder,W as default};