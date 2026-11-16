import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{j as ee,t as r}from"./policy-DFk5vpS8.js";import{r as i,t as te}from"./typed-confirm-CDAM2A72.js";import{a,i as o,n as s,r as c}from"./claim-tab-CjhKJp9p.js";import{n as l,r as ne,t as re}from"./toast-BDs8vH4K.js";import{n as u,t as ie}from"./status-chip-BDeww2tv.js";import{n as d,t as ae}from"./change-review-CREwqSXf.js";import{n as f,t as oe}from"./change-bar-C0qJbcyB.js";import"./claim-edit-D6OFTx2q.js";import{_ as se,b as ce,g as le,h as ue,t as p,x as m,y as de}from"./usage-XicndySu.js";import{a as fe,o as h,r as g}from"./rows-DuAU-Tbp.js";import{n as _}from"./cluster-edit-Gl8mWWc3.js";import{i as v,n as y,r as b}from"./namespaced-secret-ref-picker-Da4Af3j_.js";import{o as x,t as pe}from"./form-parts-BxM9vwQQ.js";import{n as me,t as S}from"./use-setup-editor-PgXHZf1b.js";import{a as he,n as C,o as w,t as T}from"./fixtures-PmE5ozkW.js";import{a as ge,c as E,i as _e,l as D,o as ve,r as O,s as k}from"./connection-list-BFilP7xs.js";import{a as A,c as j,i as ye,n as be,o as xe,r as Se,s as Ce,t as we}from"./draft-BksvNeYb.js";import{n as M,t as Te}from"./users-table-CIvylBLg.js";function Ee(e,t){let n=t.map(t=>`${t.kind} ${t.namespace}/${t.name} ${N[e][t.kind]??`reads the changed contract.`}`);return n.length>0?n:[`No resource uses this contract.`]}var N;function P(){return(P=e((()=>{N={ObjectStorageConfig:{CamundaCluster:`restarts its pods, all but Connectors.`,CamundaClusterClaim:`passes the change to its cluster, which restarts its pods.`,ElasticsearchCluster:`registers its snapshot repository again. A change of the workload identity restarts its pods.`,DatabaseServer:`applies its archive again. A new bucket, base path, region, or endpoint moves the archive: a backup from before the move no longer counts for a rollback.`,LogicalBackupElasticsearch:`fails while it runs when the bucket, base path, region, or endpoint changes. Its delete then cannot reach the old location.`,LogicalBackupRDBMS:`fails while it runs when the bucket, base path, region, or endpoint changes. Its delete then leaves the dump in the old location.`,PluginRequest:`passes the change to the cluster of its claim, which restarts its pods.`},SecondaryStorageConfig:{CamundaCluster:`restarts all its pods and the pods of its Optimize. A new endpoint, host, port, or database is a new storage backend for the cluster.`,CamundaClusterClaim:`passes the change to its cluster, which restarts its pods.`,LogicalBackupElasticsearch:`fails while it runs when the endpoint changes.`,PluginRequest:`passes the change to the cluster of its claim, which restarts its pods.`},DatabaseServerConfig:{Database:`waits until the operator checks the server again. On a new server it creates its database and roles there. The database on the old server stays.`,DatabaseConfig:`checks the server again. A cluster or management plane that uses this database config restarts its pods.`,DatabaseServer:`reads the changed contract for the rollback that runs.`,PointInTimeRestore:`fails when the host or the port changes before its recovery step.`},DatabaseConfig:{SecondaryStorageConfig:`checks the database config again. A cluster that uses this secondary storage restarts its pods. A new database name is a new storage backend for that cluster.`,CamundaManagementCluster:`restarts its pods: Identity, Console, and Web Modeler for the Keycloak or the Identity database, the Web Modeler REST API for the Web Modeler database. Keycloak gets the new database settings.`,CamundaManagementClusterClaim:`passes the change to its management plane, which restarts its pods.`,PluginRequest:`passes the change to the management plane of its claim, which restarts its pods.`},ManagementAuthConfig:{CamundaOptimize:`restarts its pods.`,CamundaClusterClaim:`passes the change to the Optimize of its cluster, which restarts its pods.`}}})))()}function F({secret:e}){return(0,I.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-1`,children:[(0,I.jsx)(`span`,{className:`font-mono text-sm [overflow-wrap:anywhere]`,children:e.namespace?`${e.namespace}/${e.name}`:e.name}),(0,I.jsx)(`dl`,{className:`grid grid-cols-[max-content_1fr] gap-x-3 gap-y-0.5 text-xs`,children:e.keys.map(([e,t])=>(0,I.jsxs)(`div`,{className:`contents`,children:[(0,I.jsx)(`dt`,{className:`text-muted`,children:e}),(0,I.jsx)(`dd`,{className:`font-mono text-text-2 [overflow-wrap:anywhere]`,children:t})]},e))})]})}var I;function L(){return(L=e((()=>{I=n(),F.__docgenInfo={description:`SecretRefView shows one Secret reference: the Secret, with its namespace
when it has one, and each key that the contract reads. It shows names
and keys only, never a value.`,methods:[],displayName:`SecretRefView`,props:{secret:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  namespace?: string;
  name: string;
  keys: readonly (readonly [label: string, key: string])[];
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!1}},{key:`name`,value:{name:`string`,required:!0}},{key:`keys`,value:{name:`unknown`,required:!0}}]}},description:``}}}})))()}function De({target:e,sources:t,mode:n,actions:r}){let i=k(t,e.kind),a=i.find(t=>t.metadata.name===e.name&&(e.namespace===void 0||t.metadata.namespace===e.namespace)),l=ge.find(t=>t.kind===e.kind),u=me({live:a,isNew:!1,listed:i.filter(t=>t.metadata.namespace===e.namespace).map(e=>e.metadata.name??``),draftOf:ye,listHref:`/setup/connections?tab=${l.value}`,hrefOf:(t,n)=>ve({kind:e.kind,namespace:n,name:t})}),[d,f]=(0,Oe.useState)(!1),{toast:p,show:g,dismiss:_}=ne(),v=de(e.kind,h(t)),y=le(t).get(ue(e))??[],{base:b,draft:x,reviewing:S,locked:C}=u,w=a??b,T=w&&se(w,_e(t),t.planes),E=r!==void 0&&T!==void 0&&ce(T),D=S?.kind===`delete`;S&&!d&&(!E||v.length>0||D&&y.length>0)&&u.cancel();let O=t.errors.map(e=>(0,R.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind));if(!w)return(0,R.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[O,!h(t).includes(e.kind)&&(0,R.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`The contract `,e.name,` does not exist.`]})]});let A=ee(w.status?.conditions),j=Se(e.kind,x),M=v.length>0?`A change is possible when the fleet manager can read ${v.join(`, `)}.`:void 0,N=b?we(b,x):0,P=n=>h(t).includes(n)?void 0:k(t,n).filter(t=>t.metadata.namespace===e.namespace).map(e=>e.metadata.name??``);return(0,R.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,R.jsxs)(`header`,{className:`flex flex-col gap-2`,children:[(0,R.jsx)(`p`,{className:`text-sm text-muted`,children:l.label}),(0,R.jsxs)(`div`,{className:`flex flex-wrap items-center gap-3`,children:[(0,R.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em] [overflow-wrap:anywhere]`,children:e.name}),(0,R.jsx)(ie,{condition:A})]}),e.namespace!==void 0&&(0,R.jsx)(`p`,{className:`font-mono text-sm text-muted`,children:e.namespace})]}),E&&!n&&(0,R.jsx)(`p`,{role:`alert`,className:`text-sm text-bad`,children:`Changes are off: the fleet manager could not read its settings.`}),O,!a&&(0,R.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`The contract `,e.name,` was deleted.`]}),(0,R.jsxs)(`div`,{className:`grid gap-4 lg:grid-cols-2`,children:[(0,R.jsx)(s,{label:`Status`,children:A?(0,R.jsxs)(o,{children:[(0,R.jsx)(c,{label:`Reason`,children:A.reason||`None`}),(0,R.jsx)(c,{label:`Message`,children:A.message||`None`})]}):(0,R.jsx)(`p`,{className:`text-sm text-muted`,children:`No status yet`})}),(0,R.jsx)(s,{label:`Writer`,children:(0,R.jsxs)(`div`,{className:`flex flex-col gap-1 text-sm`,children:[T&&(0,R.jsx)(`p`,{children:m(T)}),T&&!ce(T)&&(0,R.jsxs)(`p`,{className:`text-muted`,children:[`Managed by `,m(T),`. Change it there.`]})]})})]}),E?(0,R.jsx)(`fieldset`,{disabled:C||!a||!n,className:`flex min-w-0 flex-col gap-4`,children:(0,R.jsxs)(pe,{title:`Settings`,children:[M&&(0,R.jsx)(`p`,{className:`text-sm text-muted`,children:M}),(0,R.jsx)(Ce,{kind:e.kind,namespace:e.namespace,draft:x,errors:j,namespaces:t.namespaces.map(e=>e.metadata?.name??``),databaseConfigs:P(`DatabaseConfig`),databaseServerConfigs:P(`DatabaseServerConfig`),onChange:e=>u.setDraft(()=>e)})]})},`settings-${u.generation}`):(0,R.jsx)(s,{label:`Settings`,children:(0,R.jsx)(o,{children:he(w).map(e=>(0,R.jsx)(c,{label:e.label,note:e.note,children:e.secret?(0,R.jsx)(F,{secret:e.secret}):e.value},e.label))})}),(0,R.jsx)(s,{label:`Used by`,children:(0,R.jsx)(Te,{users:y,unread:v})}),E&&a&&(0,R.jsx)(`fieldset`,{disabled:C||!n,className:`flex min-w-0 flex-col gap-4`,children:(0,R.jsx)(pe,{title:`Delete the contract`,children:v.length>0?(0,R.jsx)(`p`,{className:`text-sm text-muted`,children:fe(v)}):y.length>0?(0,R.jsx)(`p`,{className:`text-sm text-muted`,children:`Delete is possible when no resource uses this contract.`}):(0,R.jsxs)(te,{name:e.name,confirmLabel:`Review the delete`,onConfirm:()=>u.review({change:be(a),kind:`delete`}),children:[`The contract `,e.name,` is deleted.`]})})},`delete-${u.generation}`),E&&S&&n?(0,R.jsx)(`section`,{"aria-label":D?`Review the delete`:`Review the contract`,className:`rounded-md border border-border bg-panel p-5`,children:(0,R.jsx)(ae,{change:S.change,mode:n,diffLabel:`Changes to the contract`,effects:D?[`The contract ${e.name} is deleted.`,...n===`git`?[`In Git mode the delete is a commit that removes the file of the contract. GitOps must prune the ${e.kind} to delete it from the cluster.`]:[]]:Ee(e.kind,y),confirm:D?{label:`Delete the contract`,gitLabel:`Push the delete to Git`,danger:!0}:{label:`Save`},savedMessage:D?`The contract ${e.name} is deleted.`:`The contract ${e.name} is saved.`,preview:r.previewChange,apply:r.applyChange,show:(t,n)=>{g(t,n),u.written(t,e.name,e.namespace)},onCancel:u.cancel,onWriting:f})}):E&&N>0&&!C&&b&&(0,R.jsx)(oe,{count:N,blocked:M??j[0]?.message,onDiscard:u.discard,onReview:()=>u.review({change:xe(b,x),kind:`save`})}),p&&(0,R.jsx)(`div`,{className:`fixed right-4 bottom-4 z-20`,children:(0,R.jsx)(re,{result:p,onDismiss:_})})]})}var R,Oe;function ke(){return(ke=e((()=>{R=n(),Oe=t(),a(),_(),f(),d(),u(),l(),i(),x(),g(),S(),p(),O(),j(),A(),w(),r(),P(),L(),D(),E(),M(),De.__docgenInfo={description:`ConnectionDetail is the page of one contract: its own Ready, its writer,
its settings, and the objects that use it. A contract that sources does
not hold shows as missing, unless its kind is not read. One that leaves
sources while the page shows it keeps its last copy and shows as deleted.
Each of the read errors shows above.

With actions, a contract that a person may change (writerEditable) has a
form with the form state of useSetupEditor, and a delete. A save and a
delete each show a review with a dry run before the write. Without mode
the form is disabled. Any other contract keeps the read view.`,methods:[],displayName:`ConnectionDetail`,props:{target:{required:!0,tsType:{name:`ContractRef`},description:``},sources:{required:!0,tsType:{name:`intersection`,raw:`ConnectionSources & { errors: ReadError[] } & SourceReads`,elements:[{name:`intersection`,raw:`ContractUserSources & {
  objectStorageConfigs: ObjectStorageConfig[];
  databaseServerConfigs: DatabaseServerConfig[];
  managementAuthConfigs: ManagementAuthConfig[];
  requests: PluginRequest[];
  planeClaims: CamundaManagementClusterClaim[];
  registrations: PluginRegistration[];
}`,elements:[{name:`ContractUserSources`},{name:`signature`,type:`object`,raw:`{
  objectStorageConfigs: ObjectStorageConfig[];
  databaseServerConfigs: DatabaseServerConfig[];
  managementAuthConfigs: ManagementAuthConfig[];
  requests: PluginRequest[];
  planeClaims: CamundaManagementClusterClaim[];
  registrations: PluginRegistration[];
}`,signature:{properties:[{key:`objectStorageConfigs`,value:{name:`Array`,elements:[{name:`ObjectStorageConfig`}],raw:`ObjectStorageConfig[]`,required:!0}},{key:`databaseServerConfigs`,value:{name:`Array`,elements:[{name:`DatabaseServerConfig`}],raw:`DatabaseServerConfig[]`,required:!0}},{key:`managementAuthConfigs`,value:{name:`Array`,elements:[{name:`ManagementAuthConfig`}],raw:`ManagementAuthConfig[]`,required:!0}},{key:`requests`,value:{name:`Array`,elements:[{name:`PluginRequest`}],raw:`PluginRequest[]`,required:!0}},{key:`planeClaims`,value:{name:`Array`,elements:[{name:`CamundaManagementClusterClaim`}],raw:`CamundaManagementClusterClaim[]`,required:!0}},{key:`registrations`,value:{name:`Array`,elements:[{name:`PluginRegistration`}],raw:`PluginRegistration[]`,required:!0}}]}}]},{name:`signature`,type:`object`,raw:`{ errors: ReadError[] }`,signature:{properties:[{key:`errors`,value:{name:`Array`,elements:[{name:`ReadError`}],raw:`ReadError[]`,required:!0}}]}},{name:`SourceReads`}]},description:``},mode:{required:!1,tsType:{name:`WriteMode`},description:``},actions:{required:!1,tsType:{name:`signature`,type:`object`,raw:`{
  previewChange: (change: Change) => Promise<Preview>;
  applyChange: (change: Change) => Promise<WriteResult>;
}`,signature:{properties:[{key:`previewChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<Preview>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`Preview`}],raw:`Promise<Preview>`}},required:!0}},{key:`applyChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}},required:!0}}]}},description:``}}}})))()}var z,Ae,B,V,je,Me,H,Ne,U,W,G,K,q,J,Y,Pe,X,Z,Q,$,Fe;function Ie(){return(Ie=e((()=>{z=n(),b(),ke(),C(),{expect:Ae,fn:B,userEvent:V,within:je}=__STORYBOOK_MODULE_TEST__,Me=v({"team-a":[{name:`postgres-admin`,keys:[`username`,`password`]},{name:`orders-db`,keys:[`user`,`pass`]}]}),H={previewChange:B(async e=>({objects:[...e.apply,...e.delete??[]].map(e=>{let t=`metadata`in e?{...e.metadata,apiVersion:e.apiVersion,kind:e.kind}:e;return{ref:{apiVersion:t.apiVersion,kind:t.kind,namespace:t.namespace,name:t.name},diff:[{op:`added`,text:`host: pg-2.team-a`}]}}),errors:[]})),applyChange:B(async()=>({ok:!0,mode:`direct`}))},Ne={title:`setup/connections/ConnectionDetail`,component:De,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`shared-es`},sources:{...T(),errors:[]}},decorators:[e=>(0,z.jsx)(y.Provider,{value:Me,children:(0,z.jsx)(e,{})})]},U={name:`Created by hand`},W={name:`Managed by an Elasticsearch cluster`,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`orders-es`}}},G={name:`Written by a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`orders-backup`}}},K={name:`Kept from a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`old-backup`}}},q={name:`Management Identity of a plane`,args:{target:{kind:`ManagementAuthConfig`,name:`plane-a-optimize`}}},J={name:`Database server with default keys`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`}}},Y={name:`Not ready`,args:{target:{kind:`DatabaseConfig`,namespace:`team-a`,name:`orders-db`},sources:{...T(),databaseConfigs:T().databaseConfigs.map(e=>e.metadata.name===`orders-db`?{...e,status:{conditions:[{type:`Ready`,status:`False`,reason:`MissingSecret`,message:`Secret "orders-db" has no key "pass"`,lastTransitionTime:`2026-11-10T09:00:00Z`}]}}:e),errors:[]}}},Pe={...T(),databaseConfigs:[],errors:[]},X={name:`Editing a contract created by hand`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},mode:`direct`,actions:H}},Z={name:`Review of a save`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},mode:`direct`,actions:H},play:async({canvasElement:e})=>{let t=je(e),n=t.getByLabelText(`Host`);await V.clear(n),await V.type(n,`pg-2.team-a`),await V.click(t.getByRole(`button`,{name:`Review`})),await Ae(await t.findByRole(`region`,{name:`Review the contract`})).toBeVisible()}},Q={name:`Delete of an unused contract`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},sources:Pe,mode:`direct`,actions:H}},$={name:`Delete refused for a used contract`,args:{target:{kind:`DatabaseConfig`,namespace:`team-a`,name:`orders-db`},sources:{...T(),secondaryStorageConfigs:[...T().secondaryStorageConfigs,{apiVersion:`core.camunda.io/v1`,kind:`SecondaryStorageConfig`,metadata:{namespace:`team-a`,name:`orders-rdbms`},spec:{type:`rdbms`,rdbms:{databaseConfigRef:`orders-db`}}}],errors:[]},mode:`direct`,actions:H}},Fe=[`CreatedByHand`,`ManagedByAnElasticsearchCluster`,`WrittenByARegistration`,`KeptFromARegistration`,`ManagementIdentityOfAPlane`,`DatabaseServerWithDefaultKeys`,`NotReady`,`EditingAContractCreatedByHand`,`ReviewOfASave`,`DeleteOfAnUnusedContract`,`DeleteRefusedForAUsedContract`],U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
  name: "Created by hand"
}`,...U.parameters?.docs?.source}}},W.parameters={...W.parameters,docs:{...W.parameters?.docs,source:{originalSource:`{
  name: "Managed by an Elasticsearch cluster",
  args: {
    target: {
      kind: "SecondaryStorageConfig",
      namespace: "team-a",
      name: "orders-es"
    }
  }
}`,...W.parameters?.docs?.source}}},G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
  name: "Written by a registration",
  args: {
    target: {
      kind: "ObjectStorageConfig",
      namespace: "team-a",
      name: "orders-backup"
    }
  }
}`,...G.parameters?.docs?.source}}},K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
  name: "Kept from a registration",
  args: {
    target: {
      kind: "ObjectStorageConfig",
      namespace: "team-a",
      name: "old-backup"
    }
  }
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  name: "Management Identity of a plane",
  args: {
    target: {
      kind: "ManagementAuthConfig",
      name: "plane-a-optimize"
    }
  }
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "Database server with default keys",
  args: {
    target: {
      kind: "DatabaseServerConfig",
      namespace: "team-a",
      name: "postgres"
    }
  }
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
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
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "Editing a contract created by hand",
  args: {
    target: {
      kind: "DatabaseServerConfig",
      namespace: "team-a",
      name: "postgres"
    },
    mode: "direct",
    actions
  }
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "Review of a save",
  args: {
    target: {
      kind: "DatabaseServerConfig",
      namespace: "team-a",
      name: "postgres"
    },
    mode: "direct",
    actions
  },
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    const host = canvas.getByLabelText("Host");
    await userEvent.clear(host);
    await userEvent.type(host, "pg-2.team-a");
    await userEvent.click(canvas.getByRole("button", {
      name: "Review"
    }));
    await expect(await canvas.findByRole("region", {
      name: "Review the contract"
    })).toBeVisible();
  }
}`,...Z.parameters?.docs?.source}}},Q.parameters={...Q.parameters,docs:{...Q.parameters?.docs,source:{originalSource:`{
  name: "Delete of an unused contract",
  args: {
    target: {
      kind: "DatabaseServerConfig",
      namespace: "team-a",
      name: "postgres"
    },
    sources: unusedServer,
    mode: "direct",
    actions
  }
}`,...Q.parameters?.docs?.source}}},$.parameters={...$.parameters,docs:{...$.parameters?.docs,source:{originalSource:`{
  name: "Delete refused for a used contract",
  args: {
    target: {
      kind: "DatabaseConfig",
      namespace: "team-a",
      name: "orders-db"
    },
    sources: {
      ...connectionFixtures(),
      secondaryStorageConfigs: [...connectionFixtures().secondaryStorageConfigs, {
        apiVersion: "core.camunda.io/v1",
        kind: "SecondaryStorageConfig",
        metadata: {
          namespace: "team-a",
          name: "orders-rdbms"
        },
        spec: {
          type: "rdbms",
          rdbms: {
            databaseConfigRef: "orders-db"
          }
        }
      }],
      errors: []
    },
    mode: "direct",
    actions
  }
}`,...$.parameters?.docs?.source}}}})))()}Ie();export{U as CreatedByHand,J as DatabaseServerWithDefaultKeys,Q as DeleteOfAnUnusedContract,$ as DeleteRefusedForAUsedContract,X as EditingAContractCreatedByHand,K as KeptFromARegistration,W as ManagedByAnElasticsearchCluster,q as ManagementIdentityOfAPlane,Y as NotReady,Z as ReviewOfASave,G as WrittenByARegistration,Fe as __namedExportsOrder,Ne as default};