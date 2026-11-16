import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{r,t as ee}from"./typed-confirm-TlWoxrKe.js";import{n as i,r as te,t as ne}from"./toast-D2txxnvK.js";import{a,i as re,n as o,r as s}from"./claim-tab-C2GpuiP8.js";import{n as c,t as ie}from"./status-chip-DZyD1nka.js";import{n as l,t as ae}from"./change-bar-DuDbqTD1.js";import{n as u}from"./cluster-edit-BFatbX76.js";import{_ as oe,b as d,g as se,h as ce,t as le,x as ue,y as de}from"./usage-CG6g1G0O.js";import{n as f,t as fe}from"./change-review-eps4CC76.js";import{i as pe,n as me,r as p}from"./namespaced-secret-ref-picker-D2dxBRSO.js";import{o as m,t as he}from"./form-parts-B9x9KUZu.js";import{a as ge,o as h,r as g}from"./rows-BU6YkGii.js";import{n as _e,t as _}from"./use-setup-editor-BsWgkfve.js";import{a as ve,c as ye,n as v,o as y,t as b}from"./fixtures-Cz7ehlGJ.js";import{a as be,c as x,i as xe,l as S,o as Se,r as C,s as w}from"./connection-list-DkM4LRby.js";import{a as T,c as E,i as Ce,n as we,o as Te,r as Ee,s as De,t as Oe}from"./draft-BNnPh8me.js";import{a as D,r as O}from"./model-F1l5dw2z.js";import{n as k,t as ke}from"./users-table-Bg6bgTkv.js";function Ae(e,t,n){let r=t.map(t=>`${t.kind} ${t.namespace}/${t.name} ${A[e][t.kind]??`reads the changed contract.`}`);return n.length>0?[...r,`${D(n)} ${j[e]}`]:r.length>0?r:[`No resource uses this contract.`]}var A,j;function M(){return(M=e((()=>{O(),A={ObjectStorageConfig:{CamundaCluster:`restarts its pods, all but Connectors.`,CamundaClusterClaim:`passes the change to its cluster, which restarts its pods.`,ElasticsearchCluster:`registers its snapshot repository again. A change of the workload identity restarts its pods.`,DatabaseServer:`applies its archive again. A new bucket, base path, region, or endpoint moves the archive: a backup from before the move no longer counts for a rollback.`,LogicalBackupElasticsearch:`fails while it runs when the bucket, base path, region, or endpoint changes. Its delete then cannot reach the old location.`,LogicalBackupRDBMS:`fails while it runs when the bucket, base path, region, or endpoint changes. Its delete then leaves the dump in the old location.`,PluginRequest:`passes the change to the cluster of its claim, which restarts its pods.`},SecondaryStorageConfig:{CamundaCluster:`restarts all its pods and the pods of its Optimize. A new endpoint, host, port, or database is a new storage backend for the cluster.`,CamundaClusterClaim:`passes the change to its cluster, which restarts its pods.`,LogicalBackupElasticsearch:`fails while it runs when the endpoint changes.`,PluginRequest:`passes the change to the cluster of its claim, which restarts its pods.`},DatabaseServerConfig:{Database:`waits until the operator checks the server again. On a new server it creates its database and roles there. The database on the old server stays.`,DatabaseConfig:`checks the server again. A cluster or management plane that uses this database config restarts its pods.`,DatabaseServer:`reads the changed contract for the rollback that runs.`,PointInTimeRestore:`fails when the host or the port changes before its recovery step.`},DatabaseConfig:{SecondaryStorageConfig:`checks the database config again. A cluster that uses this secondary storage restarts its pods. A new database name is a new storage backend for that cluster.`,CamundaManagementCluster:`restarts its pods: Identity, Console, and Web Modeler for the Keycloak or the Identity database, the Web Modeler REST API for the Web Modeler database. Keycloak gets the new database settings.`,CamundaManagementClusterClaim:`passes the change to its management plane, which restarts its pods.`,PluginRequest:`passes the change to the management plane of its claim, which restarts its pods.`},ManagementAuthConfig:{CamundaOptimize:`restarts its pods.`,CamundaClusterClaim:`passes the change to the Optimize of its cluster, which restarts its pods.`}},j={ObjectStorageConfig:`A cluster that uses this contract restarts its pods.`,SecondaryStorageConfig:`A cluster that uses this contract restarts its pods.`,DatabaseServerConfig:`A database moves to a changed server, and a cluster or management plane that uses it restarts its pods.`,DatabaseConfig:`A cluster or management plane that uses this contract restarts its pods.`,ManagementAuthConfig:`An Optimize that uses this contract restarts its pods.`}})))()}function N({secret:e}){return(0,P.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-1`,children:[(0,P.jsx)(`span`,{className:`font-mono text-sm [overflow-wrap:anywhere]`,children:e.namespace?`${e.namespace}/${e.name}`:e.name}),(0,P.jsx)(`dl`,{className:`grid grid-cols-[max-content_1fr] gap-x-3 gap-y-0.5 text-xs`,children:e.keys.map(([e,t])=>(0,P.jsxs)(`div`,{className:`contents`,children:[(0,P.jsx)(`dt`,{className:`text-muted`,children:e}),(0,P.jsx)(`dd`,{className:`font-mono text-text-2 [overflow-wrap:anywhere]`,children:t})]},e))})]})}var P;function F(){return(F=e((()=>{P=n(),N.__docgenInfo={description:`SecretRefView shows one Secret reference: the Secret, with its namespace
when it has one, and each key that the contract reads. It shows names
and keys only, never a value.`,methods:[],displayName:`SecretRefView`,props:{secret:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  namespace?: string;
  name: string;
  keys: readonly (readonly [label: string, key: string])[];
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!1}},{key:`name`,value:{name:`string`,required:!0}},{key:`keys`,value:{name:`unknown`,required:!0}}]}},description:``}}}})))()}function I({target:e,sources:t,mode:n,actions:r}){let i=w(t,e.kind),a=i.find(t=>t.metadata.name===e.name&&(e.namespace===void 0||t.metadata.namespace===e.namespace)),c=be.find(t=>t.kind===e.kind),l=_e({live:a,isNew:!1,listed:i.filter(t=>t.metadata.namespace===e.namespace).map(e=>e.metadata.name??``),draftOf:Ce,listHref:`/setup/connections?tab=${c.value}`,hrefOf:(t,n)=>Se({kind:e.kind,namespace:n,name:t})}),[u,le]=(0,je.useState)(!1),{toast:f,show:pe,dismiss:me}=te(),p=de(e.kind,h(t)),m=se(t).get(ce(e))??[],{base:g,draft:_,reviewing:v,locked:y}=l,b=a??g,x=b&&oe(b,xe(t),t.planes),S=r!==void 0&&x!==void 0&&d(x),C=v?.kind===`delete`;v&&!u&&(!S||C&&(m.length>0||p.length>0))&&l.cancel();let T=t.errors.map(e=>(0,L.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind));if(!b)return(0,L.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[T,!h(t).includes(e.kind)&&(0,L.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`The contract `,e.name,` does not exist.`]})]});let E=ye(b),D=Ee(e.kind,_),O=g?Oe(g,_):0,k=n=>h(t).includes(n)?void 0:w(t,n).filter(t=>t.metadata.namespace===e.namespace).map(e=>e.metadata.name??``);return(0,L.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,L.jsxs)(`header`,{className:`flex flex-col gap-2`,children:[(0,L.jsx)(`p`,{className:`text-sm text-muted`,children:c.label}),(0,L.jsxs)(`div`,{className:`flex flex-wrap items-center gap-3`,children:[(0,L.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em] [overflow-wrap:anywhere]`,children:e.name}),(0,L.jsx)(ie,{condition:E})]}),e.namespace!==void 0&&(0,L.jsx)(`p`,{className:`font-mono text-sm text-muted`,children:e.namespace})]}),S&&!n&&(0,L.jsx)(`p`,{role:`alert`,className:`text-sm text-bad`,children:`Changes are off: the fleet manager could not read its settings.`}),T,!a&&(0,L.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`The contract `,e.name,` was deleted.`]}),(0,L.jsxs)(`div`,{className:`grid gap-4 lg:grid-cols-2`,children:[(0,L.jsx)(o,{label:`Status`,children:E?(0,L.jsxs)(re,{children:[(0,L.jsx)(s,{label:`Reason`,children:E.reason||`None`}),(0,L.jsx)(s,{label:`Message`,children:E.message||`None`})]}):(0,L.jsx)(`p`,{className:`text-sm text-muted`,children:`No status yet`})}),(0,L.jsx)(o,{label:`Writer`,children:(0,L.jsxs)(`div`,{className:`flex flex-col gap-1 text-sm`,children:[x&&(0,L.jsx)(`p`,{children:ue(x)}),x&&!d(x)&&(0,L.jsxs)(`p`,{className:`text-muted`,children:[`Managed by `,ue(x),`. Change it there.`]})]})})]}),S?(0,L.jsx)(`fieldset`,{disabled:y||!a||!n,className:`flex min-w-0 flex-col gap-4`,children:(0,L.jsx)(he,{title:`Settings`,children:(0,L.jsx)(De,{kind:e.kind,namespace:e.namespace,draft:_,errors:D,namespaces:t.namespaces.map(e=>e.metadata?.name??``),databaseConfigs:k(`DatabaseConfig`),databaseServerConfigs:k(`DatabaseServerConfig`),onChange:e=>l.setDraft(()=>e)})})},l.generation):(0,L.jsx)(o,{label:`Settings`,children:(0,L.jsx)(re,{children:ve(b).map(e=>(0,L.jsx)(s,{label:e.label,note:e.note,children:e.secret?(0,L.jsx)(N,{secret:e.secret}):e.value},e.label))})}),(0,L.jsx)(o,{label:`Used by`,children:(0,L.jsx)(ke,{users:m,unread:p})}),S&&a&&(0,L.jsx)(`fieldset`,{disabled:y||!n,className:`flex min-w-0 flex-col gap-4`,children:(0,L.jsx)(he,{title:`Delete the contract`,children:p.length>0?(0,L.jsx)(`p`,{className:`text-sm text-muted`,children:ge(p)}):m.length>0?(0,L.jsx)(`p`,{className:`text-sm text-muted`,children:`Delete is possible when no resource uses this contract.`}):(0,L.jsxs)(ee,{name:e.name,confirmLabel:`Review the delete`,onConfirm:()=>l.review({change:we(a),kind:`delete`}),children:[`The contract `,e.name,` is deleted.`]})})},l.generation),S&&v&&n?(0,L.jsx)(`section`,{"aria-label":C?`Review the delete`:`Review the contract`,className:`rounded-md border border-border bg-panel p-5`,children:(0,L.jsx)(fe,{change:v.change,mode:n,diffLabel:`Changes to the contract`,effects:C?[`The contract ${e.name} is deleted.`,...n===`git`?[`In Git mode the delete is a commit that removes the file of the contract. GitOps must prune the ${e.kind} to delete it from the cluster.`]:[]]:Ae(e.kind,m,p),confirm:C?{label:`Delete the contract`,gitLabel:`Push the delete to Git`,danger:!0}:{label:`Save`},savedMessage:C?`The contract ${e.name} is deleted.`:`The contract ${e.name} is saved.`,preview:r.previewChange,apply:r.applyChange,show:(t,n)=>{pe(t,n),l.written(t,e.name,e.namespace)},onCancel:l.cancel,onWriting:le})}):S&&O>0&&!y&&g&&(0,L.jsx)(ae,{count:O,blocked:D[0]?.message,onDiscard:l.discard,onReview:()=>l.review({change:Te(g,_),kind:`save`})}),f&&(0,L.jsx)(`div`,{className:`fixed right-4 bottom-4 z-20`,children:(0,L.jsx)(ne,{result:f,onDismiss:me})})]})}var L,je;function R(){return(R=e((()=>{L=n(),je=t(),a(),u(),l(),f(),c(),i(),r(),m(),g(),_(),le(),C(),E(),T(),y(),M(),F(),S(),x(),k(),I.__docgenInfo={description:`ConnectionDetail is the page of one contract: its own Ready, its writer,
its settings, and the objects that use it. A contract that sources does
not hold shows as missing, unless its kind could not be read. Each of the
read errors shows above.

With actions, a contract that a person may change (writerEditable) has a
form with the form state of useSetupEditor, and a delete. A save and a
delete each show a review with a dry run before the write. The delete is
offered only while nothing uses the contract and each kind that can use
it is read; an open delete review closes when that stops being true, and
an open review closes when the contract gets a writer. Without mode the
form is disabled. Any other contract keeps the read view.`,methods:[],displayName:`ConnectionDetail`,props:{target:{required:!0,tsType:{name:`ContractRef`},description:``},sources:{required:!0,tsType:{name:`intersection`,raw:`ConnectionSources & { errors: ReadError[] } & SourceReads`,elements:[{name:`intersection`,raw:`ContractUserSources & {
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
}`,signature:{properties:[{key:`previewChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<Preview>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`Preview`}],raw:`Promise<Preview>`}},required:!0}},{key:`applyChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}},required:!0}}]}},description:``}}}})))()}var z,Me,B,V,Ne,Pe,H,Fe,U,W,G,K,q,J,Y,Ie,X,Z,Q,$,Le;function Re(){return(Re=e((()=>{z=n(),p(),R(),v(),{expect:Me,fn:B,userEvent:V,within:Ne}=__STORYBOOK_MODULE_TEST__,Pe=pe({"team-a":[{name:`postgres-admin`,keys:[`username`,`password`]},{name:`orders-db`,keys:[`user`,`pass`]}]}),H={previewChange:B(async e=>({objects:[...e.apply,...e.delete??[]].map(e=>{let t=`metadata`in e?{...e.metadata,apiVersion:e.apiVersion,kind:e.kind}:e;return{ref:{apiVersion:t.apiVersion,kind:t.kind,namespace:t.namespace,name:t.name},diff:[{op:`added`,text:`host: pg-2.team-a`}]}}),errors:[]})),applyChange:B(async()=>({ok:!0,mode:`direct`}))},Fe={title:`setup/connections/ConnectionDetail`,component:I,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`shared-es`},sources:{...b(),errors:[]}},decorators:[e=>(0,z.jsx)(me.Provider,{value:Pe,children:(0,z.jsx)(e,{})})]},U={name:`Created by hand`},W={name:`Managed by an Elasticsearch cluster`,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`orders-es`}}},G={name:`Written by a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`orders-backup`}}},K={name:`Kept from a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`old-backup`}}},q={name:`Management Identity of a plane`,args:{target:{kind:`ManagementAuthConfig`,name:`plane-a-optimize`}}},J={name:`Database server with default keys`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`}}},Y={name:`Not ready`,args:{target:{kind:`DatabaseConfig`,namespace:`team-a`,name:`orders-db`},sources:{...b(),databaseConfigs:b().databaseConfigs.map(e=>e.metadata.name===`orders-db`?{...e,status:{conditions:[{type:`Ready`,status:`False`,reason:`MissingSecret`,message:`Secret "orders-db" has no key "pass"`,lastTransitionTime:`2026-11-10T09:00:00Z`}]}}:e),errors:[]}}},Ie={...b(),databaseConfigs:[],errors:[]},X={name:`Editing a contract created by hand`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},mode:`direct`,actions:H}},Z={name:`Review of a save`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},mode:`direct`,actions:H},play:async({canvasElement:e})=>{let t=Ne(e),n=t.getByLabelText(`Host`);await V.clear(n),await V.type(n,`pg-2.team-a`),await V.click(t.getByRole(`button`,{name:`Review`})),await Me(await t.findByRole(`region`,{name:`Review the contract`})).toBeVisible()}},Q={name:`Delete of an unused contract`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},sources:Ie,mode:`direct`,actions:H}},$={name:`Delete refused for a used contract`,args:{target:{kind:`DatabaseConfig`,namespace:`team-a`,name:`orders-db`},sources:{...b(),secondaryStorageConfigs:[...b().secondaryStorageConfigs,{apiVersion:`core.camunda.io/v1`,kind:`SecondaryStorageConfig`,metadata:{namespace:`team-a`,name:`orders-rdbms`},spec:{type:`rdbms`,rdbms:{databaseConfigRef:`orders-db`}}}],errors:[]},mode:`direct`,actions:H}},Le=[`CreatedByHand`,`ManagedByAnElasticsearchCluster`,`WrittenByARegistration`,`KeptFromARegistration`,`ManagementIdentityOfAPlane`,`DatabaseServerWithDefaultKeys`,`NotReady`,`EditingAContractCreatedByHand`,`ReviewOfASave`,`DeleteOfAnUnusedContract`,`DeleteRefusedForAUsedContract`],U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
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
}`,...$.parameters?.docs?.source}}}})))()}Re();export{U as CreatedByHand,J as DatabaseServerWithDefaultKeys,Q as DeleteOfAnUnusedContract,$ as DeleteRefusedForAUsedContract,X as EditingAContractCreatedByHand,K as KeptFromARegistration,W as ManagedByAnElasticsearchCluster,q as ManagementIdentityOfAPlane,Y as NotReady,Z as ReviewOfASave,G as WrittenByARegistration,Le as __namedExportsOrder,Fe as default};