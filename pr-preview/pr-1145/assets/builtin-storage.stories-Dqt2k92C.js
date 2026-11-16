import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,c as r,i,n as a,o,s}from"./table-Db9DUbwD.js";import{n as c,t as l}from"./not-read-TId2yrib.js";import{n as u,t as d}from"./condition-list-K_YruUb0.js";import{n as f,t as p}from"./condition-badge-C6w0ZHRA.js";import{a as m,i as h,r as g}from"./claim-tab-BPTPcCWG.js";import{n as _,t as v}from"./detail-section-C3ovnQI7.js";import{d as y,f as b,h as x,i as S,p as C}from"./request-fixtures-D1FrXT45.js";function w({storage:e,unread:t,now:r}){let c=t.includes(e.kind);if(!e.object&&!c)return null;let u=e.object&&x(e),f=c?`Not known`:E;return(0,T.jsxs)(v,{title:e.kind,note:e.name,region:!0,children:[c&&(0,T.jsx)(l,{kinds:[e.kind]}),u&&(0,T.jsxs)(T.Fragment,{children:[u.errors.map(e=>(0,T.jsx)(`p`,{className:`text-sm text-bad`,children:e},e)),(0,T.jsxs)(h,{children:[(0,T.jsx)(g,{label:`Version`,children:u.version??f}),e.kind===`ElasticsearchCluster`?(0,T.jsx)(g,{label:`Snapshot repository`,children:u.snapshotRepository??f}):(0,T.jsxs)(T.Fragment,{children:[(0,T.jsx)(g,{label:`Archive from`,children:u.archive?.reachableFrom??f}),u.archive?.boundary&&(0,T.jsx)(g,{label:`Archive boundary`,children:u.archive.boundary}),(0,T.jsx)(g,{label:`Recovery request`,children:u.recovery?.requestID??f}),u.recovery?.targetTime&&(0,T.jsx)(g,{label:`Recovery target`,children:u.recovery.targetTime}),u.recovery?.startedAt&&(0,T.jsx)(g,{label:`Recovery started`,children:u.recovery.startedAt})]})]}),u.volumes.length>0?(0,T.jsxs)(`table`,{"aria-label":`Volumes`,className:`w-full text-sm`,children:[(0,T.jsx)(o,{children:(0,T.jsxs)(s,{children:[(0,T.jsx)(n,{children:`Volume`}),(0,T.jsx)(n,{children:`Capacity`})]})}),(0,T.jsx)(a,{children:u.volumes.map((e,t)=>`error`in e?(0,T.jsx)(s,{children:(0,T.jsx)(i,{colSpan:2,className:`whitespace-normal`,children:(0,T.jsx)(`span`,{className:`text-bad`,children:e.error})})},t):(0,T.jsxs)(s,{children:[(0,T.jsx)(i,{className:`font-mono text-xs whitespace-normal wrap-anywhere`,children:e.name}),(0,T.jsx)(i,{children:e.capacity})]},e.name))})]}):!c&&(0,T.jsx)(`p`,{className:`text-sm text-muted`,children:`No volumes reported yet`}),c?(0,T.jsx)(`span`,{className:`flex flex-wrap gap-1.5`,children:(0,T.jsx)(p,{type:`Ready`,unknown:!0})}):(0,T.jsx)(d,{label:`${e.name} conditions`,conditions:u.conditions,now:r})]})]})}var T,E;function D(){return(D=e((()=>{T=t(),u(),f(),r(),m(),_(),c(),C(),E=`Not reported yet`,w.__docgenInfo={description:`BuiltinStorage shows the ElasticsearchCluster or DatabaseServer of a
request of built-in storage, with what its status holds: the version,
the snapshot repository or the archive and the recovery, the volumes,
and the conditions. A field that the status does not hold says "Not
reported yet". unread holds the kinds that the page could not read or
that are out of date: while it holds the kind of storage, the box names
that kind, and for an object that it has, it shows the conditions as Not
known, gives Not known in place of "Not reported yet", and says nothing
of missing volumes. An object that is not
in the namespace shows nothing, unless its kind is unread. now is the
time that the ages count to.`,methods:[],displayName:`BuiltinStorage`,props:{storage:{required:!0,tsType:{name:`union`,raw:`| {
    kind: "ElasticsearchCluster";
    name: string;
    object?: Resource<"ElasticsearchCluster">;
  }
| {
    kind: "DatabaseServer";
    name: string;
    object?: Resource<"DatabaseServer">;
  }`,elements:[{name:`signature`,type:`object`,raw:`{
  kind: "ElasticsearchCluster";
  name: string;
  object?: Resource<"ElasticsearchCluster">;
}`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"ElasticsearchCluster"`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`object`,value:{name:`Resource`,elements:[{name:`literal`,value:`"ElasticsearchCluster"`}],raw:`Resource<"ElasticsearchCluster">`,required:!1}}]}},{name:`signature`,type:`object`,raw:`{
  kind: "DatabaseServer";
  name: string;
  object?: Resource<"DatabaseServer">;
}`,signature:{properties:[{key:`kind`,value:{name:`literal`,value:`"DatabaseServer"`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`object`,value:{name:`Resource`,elements:[{name:`literal`,value:`"DatabaseServer"`}],raw:`Resource<"DatabaseServer">`,required:!1}}]}}]},description:``},unread:{required:!0,tsType:{name:`unknown`},description:``},now:{required:!0,tsType:{name:`string`},description:``}}}})))()}var O,k,A,j,M;function N(){return(N=e((()=>{D(),y(),O={title:`cluster/BuiltinStorage`,component:w,args:{unread:[],now:b}},k={name:`ElasticsearchCluster`,args:{storage:{kind:`ElasticsearchCluster`,name:`orders-es`,object:{apiVersion:`core.camunda.io/v1`,kind:`ElasticsearchCluster`,metadata:{namespace:`team-a`,name:`orders-es`},status:{version:`8.19.2`,snapshotRepository:`camunda`,volumes:[{name:`data-orders-es-0`,capacity:`32Gi`},{name:`data-orders-es-1`,capacity:`32Gi`}],conditions:[S(`Ready`,`True`,`Healthy`)]}}}}},A={name:`DatabaseServer with an archive`,args:{storage:{kind:`DatabaseServer`,name:`orders-db`,object:{apiVersion:`core.camunda.io/v1`,kind:`DatabaseServer`,metadata:{namespace:`team-a`,name:`orders-db`},status:{version:`17.5`,archive:{reachableFrom:`2026-11-10T09:58:00Z`},recovery:{requestID:`3f2b1c4d-5e6a-4b7c-8d9e-0f1a2b3c4d5e`,targetTime:`2026-11-15T07:30:00Z`,startedAt:`2026-11-15T07:31:00Z`,contract:`orders-db`,requestedBy:`orders`},volumes:[{name:`orders-db-1`,capacity:`10Gi`}],conditions:[S(`Ready`,`False`,`ArchiveFailing`,`The plugin could not upload the write-ahead log for 6 minutes`)]}}}}},j={name:`ElasticsearchCluster not read`,args:{storage:{kind:`ElasticsearchCluster`,name:`orders-es`},unread:[`ElasticsearchCluster`]}},M=[`ElasticsearchCluster`,`DatabaseServerWithArchive`,`NotReadYet`],k.parameters={...k.parameters,docs:{...k.parameters?.docs,source:{originalSource:`{
  name: "ElasticsearchCluster",
  args: {
    storage: {
      kind: "ElasticsearchCluster",
      name: "orders-es",
      object: {
        apiVersion: "core.camunda.io/v1",
        kind: "ElasticsearchCluster",
        metadata: {
          namespace: "team-a",
          name: "orders-es"
        },
        status: {
          version: "8.19.2",
          snapshotRepository: "camunda",
          volumes: [{
            name: "data-orders-es-0",
            capacity: "32Gi"
          }, {
            name: "data-orders-es-1",
            capacity: "32Gi"
          }],
          conditions: [fixtureCondition("Ready", "True", "Healthy")]
        }
      } as unknown as Resource<"ElasticsearchCluster">
    }
  }
}`,...k.parameters?.docs?.source}}},A.parameters={...A.parameters,docs:{...A.parameters?.docs,source:{originalSource:`{
  name: "DatabaseServer with an archive",
  args: {
    storage: {
      kind: "DatabaseServer",
      name: "orders-db",
      object: {
        apiVersion: "core.camunda.io/v1",
        kind: "DatabaseServer",
        metadata: {
          namespace: "team-a",
          name: "orders-db"
        },
        status: {
          version: "17.5",
          archive: {
            reachableFrom: "2026-11-10T09:58:00Z"
          },
          recovery: {
            requestID: "3f2b1c4d-5e6a-4b7c-8d9e-0f1a2b3c4d5e",
            targetTime: "2026-11-15T07:30:00Z",
            startedAt: "2026-11-15T07:31:00Z",
            contract: "orders-db",
            requestedBy: "orders"
          },
          volumes: [{
            name: "orders-db-1",
            capacity: "10Gi"
          }],
          conditions: [fixtureCondition("Ready", "False", "ArchiveFailing", "The plugin could not upload the write-ahead log for 6 minutes")]
        }
      } as unknown as Resource<"DatabaseServer">
    }
  }
}`,...A.parameters?.docs?.source}}},j.parameters={...j.parameters,docs:{...j.parameters?.docs,source:{originalSource:`{
  name: "ElasticsearchCluster not read",
  args: {
    storage: {
      kind: "ElasticsearchCluster",
      name: "orders-es"
    },
    unread: ["ElasticsearchCluster"]
  }
}`,...j.parameters?.docs?.source}}}})))()}N();export{A as DatabaseServerWithArchive,k as ElasticsearchCluster,j as NotReadYet,M as __namedExportsOrder,O as default};