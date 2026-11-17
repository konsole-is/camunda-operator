import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{r,x as i}from"./labels-Be1Q9KU6.js";import{n as a,t as o}from"./chevron-right-vTNMyx5Q.js";import{o as s,t as c}from"./units-B_VbZtdu.js";import{a as l,n as u,r as d,t as f}from"./backups-5qYRUzfX.js";import{n as ee,t as p}from"./not-read-RuqQXk-B.js";import{a as m,r as h,s as g}from"./status-B-qMFQRy.js";import{a as _,c as te,i as v,n as y,o as b,s as x,t as S}from"./table-CNIcR40R.js";function C({storage:e,backups:t,restores:n,actions:r,rowActions:i,unread:a=[]}){return(0,A.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[r&&(0,A.jsx)(`div`,{className:`flex flex-wrap items-center justify-end gap-2`,children:r}),(0,A.jsxs)(`div`,{className:`grid gap-4 lg:grid-cols-[3fr_2fr]`,children:[(0,A.jsx)(ne,{storage:e}),(0,A.jsx)(ie,{schedules:e.schedules})]}),(0,A.jsx)(ae,{rows:t,rowActions:i,unread:a.filter(e=>P.includes(e))}),(0,A.jsx)(E,{rows:n,unread:a.filter(e=>F.includes(e))})]})}function ne({storage:e}){return(0,A.jsxs)(`section`,{"aria-label":`Backup storage`,className:M,children:[(0,A.jsx)(`h2`,{className:N,children:`Backup storage`}),e.kind===`none`?(0,A.jsx)(`p`,{className:`text-sm text-text-2`,children:`This cluster has no backup storage. Backups need a bucket from the Backup storage section.`}):(0,A.jsxs)(`dl`,{className:`grid grid-cols-[max-content_1fr] gap-x-4 gap-y-2 text-sm`,children:[e.missing&&(0,A.jsx)(D,{label:`Secondary storage`,children:(0,A.jsxs)(`span`,{className:`text-bad`,children:[e.missing,` not found`]})}),(0,A.jsx)(D,{label:`Bucket`,children:k(e.bucket)}),e.kind===`elasticsearch`&&(0,A.jsx)(D,{label:`Snapshot repository`,children:e.repository??(0,A.jsx)(`span`,{className:`text-warn`,children:`Not registered yet`})}),e.database&&(0,A.jsx)(re,{database:e.database})]})]})}function re({database:e}){if(e.missing)return(0,A.jsx)(D,{label:`Database server`,children:(0,A.jsxs)(`span`,{className:`text-bad`,children:[e.missing,` not found`]})});let{pitr:t,archive:n}=e;return(0,A.jsxs)(A.Fragment,{children:[(0,A.jsxs)(D,{label:`Database server`,children:[e.serverConfig,(0,A.jsx)(`span`,{className:`block break-all text-xs text-muted`,children:e.endpoint})]}),(0,A.jsx)(D,{label:`Point-in-time restore`,children:t?.enabled?[`On`,t.retentionDays!==void 0&&`${t.retentionDays} days`,t.recovery===`operator`?`recovery by the operator`:`recovery outside the operator`].filter(Boolean).join(`, `):`Off`}),n&&(0,A.jsxs)(A.Fragment,{children:[(0,A.jsx)(D,{label:`Archive`,children:(0,A.jsx)(O,{tone:n.state.tone,title:n.state.message,children:n.state.label})}),n.bucket&&(0,A.jsx)(D,{label:`Archive bucket`,children:k(n.bucket)}),n.window&&(0,A.jsx)(D,{label:`Restore window`,children:(0,A.jsx)(w,{window:n.window})})]})]})}function w({window:e}){return e.length===0?(0,A.jsx)(A.Fragment,{children:`No restore point yet`}):(0,A.jsx)(`ul`,{className:`flex flex-col gap-0.5`,children:e.map((t,n)=>(0,A.jsxs)(j.Fragment,{children:[n>0&&!T(e[n-1],t)&&(0,A.jsx)(`li`,{className:`text-xs text-muted`,children:`No restore point in this gap`}),(0,A.jsxs)(`li`,{children:[c(t.from),` to`,` `,t.to?c(t.to):`now`,t.failing&&(0,A.jsx)(`span`,{className:`block text-xs text-warn`,children:`Uploads are failing after this point`})]})]},t.from))})}function T(e,t){return e.to!==void 0&&Date.parse(e.to)===Date.parse(t.from)}function ie({schedules:e}){return(0,A.jsxs)(`section`,{"aria-label":`Backup schedule`,className:M,children:[(0,A.jsx)(`h2`,{className:N,children:`Schedule`}),e.length===0?(0,A.jsx)(`p`,{className:`text-sm text-muted`,children:`No backup schedule. Backups run only when someone starts one.`}):(0,A.jsx)(`ul`,{className:`flex flex-col gap-2 text-sm`,children:e.map(e=>(0,A.jsxs)(`li`,{children:[(0,A.jsx)(`span`,{className:`font-medium`,children:e.name}),`:`,` `,(0,A.jsx)(`code`,{className:`rounded bg-panel-2 px-1 text-xs`,children:e.cron}),`, keeps `,e.completed,` completed and `,e.failed,` failed`,e.lastBackup&&(0,A.jsxs)(`span`,{className:`block text-xs text-muted`,children:[`Last backup `,e.lastBackup]})]},e.name))})]})}function ae({rows:e,rowActions:t,unread:n}){let[r,i]=(0,j.useState)(),a=t?9:8;return(0,A.jsxs)(`section`,{"aria-label":`Backups`,className:M,children:[(0,A.jsx)(`h2`,{className:N,children:`Backups`}),n.length>0&&(0,A.jsx)(p,{kinds:n}),e.length===0?n.length===0&&(0,A.jsx)(`p`,{className:`text-sm text-muted`,children:`No backups yet.`}):(0,A.jsxs)(S,{label:`Backup list`,children:[(0,A.jsx)(b,{children:(0,A.jsxs)(x,{children:[(0,A.jsx)(_,{children:`Name`}),(0,A.jsx)(_,{children:`Origin`}),(0,A.jsx)(_,{children:`Started`}),(0,A.jsx)(_,{children:`Finished`}),(0,A.jsx)(_,{children:`Duration`}),(0,A.jsx)(_,{children:`Phase`}),(0,A.jsx)(_,{children:`Step`}),(0,A.jsx)(_,{children:`Version`}),t&&(0,A.jsx)(_,{children:`Actions`})]})}),(0,A.jsx)(y,{children:e.map(e=>{let n=`backup-${e.kind}-${e.name}`,s=r===n;return(0,A.jsxs)(j.Fragment,{children:[(0,A.jsxs)(x,{children:[(0,A.jsx)(v,{children:(0,A.jsxs)(`button`,{type:`button`,"aria-expanded":s,"aria-controls":n,"aria-label":`Details of ${e.name}`,onClick:()=>i(s?void 0:n),className:`relative flex items-center gap-1.5 rounded font-medium outline-none hit-area hover:underline focus-visible:ring-3 focus-visible:ring-ring/50`,children:[(0,A.jsx)(o,{"aria-hidden":!0,className:`size-3.5 text-muted transition-transform motion-reduce:transition-none ${s?`rotate-90`:``}`}),e.name]})}),(0,A.jsx)(v,{className:`text-text-2`,children:e.origin}),(0,A.jsx)(v,{className:`text-text-2`,children:e.started}),(0,A.jsx)(v,{className:`text-text-2`,children:e.finished}),(0,A.jsx)(v,{className:`tabular-nums`,children:e.duration}),(0,A.jsx)(v,{children:(0,A.jsx)(O,{tone:e.tone,children:e.phase})}),(0,A.jsx)(v,{className:`text-text-2`,children:e.step}),(0,A.jsx)(v,{className:`text-text-2`,children:e.version}),t&&(0,A.jsx)(v,{className:`min-w-56 whitespace-normal`,children:t(e)})]}),s&&(0,A.jsx)(x,{id:n,className:`hover:bg-transparent`,children:(0,A.jsx)(v,{colSpan:a,className:`bg-panel-2/50`,children:e.details.length===0?(0,A.jsx)(`p`,{className:`text-sm text-muted`,children:`No details yet.`}):(0,A.jsx)(`dl`,{className:`grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-sm whitespace-normal`,children:e.details.map(e=>(0,A.jsx)(D,{label:e.label,children:(0,A.jsx)(`span`,{className:`break-all`,children:e.value})},e.label))})})})]},n)})})]})]})}function E({rows:e,unread:t}){return(0,A.jsxs)(`section`,{"aria-label":`Restores`,className:M,children:[(0,A.jsx)(`h2`,{className:N,children:`Restores`}),t.length>0&&(0,A.jsx)(p,{kinds:t}),e.length===0?t.length===0&&(0,A.jsx)(`p`,{className:`text-sm text-muted`,children:`No restores.`}):(0,A.jsxs)(S,{label:`Restore list`,children:[(0,A.jsx)(b,{children:(0,A.jsxs)(x,{children:[(0,A.jsx)(_,{children:`Name`}),(0,A.jsx)(_,{children:`Type`}),(0,A.jsx)(_,{children:`Source`}),(0,A.jsx)(_,{children:`Started`}),(0,A.jsx)(_,{children:`Phase`})]})}),(0,A.jsx)(y,{children:e.map(e=>(0,A.jsxs)(x,{children:[(0,A.jsx)(v,{className:`font-medium`,children:e.name}),(0,A.jsx)(v,{className:`text-text-2`,children:e.label}),(0,A.jsx)(v,{className:`text-text-2`,children:e.source}),(0,A.jsx)(v,{className:`text-text-2`,children:e.started}),(0,A.jsxs)(v,{className:`whitespace-normal`,children:[(0,A.jsx)(O,{tone:e.tone,children:e.phase}),e.failureMessage&&(0,A.jsx)(`span`,{className:`block text-xs text-text-2`,children:e.failureMessage})]})]},`${e.kind}-${e.name}`))})]})]})}function D({label:e,children:t}){return(0,A.jsxs)(A.Fragment,{children:[(0,A.jsx)(`dt`,{className:`text-muted`,children:e}),(0,A.jsx)(`dd`,{className:`min-w-0`,children:t})]})}function O({tone:e,title:t,children:n}){return(0,A.jsxs)(`span`,{title:t,className:`inline-flex items-center gap-1.5 ${m[e]}`,children:[(0,A.jsx)(`span`,{"aria-hidden":!0,className:`inline-block size-2 shrink-0 rounded-full ${h[e]}`}),n]})}function k(e){return e.location??`${e.name} (not found)`}var A,j,M,N,P,F;function I(){return(I=e((()=>{A=n(),a(),j=t(),ee(),g(),s(),te(),M=`flex flex-col gap-3 rounded-lg border border-border bg-panel p-4`,N=`font-heading text-base font-semibold`,P=[`LogicalBackupElasticsearch`,`LogicalBackupRDBMS`],F=[`LogicalRestoreElasticsearch`,`LogicalRestoreRDBMS`,`PointInTimeRestore`],C.__docgenInfo={description:`BackupsTab is the content of the Backups tab: the backup storage and the
schedules, the backups, and the restores of one cluster. actions is the
node of the cluster.backups.actions slot, shown in the header. When
rowActions is set, the backup table has an Actions column with its node
for each row. unread holds the kinds that the page could not read: a
list of backups or restores that depends on one of them names the kind,
and never says that there are none.`,methods:[],displayName:`BackupsTab`,props:{storage:{required:!0,tsType:{name:`union`,raw:`| { kind: "none"; schedules: ScheduleView[] }
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
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`cron`,value:{name:`string`,required:!0}},{key:`completed`,value:{name:`number`,required:!0}},{key:`failed`,value:{name:`number`,required:!0}},{key:`lastBackup`,value:{name:`string`,required:!1}}]}}],raw:`ScheduleView[]`,required:!0}}]}}]},description:``},backups:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
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
}`,signature:{properties:[{key:`kind`,value:{name:`BackupKind`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`uid`,value:{name:`string`,required:!1}},{key:`origin`,value:{name:`union`,raw:`"Scheduled" | "Manual"`,elements:[{name:`literal`,value:`"Scheduled"`},{name:`literal`,value:`"Manual"`}],required:!0}},{key:`started`,value:{name:`string`,required:!0}},{key:`finished`,value:{name:`string`,required:!0}},{key:`duration`,value:{name:`string`,required:!0}},{key:`phase`,value:{name:`string`,required:!0}},{key:`tone`,value:{name:`StatusTone`,required:!0}},{key:`step`,value:{name:`string`,required:!0}},{key:`version`,value:{name:`string`,required:!0}},{key:`details`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ label: string; value: string }`,signature:{properties:[{key:`label`,value:{name:`string`,required:!0}},{key:`value`,value:{name:`string`,required:!0}}]}}],raw:`Detail[]`,required:!0}}]}}],raw:`BackupRow[]`},description:``},restores:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  kind: RestoreKind;
  name: string;
  label: "Restore" | "Point-in-time restore";
  source: string;
  started: string;
  phase: string;
  tone: StatusTone;
  running: boolean;
  failureMessage?: string;
}`,signature:{properties:[{key:`kind`,value:{name:`RestoreKind`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`label`,value:{name:`union`,raw:`"Restore" | "Point-in-time restore"`,elements:[{name:`literal`,value:`"Restore"`},{name:`literal`,value:`"Point-in-time restore"`}],required:!0}},{key:`source`,value:{name:`string`,required:!0}},{key:`started`,value:{name:`string`,required:!0}},{key:`phase`,value:{name:`string`,required:!0}},{key:`tone`,value:{name:`StatusTone`,required:!0}},{key:`running`,value:{name:`boolean`,required:!0}},{key:`failureMessage`,value:{name:`string`,required:!1}}]}}],raw:`RestoreRow[]`},description:``},actions:{required:!1,tsType:{name:`ReactNode`},description:``},rowActions:{required:!1,tsType:{name:`signature`,type:`function`,raw:`(row: BackupRow) => ReactNode`,signature:{arguments:[{type:{name:`signature`,type:`object`,raw:`{
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
}`,signature:{properties:[{key:`kind`,value:{name:`BackupKind`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`uid`,value:{name:`string`,required:!1}},{key:`origin`,value:{name:`union`,raw:`"Scheduled" | "Manual"`,elements:[{name:`literal`,value:`"Scheduled"`},{name:`literal`,value:`"Manual"`}],required:!0}},{key:`started`,value:{name:`string`,required:!0}},{key:`finished`,value:{name:`string`,required:!0}},{key:`duration`,value:{name:`string`,required:!0}},{key:`phase`,value:{name:`string`,required:!0}},{key:`tone`,value:{name:`StatusTone`,required:!0}},{key:`step`,value:{name:`string`,required:!0}},{key:`version`,value:{name:`string`,required:!0}},{key:`details`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ label: string; value: string }`,signature:{properties:[{key:`label`,value:{name:`string`,required:!0}},{key:`value`,value:{name:`string`,required:!0}}]}}],raw:`Detail[]`,required:!0}}]}},name:`row`}],return:{name:`ReactNode`}}},description:``},unread:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}}}}})))()}function L(e,t,n,r={},i={}){return{kind:e,metadata:{namespace:`team-a`,name:t,...i},spec:n,status:r}}function R(e){return{cluster:B(),spec:{},storage:[],objectStorage:[],databaseConfigs:[],databaseServerConfigs:[],databaseServers:[],databaseServerPresets:[],schedules:[],now:z,...e}}var z,B,V,H,U,W,G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{i(),d(),I(),z=`2026-10-18T12:00:00Z`,B=e=>L(`CamundaCluster`,`orders`,{},e?{management:{backupRepository:e}}:{}),V=L(`BackupSchedule`,`nightly`,{clusterRef:{name:`orders`},schedule:`0 2 * * *`,retained:{completed:7,failed:3}},{lastBackupName:`nightly-20261018`}),H=L(`ObjectStorageConfig`,`backups`,{type:`S3`,s3:{bucketName:`acme-backups`,basePath:`orders`,region:`eu-west-1`}}),U=[L(`LogicalBackupElasticsearch`,`nightly-20261018`,{clusterRef:{name:`orders`}},{phase:`Completed`,step:`ResumeExporting`,backupId:1760752800,repository:`camunda-backups`,partitionsCount:3,version:`8.9.23`,storageSizes:{zeebe:`10Gi`,elasticsearch:21474836480},completionTime:`2026-10-18T02:14:22Z`},{creationTimestamp:`2026-10-18T02:00:00Z`,labels:{[r]:`nightly`}}),L(`LogicalBackupElasticsearch`,`before-upgrade`,{clusterRef:{name:`orders`}},{phase:`Failed`,step:`SnapshotRecords`,backupId:176077e4,repository:`camunda-backups`,failureMessage:`Elasticsearch refused the snapshot: the repository is read-only`},{creationTimestamp:`2026-10-18T06:46:40Z`}),L(`LogicalBackupElasticsearch`,`manual-now`,{clusterRef:{name:`orders`}},{phase:`Running`,step:`BackupRuntime`,version:`8.9.23`},{creationTimestamp:`2026-10-18T11:48:00Z`})],W=[L(`LogicalRestoreElasticsearch`,`restore-staging`,{targetClusterRef:{name:`orders`},backupRef:{name:`nightly-20261018`}},{phase:`RestoringPrimaryStorage`},{creationTimestamp:`2026-10-18T11:50:00Z`})],G=R({spec:{storageRef:`orders-db-storage`,backupStorageRef:`backups`},storage:[L(`SecondaryStorageConfig`,`orders-db-storage`,{type:`rdbms`,rdbms:{databaseConfigRef:`orders-db`}})],objectStorage:[H,L(`ObjectStorageConfig`,`wal`,{type:`GCS`,gcs:{bucketName:`acme-wal`,basePath:`orders-pg`}})],databaseConfigs:[L(`DatabaseConfig`,`orders-db`,{serverRef:`orders-pg`})],databaseServerConfigs:[L(`DatabaseServerConfig`,`orders-pg`,{host:`orders-pg-rw.team-a.svc`,port:5432,pitr:{enabled:!0,recovery:`operator`,retentionPeriodDays:30}},{},{ownerReferences:[{apiVersion:`core.camunda.io/v1`,kind:`DatabaseServer`,name:`orders-pg`,uid:`uid-orders-pg`,controller:!0}]})],databaseServers:[L(`DatabaseServer`,`orders-pg`,{databaseServerConfig:`orders-pg`,archive:{objectStorageRef:`wal`,retentionPeriodDays:30}},{conditions:[{type:`ArchiveReady`,status:`True`,reason:`Archiving`}],archive:{history:[{serverName:`orders-pg`,from:`2026-09-01T00:00:00Z`,to:`2026-09-20T15:20:40Z`},{serverName:`orders-pg-r1`,from:`2026-09-22T06:00:00Z`}],reachableFrom:`2026-09-19T08:00:00Z`}},{uid:`uid-orders-pg`})],schedules:[V]}),K=[L(`LogicalBackupRDBMS`,`nightly-20261018`,{clusterRef:{name:`orders`}},{phase:`Completed`,step:`ZeebeBackup`,backupId:1760752800,objectKey:`backups/team-a/orders/1760752800/camunda.dump`,zeebeBackupId:1760752800,version:`8.9.23`,storageSizes:{zeebe:`10Gi`},completionTime:`2026-10-18T02:06:10Z`},{creationTimestamp:`2026-10-18T02:00:00Z`,labels:{[r]:`nightly`}})],q=[L(`PointInTimeRestore`,`undo-bad-deploy`,{clusterRef:{name:`orders`},timestamp:`2026-10-17T15:30:00Z`},{phase:`Completed`,completionTime:`2026-10-17T16:02:00Z`},{creationTimestamp:`2026-10-17T15:41:00Z`})],J={title:`cluster/BackupsTab`,component:C},Y={name:`Elasticsearch with backups and a running restore`,args:{storage:u(R({cluster:B(`camunda-backups`),spec:{storageRef:`orders-es`,backupStorageRef:`backups`},storage:[L(`SecondaryStorageConfig`,`orders-es`,{type:`elasticsearch`,elasticsearch:{endpoint:`http://orders-es:9200`}})],objectStorage:[H],schedules:[V]})),backups:f(B(),U,z),restores:l(B(),W)}},X={name:`PostgreSQL with point-in-time restore`,args:{storage:u(G),backups:f(B(),K,z),restores:l(B(),q)}},Z={name:`No backup storage`,args:{storage:u(R({})),backups:[],restores:[]}},Q=[`ElasticsearchWithBackupsAndARunningRestore`,`PostgreSQLWithPointInTimeRestore`,`NoBackupStorage`],Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "Elasticsearch with backups and a running restore",
  args: {
    storage: backupStorageOf(input({
      cluster: cluster("camunda-backups"),
      spec: {
        storageRef: "orders-es",
        backupStorageRef: "backups"
      },
      storage: [obj("SecondaryStorageConfig", "orders-es", {
        type: "elasticsearch",
        elasticsearch: {
          endpoint: "http://orders-es:9200"
        }
      })],
      objectStorage: [s3Backups],
      schedules: [nightly]
    })),
    backups: backupRowsOf(cluster(), esBackups, now),
    restores: restoreRowsOf(cluster(), esRestores)
  }
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "PostgreSQL with point-in-time restore",
  args: {
    storage: backupStorageOf(pgInput),
    backups: backupRowsOf(cluster(), pgBackups, now),
    restores: restoreRowsOf(cluster(), pgRestores)
  }
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "No backup storage",
  args: {
    storage: backupStorageOf(input({})),
    backups: [],
    restores: []
  }
}`,...Z.parameters?.docs?.source}}}})))()}$();export{Y as ElasticsearchWithBackupsAndARunningRestore,Z as NoBackupStorage,X as PostgreSQLWithPointInTimeRestore,Q as __namedExportsOrder,J as default};