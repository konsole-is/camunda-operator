import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,i as r,n as i,r as a}from"./claim-tab-CTWWTJtw.js";import{n as o,t as s}from"./status-chip-DiVYkXFw.js";import{_ as c,b as l,g as u,h as d,t as f,x as p,y as m}from"./usage-CY4_bYTl.js";import{o as h,r as g}from"./rows-D8aPniFP.js";import{a as _,c as v,f as y,l as b,m as x,n as S,o as C,p as w,s as T,t as E,u as D}from"./fixtures-BiGOpeGd.js";import{n as O,t as k}from"./users-table-CQ6XKGM8.js";function A({secret:e}){return(0,j.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-1`,children:[(0,j.jsx)(`span`,{className:`font-mono text-sm [overflow-wrap:anywhere]`,children:e.namespace?`${e.namespace}/${e.name}`:e.name}),(0,j.jsx)(`dl`,{className:`grid grid-cols-[max-content_1fr] gap-x-3 gap-y-0.5 text-xs`,children:e.keys.map(([e,t])=>(0,j.jsxs)(`div`,{className:`contents`,children:[(0,j.jsx)(`dt`,{className:`text-muted`,children:e}),(0,j.jsx)(`dd`,{className:`font-mono text-text-2 [overflow-wrap:anywhere]`,children:t})]},e))})]})}var j;function M(){return(M=e((()=>{j=t(),A.__docgenInfo={description:`SecretRefView shows one Secret reference: the Secret, with its namespace
when it has one, and each key that the contract reads. It shows names
and keys only, never a value.`,methods:[],displayName:`SecretRefView`,props:{secret:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  namespace?: string;
  name: string;
  keys: readonly (readonly [label: string, key: string])[];
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!1}},{key:`name`,value:{name:`string`,required:!0}},{key:`keys`,value:{name:`unknown`,required:!0}}]}},description:``}}}})))()}function N({target:e,sources:t}){let n=v(t,e.kind).find(t=>t.metadata.name===e.name&&(e.namespace===void 0||t.metadata.namespace===e.namespace)),o=T.find(t=>t.kind===e.kind).label,f=t.errors.map(e=>(0,P.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind));if(!n)return(0,P.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[f,!h(t).includes(e.kind)&&(0,P.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`The contract `,e.name,` does not exist.`]})]});let g=x(n),_=c(n,C(t),t.planes),b=u(t).get(d(e))??[],S=m(e.kind,h(t));return(0,P.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,P.jsxs)(`header`,{className:`flex flex-col gap-2`,children:[(0,P.jsx)(`p`,{className:`text-sm text-muted`,children:o}),(0,P.jsxs)(`div`,{className:`flex flex-wrap items-center gap-3`,children:[(0,P.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em] [overflow-wrap:anywhere]`,children:e.name}),(0,P.jsx)(s,{condition:g})]}),e.namespace!==void 0&&(0,P.jsx)(`p`,{className:`font-mono text-sm text-muted`,children:e.namespace})]}),f,(0,P.jsxs)(`div`,{className:`grid gap-4 lg:grid-cols-2`,children:[(0,P.jsx)(i,{label:`Status`,children:g?(0,P.jsxs)(r,{children:[(0,P.jsx)(a,{label:`Reason`,children:g.reason||`None`}),(0,P.jsx)(a,{label:`Message`,children:g.message||`None`})]}):(0,P.jsx)(`p`,{className:`text-sm text-muted`,children:`No status yet`})}),(0,P.jsx)(i,{label:`Writer`,children:(0,P.jsxs)(`div`,{className:`flex flex-col gap-1 text-sm`,children:[(0,P.jsx)(`p`,{children:p(_)}),!l(_)&&(0,P.jsxs)(`p`,{className:`text-muted`,children:[`Managed by `,p(_),`. Change it there.`]})]})})]}),(0,P.jsx)(i,{label:`Settings`,children:(0,P.jsx)(r,{children:y(n).map(e=>(0,P.jsx)(a,{label:e.label,note:e.note,children:e.secret?(0,P.jsx)(A,{secret:e.secret}):e.value},e.label))})}),(0,P.jsx)(i,{label:`Used by`,children:(0,P.jsx)(k,{users:b,unread:S})})]})}var P;function F(){return(F=e((()=>{P=t(),n(),o(),g(),f(),_(),w(),M(),D(),b(),O(),N.__docgenInfo={description:`ConnectionDetail is the page of one contract: its own Ready, its writer,
its settings, and the objects that use it. A contract that sources does
not hold shows as missing, unless its kind could not be read. Each of the
read errors shows above.`,methods:[],displayName:`ConnectionDetail`,props:{target:{required:!0,tsType:{name:`ContractRef`},description:``},sources:{required:!0,tsType:{name:`intersection`,raw:`ConnectionSources & { errors: ReadError[] }`,elements:[{name:`intersection`,raw:`ContractUserSources & {
  objectStorageConfigs: ObjectStorageConfig[];
  databaseServerConfigs: DatabaseServerConfig[];
  managementAuthConfigs: ManagementAuthConfig[];
  requests: PluginRequest[];
}`,elements:[{name:`ContractUserSources`},{name:`signature`,type:`object`,raw:`{
  objectStorageConfigs: ObjectStorageConfig[];
  databaseServerConfigs: DatabaseServerConfig[];
  managementAuthConfigs: ManagementAuthConfig[];
  requests: PluginRequest[];
}`,signature:{properties:[{key:`objectStorageConfigs`,value:{name:`Array`,elements:[{name:`ObjectStorageConfig`}],raw:`ObjectStorageConfig[]`,required:!0}},{key:`databaseServerConfigs`,value:{name:`Array`,elements:[{name:`DatabaseServerConfig`}],raw:`DatabaseServerConfig[]`,required:!0}},{key:`managementAuthConfigs`,value:{name:`Array`,elements:[{name:`ManagementAuthConfig`}],raw:`ManagementAuthConfig[]`,required:!0}},{key:`requests`,value:{name:`Array`,elements:[{name:`PluginRequest`}],raw:`PluginRequest[]`,required:!0}}]}}]},{name:`signature`,type:`object`,raw:`{ errors: ReadError[] }`,signature:{properties:[{key:`errors`,value:{name:`Array`,elements:[{name:`ReadError`}],raw:`ReadError[]`,required:!0}}]}}]},description:``}}}})))()}var I,L,R,z,B,V,H,U,W;function G(){return(G=e((()=>{F(),S(),I={title:`setup/connections/ConnectionDetail`,component:N,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`shared-es`},sources:{...E(),errors:[]}}},L={name:`Created by hand`},R={name:`Managed by an Elasticsearch cluster`,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`orders-es`}}},z={name:`Written by a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`orders-backup`}}},B={name:`Kept from a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`old-backup`}}},V={name:`Management Identity of a plane`,args:{target:{kind:`ManagementAuthConfig`,name:`plane-a-optimize`}}},H={name:`Database server with default keys`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`}}},U={name:`Not ready`,args:{target:{kind:`DatabaseConfig`,namespace:`team-a`,name:`orders-db`},sources:{...E(),databaseConfigs:E().databaseConfigs.map(e=>e.metadata.name===`orders-db`?{...e,status:{conditions:[{type:`Ready`,status:`False`,reason:`MissingSecret`,message:`Secret "orders-db" has no key "pass"`,lastTransitionTime:`2026-11-10T09:00:00Z`}]}}:e),errors:[]}}},W=[`CreatedByHand`,`ManagedByAnElasticsearchCluster`,`WrittenByARegistration`,`KeptFromARegistration`,`ManagementIdentityOfAPlane`,`DatabaseServerWithDefaultKeys`,`NotReady`],L.parameters={...L.parameters,docs:{...L.parameters?.docs,source:{originalSource:`{
  name: "Created by hand"
}`,...L.parameters?.docs?.source}}},R.parameters={...R.parameters,docs:{...R.parameters?.docs,source:{originalSource:`{
  name: "Managed by an Elasticsearch cluster",
  args: {
    target: {
      kind: "SecondaryStorageConfig",
      namespace: "team-a",
      name: "orders-es"
    }
  }
}`,...R.parameters?.docs?.source}}},z.parameters={...z.parameters,docs:{...z.parameters?.docs,source:{originalSource:`{
  name: "Written by a registration",
  args: {
    target: {
      kind: "ObjectStorageConfig",
      namespace: "team-a",
      name: "orders-backup"
    }
  }
}`,...z.parameters?.docs?.source}}},B.parameters={...B.parameters,docs:{...B.parameters?.docs,source:{originalSource:`{
  name: "Kept from a registration",
  args: {
    target: {
      kind: "ObjectStorageConfig",
      namespace: "team-a",
      name: "old-backup"
    }
  }
}`,...B.parameters?.docs?.source}}},V.parameters={...V.parameters,docs:{...V.parameters?.docs,source:{originalSource:`{
  name: "Management Identity of a plane",
  args: {
    target: {
      kind: "ManagementAuthConfig",
      name: "plane-a-optimize"
    }
  }
}`,...V.parameters?.docs?.source}}},H.parameters={...H.parameters,docs:{...H.parameters?.docs,source:{originalSource:`{
  name: "Database server with default keys",
  args: {
    target: {
      kind: "DatabaseServerConfig",
      namespace: "team-a",
      name: "postgres"
    }
  }
}`,...H.parameters?.docs?.source}}},U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
  name: "Not ready",
  args: {
    target: {
      kind: "DatabaseConfig",
      namespace: "team-a",
      name: "orders-db"
    },
    sources: {
      ...connectionFixtures(),
      databaseConfigs: connectionFixtures().databaseConfigs.map(c => c.metadata.name === "orders-db" ? {
        ...c,
        status: {
          conditions: [{
            type: "Ready",
            status: "False" as const,
            reason: "MissingSecret",
            message: 'Secret "orders-db" has no key "pass"',
            lastTransitionTime: "2026-11-10T09:00:00Z"
          }]
        }
      } : c),
      errors: []
    }
  }
}`,...U.parameters?.docs?.source}}}})))()}G();export{L as CreatedByHand,H as DatabaseServerWithDefaultKeys,B as KeptFromARegistration,R as ManagedByAnElasticsearchCluster,V as ManagementIdentityOfAPlane,U as NotReady,z as WrittenByARegistration,W as __namedExportsOrder,I as default};