import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{x as r}from"./labels-Be1Q9KU6.js";import{i}from"./release-zhHsTP2R.js";import{t as a}from"./policy-C-971q0L.js";import{v as o}from"./draft-BXud8ANW.js";import{v as s}from"./schema-CkYx6TI9.js";import{t as c}from"./fields-CnMyiCBC.js";import{it as l,lt as u,nt as d,t as f}from"./session-DKdEncwg.js";import{n as p,t as m}from"./diff-BLIgMNCc.js";import{i as h,o as g,r as _}from"./impacts-BWtrXMeM.js";import{n as v,t as y}from"./backup-caution-C5KuyNjj.js";import{i as b,n as x,r as S}from"./claim-edit-CXY_cuS0.js";function C(e,t){let n=e.cluster.spec.platformConfigRef;return{session:t,live:e.cluster,presets:e.presets,releases:e.releases,sections:e.sections,environment:e.environment,platform:n===void 0?void 0:e.platformConfigs.find(e=>e.metadata?.name===n),storage:e.storage}}function w(){return(w=e((()=>{})))()}function T(e,t){let n=t.spec?.allowVersionDowngrade;return!!n&&e.status?.management?.version===n&&!e.metadata?.annotations?.[`camunda.io/allow-version-downgrade`]}function E(e,t){let n=e.environment?.spec.releases?.default,r=l(t).spec,a=t.claim.spec?.releaseRef||n,o=r?.releaseRef||n,s=r?.allowVersionDowngrade!==void 0&&r.allowVersionDowngrade!==t.claim.spec?.allowVersionDowngrade;if(!o||o===a&&!s)return;let c=e.releases.find(e=>e.metadata?.name===o);if(!c)return;let u=i(e.cluster,c,e.environment,e.backups);return u.downgrade&&u.current?{from:u.current,to:u.target}:void 0}function D(){return(D=e((()=>{r(),a(),f()})))()}function O(e,t,n,r){let i=y(C(e,t),e.backups,n,r),a=E(e,t);return a?{impacts:i.impacts,caution:(0,k.jsxs)(`div`,{className:`flex flex-col gap-3`,children:[(0,k.jsxs)(`p`,{children:[`Camunda does not support a downgrade of a running cluster. This change moves the cluster from `,a.from,` to `,a.to,`.`]}),i.caution]}),confirm:i.confirm??A}:i}var k,A;function j(){return(j=e((()=>{k=n(),v(),D(),A={danger:!0,label:`Downgrade`,gitLabel:`Push downgrade`}})))()}function M({context:e,actions:t,children:n}){let{fields:r,environment:i,sections:a,registrations:s,releases:c,cluster:l,backups:u,requests:f}=e,p=!e.unread.includes(`PluginRegistration`),m=(0,L.useMemo)(()=>o(r),[r]),h=(0,L.useCallback)(e=>d(e,{environment:i,sections:a,registrations:s,releases:c,cluster:l,backups:u,requests:f,registrationsRead:p}),[i,a,s,c,l,u,f,p]),g={claim:e.claim,resource:l,writeMode:e.writeMode,locks:m,validate:h,prepare:e=>F(e,l),review:(t,n)=>P(e,t,n)};return(0,I.jsx)(x,{host:g,actions:t,children:n})}function N(){return b()}function P(e,t,{mode:n,close:r,saved:i}){let a=g(t)?h(C(e,i)):void 0;return{...O(e,i,n,{backupsHref:`?tab=backups`,onOpenBackups:()=>{r(),window.history.replaceState(null,``,`?tab=backups`)}}),mergedLines:a&&p(a.before.spec,a.after.spec)}}function F(e,t){if(!T(t,e.claim)||e.pending.has(s(``,R)))return e;let n=u(e,R);return`session`in n?n.session:e}var I,L,R;function z(){return(z=e((()=>{I=n(),L=t(),S(),m(),c(),f(),_(),D(),j(),R=[{field:`spec`},{field:`allowVersionDowngrade`}],M.__docgenInfo={description:`ClusterEditProvider is the claim edit host of the cluster page, over the
live claim of context, with clusterLocks of context.fields and
validateClusterClaim. Review adds the reset of allowVersionDowngrade once
the downgrade is done, and shows the impacts, cautions, confirm, and
merged diff of the cluster. It needs a GitPushesProvider above it.`,methods:[],displayName:`ClusterEditProvider`,props:{context:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}],required:!0}},{key:`schedules`,value:{name:`unknown`,required:!0}},{key:`optimize`,value:{name:`Resource`,elements:[{name:`literal`,value:`"CamundaOptimize"`}],raw:`Resource<"CamundaOptimize">`,required:!1}},{key:`fields`,value:{name:`unknown`,required:!0}},{key:`writeMode`,value:{name:`WriteMode`,required:!1}},{key:`openId`,value:{name:`union`,raw:`ProcessCardId | SectionComponentId`,elements:[{name:`union`,raw:`Process | "optimize"`,elements:[{name:`CoreComponent`},{name:`literal`,value:`"optimize"`}]},{name:`SectionComponentId`}],required:!1}},{key:`now`,value:{name:`string`,required:!0}},{key:`unread`,value:{name:`unknown`,required:!0}}]}},description:``},actions:{required:!0,tsType:{name:`ClaimActions`},description:``},children:{required:!0,tsType:{name:`ReactNode`},description:``}}}})))()}export{D as a,T as i,z as n,C as o,N as r,w as s,M as t};