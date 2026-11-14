import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{h as r,r as i}from"./labels-a4ptf10d.js";import{n as a,t as o}from"./createLucideIcon-B55dJ-eE.js";import{o as ee,t as s}from"./units-B_VbZtdu.js";import{a as c,n as l,r as te,t as u}from"./backups-D-a1L1yD.js";import{i as d,o as f,r as ne}from"./status-CBDUVTbC.js";import{a as p,i as m,n as h,o as g,r as _,s as re,t as v}from"./table-DQOdhbmg.js";var y,b;function x(){return(x=e((()=>{a(),y={name:`chevron-right`,size:24,node:[[`path`,{d:`m9 18 6-6-6-6`,key:`mthhwq`}]]},y.node,b=o(y)})))()}function S({storage:e,backups:t,restores:n,actions:r,rowActions:i}){return(0,M.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[r&&(0,M.jsx)(`div`,{className:`flex flex-wrap items-center justify-end gap-2`,children:r}),(0,M.jsxs)(`div`,{className:`grid gap-4 lg:grid-cols-[3fr_2fr]`,children:[(0,M.jsx)(ie,{storage:e}),(0,M.jsx)(E,{schedules:e.schedules})]}),(0,M.jsx)(D,{rows:t,rowActions:i}),(0,M.jsx)(O,{rows:n})]})}function ie({storage:e}){return(0,M.jsxs)(`section`,{"aria-label":`Backup storage`,className:P,children:[(0,M.jsx)(`h2`,{className:F,children:`Backup storage`}),e.kind===`none`?(0,M.jsx)(`p`,{className:`text-sm text-text-2`,children:`This cluster has no backup storage. Backups need a bucket from the Backup storage section.`}):(0,M.jsxs)(`dl`,{className:`grid grid-cols-[max-content_1fr] gap-x-4 gap-y-2 text-sm`,children:[e.missing&&(0,M.jsx)(k,{label:`Secondary storage`,children:(0,M.jsxs)(`span`,{className:`text-bad`,children:[e.missing,` not found`]})}),(0,M.jsx)(k,{label:`Bucket`,children:j(e.bucket)}),e.kind===`elasticsearch`&&(0,M.jsx)(k,{label:`Snapshot repository`,children:e.repository??(0,M.jsx)(`span`,{className:`text-warn`,children:`Not registered yet`})}),e.database&&(0,M.jsx)(C,{database:e.database})]})]})}function C({database:e}){if(e.missing)return(0,M.jsx)(k,{label:`Database server`,children:(0,M.jsxs)(`span`,{className:`text-bad`,children:[e.missing,` not found`]})});let{pitr:t,archive:n}=e;return(0,M.jsxs)(M.Fragment,{children:[(0,M.jsxs)(k,{label:`Database server`,children:[e.serverConfig,(0,M.jsx)(`span`,{className:`block break-all text-xs text-muted`,children:e.endpoint})]}),(0,M.jsx)(k,{label:`Point-in-time restore`,children:t?.enabled?[`On`,t.retentionDays!==void 0&&`${t.retentionDays} days`,t.recovery===`operator`?`recovery by the operator`:`recovery outside the operator`].filter(Boolean).join(`, `):`Off`}),n&&(0,M.jsxs)(M.Fragment,{children:[(0,M.jsx)(k,{label:`Archive`,children:(0,M.jsx)(A,{tone:n.state.tone,title:n.state.message,children:n.state.label})}),n.bucket&&(0,M.jsx)(k,{label:`Archive bucket`,children:j(n.bucket)}),n.window&&(0,M.jsx)(k,{label:`Restore window`,children:(0,M.jsx)(w,{window:n.window})})]})]})}function w({window:e}){return e.length===0?(0,M.jsx)(M.Fragment,{children:`No restore point yet`}):(0,M.jsx)(`ul`,{className:`flex flex-col gap-0.5`,children:e.map((t,n)=>(0,M.jsxs)(N.Fragment,{children:[n>0&&!T(e[n-1],t)&&(0,M.jsx)(`li`,{className:`text-xs text-muted`,children:`No restore point in this gap`}),(0,M.jsxs)(`li`,{children:[s(t.from),` to`,` `,t.to?s(t.to):`now`,t.failing&&(0,M.jsx)(`span`,{className:`block text-xs text-warn`,children:`Uploads are failing after this point`})]})]},t.from))})}function T(e,t){return e.to!==void 0&&Date.parse(e.to)===Date.parse(t.from)}function E({schedules:e}){return(0,M.jsxs)(`section`,{"aria-label":`Backup schedule`,className:P,children:[(0,M.jsx)(`h2`,{className:F,children:`Schedule`}),e.length===0?(0,M.jsx)(`p`,{className:`text-sm text-muted`,children:`No backup schedule. Backups run only when someone starts one.`}):(0,M.jsx)(`ul`,{className:`flex flex-col gap-2 text-sm`,children:e.map(e=>(0,M.jsxs)(`li`,{children:[(0,M.jsx)(`span`,{className:`font-medium`,children:e.name}),`:`,` `,(0,M.jsx)(`code`,{className:`rounded bg-panel-2 px-1 text-xs`,children:e.cron}),`, keeps `,e.completed,` completed and `,e.failed,` failed`,e.lastBackup&&(0,M.jsxs)(`span`,{className:`block text-xs text-muted`,children:[`Last backup `,e.lastBackup]})]},e.name))})]})}function D({rows:e,rowActions:t}){let[n,r]=(0,N.useState)(),i=t?9:8;return(0,M.jsxs)(`section`,{"aria-label":`Backups`,className:P,children:[(0,M.jsx)(`h2`,{className:F,children:`Backups`}),e.length===0?(0,M.jsx)(`p`,{className:`text-sm text-muted`,children:`No backups yet.`}):(0,M.jsxs)(v,{label:`Backup list`,children:[(0,M.jsx)(p,{children:(0,M.jsxs)(g,{children:[(0,M.jsx)(m,{children:`Name`}),(0,M.jsx)(m,{children:`Origin`}),(0,M.jsx)(m,{children:`Started`}),(0,M.jsx)(m,{children:`Finished`}),(0,M.jsx)(m,{children:`Duration`}),(0,M.jsx)(m,{children:`Phase`}),(0,M.jsx)(m,{children:`Step`}),(0,M.jsx)(m,{children:`Version`}),t&&(0,M.jsx)(m,{children:`Actions`})]})}),(0,M.jsx)(h,{children:e.map(e=>{let a=`backup-${e.kind}-${e.name}`,o=n===a;return(0,M.jsxs)(N.Fragment,{children:[(0,M.jsxs)(g,{children:[(0,M.jsx)(_,{children:(0,M.jsxs)(`button`,{type:`button`,"aria-expanded":o,"aria-controls":a,"aria-label":`Details of ${e.name}`,onClick:()=>r(o?void 0:a),className:`flex items-center gap-1.5 rounded font-medium outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50`,children:[(0,M.jsx)(b,{"aria-hidden":!0,className:`size-3.5 text-muted transition-transform motion-reduce:transition-none ${o?`rotate-90`:``}`}),e.name]})}),(0,M.jsx)(_,{className:`text-text-2`,children:e.origin}),(0,M.jsx)(_,{className:`text-text-2`,children:e.started}),(0,M.jsx)(_,{className:`text-text-2`,children:e.finished}),(0,M.jsx)(_,{className:`tabular-nums`,children:e.duration}),(0,M.jsx)(_,{children:(0,M.jsx)(A,{tone:e.tone,children:e.phase})}),(0,M.jsx)(_,{className:`text-text-2`,children:e.step}),(0,M.jsx)(_,{className:`text-text-2`,children:e.version}),t&&(0,M.jsx)(_,{className:`min-w-56 whitespace-normal`,children:t(e)})]}),o&&(0,M.jsx)(g,{id:a,className:`hover:bg-transparent`,children:(0,M.jsx)(_,{colSpan:i,className:`bg-panel-2/50`,children:e.details.length===0?(0,M.jsx)(`p`,{className:`text-sm text-muted`,children:`No details yet.`}):(0,M.jsx)(`dl`,{className:`grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-sm whitespace-normal`,children:e.details.map(e=>(0,M.jsx)(k,{label:e.label,children:(0,M.jsx)(`span`,{className:`break-all`,children:e.value})},e.label))})})})]},a)})})]})]})}function O({rows:e}){return(0,M.jsxs)(`section`,{"aria-label":`Restores`,className:P,children:[(0,M.jsx)(`h2`,{className:F,children:`Restores`}),e.length===0?(0,M.jsx)(`p`,{className:`text-sm text-muted`,children:`No restores.`}):(0,M.jsxs)(v,{label:`Restore list`,children:[(0,M.jsx)(p,{children:(0,M.jsxs)(g,{children:[(0,M.jsx)(m,{children:`Name`}),(0,M.jsx)(m,{children:`Type`}),(0,M.jsx)(m,{children:`Source`}),(0,M.jsx)(m,{children:`Started`}),(0,M.jsx)(m,{children:`Phase`})]})}),(0,M.jsx)(h,{children:e.map(e=>(0,M.jsxs)(g,{children:[(0,M.jsx)(_,{className:`font-medium`,children:e.name}),(0,M.jsx)(_,{className:`text-text-2`,children:e.label}),(0,M.jsx)(_,{className:`text-text-2`,children:e.source}),(0,M.jsx)(_,{className:`text-text-2`,children:e.started}),(0,M.jsxs)(_,{className:`whitespace-normal`,children:[(0,M.jsx)(A,{tone:e.tone,children:e.phase}),e.failureMessage&&(0,M.jsx)(`span`,{className:`block text-xs text-text-2`,children:e.failureMessage})]})]},`${e.kind}-${e.name}`))})]})]})}function k({label:e,children:t}){return(0,M.jsxs)(M.Fragment,{children:[(0,M.jsx)(`dt`,{className:`text-muted`,children:e}),(0,M.jsx)(`dd`,{className:`min-w-0`,children:t})]})}function A({tone:e,title:t,children:n}){return(0,M.jsxs)(`span`,{title:t,className:`inline-flex items-center gap-1.5 ${d[e]}`,children:[(0,M.jsx)(`span`,{"aria-hidden":!0,className:`inline-block size-2 shrink-0 rounded-full ${ne[e]}`}),n]})}function j(e){return e.location??`${e.name} (not found)`}var M,N,P,F;function I(){return(I=e((()=>{M=n(),x(),N=t(),f(),ee(),re(),P=`flex flex-col gap-3 rounded-lg border border-border bg-panel p-4`,F=`font-heading text-base font-semibold`,S.__docgenInfo={description:`BackupsTab is the content of the Backups tab: the backup storage and the
schedules, the backups, and the restores of one cluster. actions is the
node of the cluster.backups.actions slot, shown in the header. When
rowActions is set, the backup table has an Actions column with its node
for each row.`,methods:[],displayName:`BackupsTab`,props:{storage:{required:!0,tsType:{name:`union`,raw:`| { kind: "none"; schedules: ScheduleView[] }
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
}`,signature:{properties:[{key:`kind`,value:{name:`BackupKind`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`uid`,value:{name:`string`,required:!1}},{key:`origin`,value:{name:`union`,raw:`"Scheduled" | "Manual"`,elements:[{name:`literal`,value:`"Scheduled"`},{name:`literal`,value:`"Manual"`}],required:!0}},{key:`started`,value:{name:`string`,required:!0}},{key:`finished`,value:{name:`string`,required:!0}},{key:`duration`,value:{name:`string`,required:!0}},{key:`phase`,value:{name:`string`,required:!0}},{key:`tone`,value:{name:`StatusTone`,required:!0}},{key:`step`,value:{name:`string`,required:!0}},{key:`version`,value:{name:`string`,required:!0}},{key:`details`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ label: string; value: string }`,signature:{properties:[{key:`label`,value:{name:`string`,required:!0}},{key:`value`,value:{name:`string`,required:!0}}]}}],raw:`Detail[]`,required:!0}}]}},name:`row`}],return:{name:`ReactNode`}}},description:``}}}})))()}function L(e,t,n,r={},i={}){return{kind:e,metadata:{namespace:`team-a`,name:t,...i},spec:n,status:r}}function R(e){return{cluster:B(),spec:{},storage:[],objectStorage:[],databaseConfigs:[],databaseServerConfigs:[],databaseServers:[],databaseServerPresets:[],schedules:[],now:z,...e}}var z,B,V,H,U,W,G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{r(),te(),I(),z=`2026-10-18T12:00:00Z`,B=e=>L(`CamundaCluster`,`orders`,{},e?{management:{backupRepository:e}}:{}),V=L(`BackupSchedule`,`nightly`,{clusterRef:{name:`orders`},schedule:`0 2 * * *`,retained:{completed:7,failed:3}},{lastBackupName:`nightly-20261018`}),H=L(`ObjectStorageConfig`,`backups`,{type:`S3`,s3:{bucketName:`acme-backups`,basePath:`orders`,region:`eu-west-1`}}),U=[L(`LogicalBackupElasticsearch`,`nightly-20261018`,{clusterRef:{name:`orders`}},{phase:`Completed`,step:`ResumeExporting`,backupId:1760752800,repository:`camunda-backups`,partitionsCount:3,version:`8.9.23`,storageSizes:{zeebe:`10Gi`,elasticsearch:21474836480},completionTime:`2026-10-18T02:14:22Z`},{creationTimestamp:`2026-10-18T02:00:00Z`,labels:{[i]:`nightly`}}),L(`LogicalBackupElasticsearch`,`before-upgrade`,{clusterRef:{name:`orders`}},{phase:`Failed`,step:`SnapshotRecords`,backupId:176077e4,repository:`camunda-backups`,failureMessage:`Elasticsearch refused the snapshot: the repository is read-only`},{creationTimestamp:`2026-10-18T06:46:40Z`}),L(`LogicalBackupElasticsearch`,`manual-now`,{clusterRef:{name:`orders`}},{phase:`Running`,step:`BackupRuntime`,version:`8.9.23`},{creationTimestamp:`2026-10-18T11:48:00Z`})],W=[L(`LogicalRestoreElasticsearch`,`restore-staging`,{targetClusterRef:{name:`orders`},backupRef:{name:`nightly-20261018`}},{phase:`RestoringPrimaryStorage`},{creationTimestamp:`2026-10-18T11:50:00Z`})],G=R({spec:{storageRef:`orders-db-storage`,backupStorageRef:`backups`},storage:[L(`SecondaryStorageConfig`,`orders-db-storage`,{type:`rdbms`,rdbms:{databaseConfigRef:`orders-db`}})],objectStorage:[H,L(`ObjectStorageConfig`,`wal`,{type:`GCS`,gcs:{bucketName:`acme-wal`,basePath:`orders-pg`}})],databaseConfigs:[L(`DatabaseConfig`,`orders-db`,{serverRef:`orders-pg`})],databaseServerConfigs:[L(`DatabaseServerConfig`,`orders-pg`,{host:`orders-pg-rw.team-a.svc`,port:5432,pitr:{enabled:!0,recovery:`operator`,retentionPeriodDays:30}},{},{ownerReferences:[{apiVersion:`core.camunda.io/v1`,kind:`DatabaseServer`,name:`orders-pg`,uid:`uid-orders-pg`,controller:!0}]})],databaseServers:[L(`DatabaseServer`,`orders-pg`,{databaseServerConfig:`orders-pg`,archive:{objectStorageRef:`wal`,retentionPeriodDays:30}},{conditions:[{type:`ArchiveReady`,status:`True`,reason:`Archiving`}],archive:{history:[{serverName:`orders-pg`,from:`2026-09-01T00:00:00Z`,to:`2026-09-20T15:20:40Z`},{serverName:`orders-pg-r1`,from:`2026-09-22T06:00:00Z`}],reachableFrom:`2026-09-19T08:00:00Z`}},{uid:`uid-orders-pg`})],schedules:[V]}),K=[L(`LogicalBackupRDBMS`,`nightly-20261018`,{clusterRef:{name:`orders`}},{phase:`Completed`,step:`ZeebeBackup`,backupId:1760752800,objectKey:`backups/team-a/orders/1760752800/camunda.dump`,zeebeBackupId:1760752800,version:`8.9.23`,storageSizes:{zeebe:`10Gi`},completionTime:`2026-10-18T02:06:10Z`},{creationTimestamp:`2026-10-18T02:00:00Z`,labels:{[i]:`nightly`}})],q=[L(`PointInTimeRestore`,`undo-bad-deploy`,{clusterRef:{name:`orders`},timestamp:`2026-10-17T15:30:00Z`},{phase:`Completed`,completionTime:`2026-10-17T16:02:00Z`},{creationTimestamp:`2026-10-17T15:41:00Z`})],J={title:`cluster/BackupsTab`,component:S},Y={name:`Elasticsearch with backups and a running restore`,args:{storage:l(R({cluster:B(`camunda-backups`),spec:{storageRef:`orders-es`,backupStorageRef:`backups`},storage:[L(`SecondaryStorageConfig`,`orders-es`,{type:`elasticsearch`,elasticsearch:{endpoint:`http://orders-es:9200`}})],objectStorage:[H],schedules:[V]})),backups:u(B(),U,z),restores:c(B(),W)}},X={name:`PostgreSQL with point-in-time restore`,args:{storage:l(G),backups:u(B(),K,z),restores:c(B(),q)}},Z={name:`No backup storage`,args:{storage:l(R({})),backups:[],restores:[]}},Q=[`ElasticsearchWithBackupsAndARunningRestore`,`PostgreSQLWithPointInTimeRestore`,`NoBackupStorage`],Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
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