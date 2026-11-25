import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{x as r}from"./labels-Be1Q9KU6.js";import{i}from"./release-zhHsTP2R.js";import{t as a}from"./policy-BJNT8nX8.js";import{v as o}from"./draft-BoQQKzTU.js";import{b as s,p as c}from"./schema-COgdfV5K.js";import{t as l}from"./fields-BnOYzedk.js";import{at as u,rt as d,t as f,ut as p}from"./session-C0vHNhSg.js";import{i as m,n as h}from"./settings-DiVyIVyU.js";import{n as g,t as _}from"./editable-settings-D2zxWaOq.js";import{n as v,t as y}from"./diff-BLIgMNCc.js";import{i as b,o as x,r as S}from"./impacts-MDttJKl0.js";import{n as C,t as w}from"./backup-caution-Dme4pSiZ.js";import{i as T,n as E,r as D}from"./claim-edit-DiW9Ybtk.js";import{i as O}from"./unread-Do34-4tt.js";function k(e,t){let n=e.cluster.spec.platformConfigRef;return{session:t,live:e.cluster,presets:e.presets,releases:e.releases,sections:e.sections,environment:e.environment,platform:n===void 0?void 0:e.platformConfigs.find(e=>e.metadata?.name===n),storage:e.storage}}function A(){return(A=e((()=>{})))()}function j(e,t){let n=t.spec?.allowVersionDowngrade;return!!n&&e.status?.management?.version===n&&!e.metadata?.annotations?.[`camunda.io/allow-version-downgrade`]}function M(e,t){let n=e.environment?.spec.releases?.default,r=u(t).spec,a=t.claim.spec?.releaseRef||n,o=r?.releaseRef||n,s=r?.allowVersionDowngrade!==void 0&&r.allowVersionDowngrade!==t.claim.spec?.allowVersionDowngrade;if(!o||o===a&&!s)return;let c=e.releases.find(e=>e.metadata?.name===o);if(!c)return;let l=i(e.cluster,c,e.environment,e.backups);return l.downgrade&&l.current?{from:l.current,to:l.target}:void 0}function N(){return(N=e((()=>{r(),a(),f()})))()}function P(e,t,n,r){let i=w(k(e,t),e.backups,n,r),a=M(e,t);return a?{impacts:i.impacts,caution:(0,F.jsxs)(`div`,{className:`flex flex-col gap-3`,children:[(0,F.jsxs)(`p`,{children:[`Camunda does not support a downgrade of a running cluster. This change moves the cluster from `,a.from,` to `,a.to,`.`]}),i.caution]}),confirm:i.confirm??I}:i}var F,I;function L(){return(L=e((()=>{F=n(),C(),N(),I={danger:!0,label:`Downgrade`,gitLabel:`Push downgrade`}})))()}function R({context:e,actions:t,children:n}){let{fields:r,environment:i,sections:a,registrations:s,releases:c,cluster:l,backups:u,requests:f,presets:p,platformConfigs:m,storage:h,optimize:g,schedules:_}=e,v=!e.unread.includes(`PluginRegistration`),y=(0,W.useMemo)(()=>o(r),[r]),b=(0,W.useCallback)(e=>[...d(e,{environment:i,sections:a,registrations:s,releases:c,cluster:l,backups:u,requests:f,registrationsRead:v}),...z({cluster:l,presets:p,releases:c,sections:a,environment:i,platformConfigs:m,storage:h,registrations:s,optimize:g,schedules:_},e)],[i,a,s,c,l,u,f,v,p,m,h,g,_]),x={claim:e.claim,resource:l,writeMode:e.writeMode,locks:y,validate:b,keptByDryRun:!0,prepare:e=>H(e,l),review:(t,n)=>V(e,t,n)};return(0,U.jsx)(E,{host:x,actions:t,children:n})}function z(e,t){let{after:n}=b(k(e,t)),r=u(t),i=c(h({cluster:e.cluster,merged:n,claim:r,sections:e.sections,registrations:e.registrations,optimize:e.optimize,schedules:e.schedules}));return _(i,n.spec,r.spec.cluster??{}).map(e=>({message:e}))}function B(){return T()}function V(e,t,{mode:n,close:r,saved:i}){let a=x(t)?b(k(e,i)):void 0;return{...e.drafted?{impacts:[]}:P(e,i,n,{backupsHref:`?tab=backups`,onOpenBackups:()=>{r(),window.history.replaceState(null,``,`?tab=backups`)}}),mergedLines:a&&v(a.before.spec,a.after.spec),blocking:[...e.drafted&&e.unread.includes(`CamundaCluster`)?[O([`CamundaCluster`])]:[],...i===t?[]:z(e,i).map(e=>`${e.message} ${G}`)]}}function H(e,t){if(!j(t,e.claim)||e.pending.has(s(``,K)))return e;let n=p(e,K);return`session`in n?n.session:e}var U,W,G,K;function q(){return(q=e((()=>{U=n(),W=t(),D(),y(),l(),f(),g(),S(),m(),N(),L(),G=`A field that you cleared stays in the claim that the save leaves, or gets its default value again. That causes this error. Go back and change the edit, or take the fields over when the review offers that.`,K=[{field:`spec`},{field:`allowVersionDowngrade`}],R.__docgenInfo={description:`ClusterEditProvider is the claim edit host of the cluster page, over the
live claim of context, with clusterLocks of context.fields, and with
validateClusterClaim and the merged errors that block as its
validation. Review adds the reset of allowVersionDowngrade once the
downgrade is done, and shows the impacts, cautions, confirm, and merged
diff of the cluster. For a drafted cluster, the review shows no impacts,
cautions, or confirm, because no cluster runs yet, and it blocks the save
while context.unread names CamundaCluster. In Git mode, the review keeps
each field that the change removes and the dry run of the push still
holds. It needs a GitPushesProvider above it.`,methods:[],displayName:`ClusterEditProvider`,props:{context:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
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
  unread: readonly string[];
  drafted?: true;
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
  serverUse?: ServerUse;
  archive?: ArchiveView;
}`,signature:{properties:[{key:`missing`,value:{name:`string`,required:!1}},{key:`serverConfig`,value:{name:`string`,required:!1}},{key:`endpoint`,value:{name:`string`,required:!1}},{key:`pitr`,value:{name:`signature`,type:`object`,raw:`{ enabled: boolean; recovery?: string; retentionDays?: number }`,signature:{properties:[{key:`enabled`,value:{name:`boolean`,required:!0}},{key:`recovery`,value:{name:`string`,required:!1}},{key:`retentionDays`,value:{name:`number`,required:!1}}]},required:!1}},{key:`serverUse`,value:{name:`union`,raw:`| { state: "dedicated" }
| { state: "unheld"; database: string }
| { state: "shared"; other: string }
| { state: "unprobed" }
| { state: "unplaced"; database: string }
| { state: "unread" }`,elements:[{name:`signature`,type:`object`,raw:`{ state: "dedicated" }`,signature:{properties:[{key:`state`,value:{name:`literal`,value:`"dedicated"`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{ state: "unheld"; database: string }`,signature:{properties:[{key:`state`,value:{name:`literal`,value:`"unheld"`,required:!0}},{key:`database`,value:{name:`string`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{ state: "shared"; other: string }`,signature:{properties:[{key:`state`,value:{name:`literal`,value:`"shared"`,required:!0}},{key:`other`,value:{name:`string`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{ state: "unprobed" }`,signature:{properties:[{key:`state`,value:{name:`literal`,value:`"unprobed"`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{ state: "unplaced"; database: string }`,signature:{properties:[{key:`state`,value:{name:`literal`,value:`"unplaced"`,required:!0}},{key:`database`,value:{name:`string`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{ state: "unread" }`,signature:{properties:[{key:`state`,value:{name:`literal`,value:`"unread"`,required:!0}}]}}],required:!1}},{key:`archive`,value:{name:`signature`,type:`object`,raw:`{
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`ProcessCardId | SectionComponentId`,elements:[{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}]},{name:`SectionComponentId`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}},{key:`unread`,value:{name:`unknown`,required:!0}},{key:`drafted`,value:{name:`literal`,value:`true`,required:!1}}]}},description:``},actions:{required:!0,tsType:{name:`ClaimActions`},description:``},children:{required:!0,tsType:{name:`ReactNode`},description:``}}}})))()}export{N as a,j as i,q as n,k as o,B as r,A as s,R as t};