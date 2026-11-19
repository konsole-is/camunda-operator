import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{N as ee,t as r}from"./policy-DPUkXbbW.js";import{o as i,t as a}from"./form-parts-9e442Qjn.js";import{r as o,t as te}from"./typed-confirm-UQsCjP0N.js";import{a as s,i as c,n as l,o as u}from"./claim-tab-DL0fMWe1.js";import{n as d,r as ne,t as re}from"./toast-DRLb2aFL.js";import{n as f,t as ie}from"./status-chip-Bw4cGxFw.js";import{n as p,t as ae}from"./change-review-DVX71JLZ.js";import{n as m,t as oe}from"./change-bar-BZpyuCVK.js";import"./claim-edit-orHFkWTp.js";import{a as se,i as ce,o as h}from"./unread-Do34-4tt.js";import{n as le}from"./cluster-edit-0ixoyx1w.js";import{C as ue,D as de,O as fe,T as pe,b as me,t as he,w as ge,x as _e,y as ve}from"./usage-BCfxeIht.js";import{i as g,n as _,r as v}from"./namespaced-secret-ref-picker-BuJAj5Ez.js";import{n as ye,t as y}from"./use-setup-editor-DIGX4pFd.js";import{a as be,n as b,o as x,t as S}from"./fixtures-kch9eRRI.js";import{a as xe,c as C,i as Se,l as w,o as Ce,r as T,s as E}from"./connection-list-B7RxLSU2.js";import{a as D,c as O,i as we,n as Te,o as Ee,r as De,s as Oe,t as ke}from"./draft-BxI2ucvh.js";import{n as k,t as Ae}from"./users-table-DFWzJ-l4.js";function je(e,t){let n=t.map(t=>`${t.kind} ${t.namespace}/${t.name} ${A[e][t.kind]??`reads the changed contract.`}`);return n.length>0?n:[`No resource uses this contract.`]}var A;function j(){return(j=e((()=>{A={ObjectStorageConfig:{CamundaCluster:`restarts its pods, all but Connectors.`,CamundaClusterClaim:`passes the change to its cluster, which restarts its pods.`,ElasticsearchCluster:`registers its snapshot repository again. A change of the workload identity restarts its pods.`,DatabaseServer:`applies its archive again. A new bucket, base path, region, or endpoint moves the archive: a backup from before the move no longer counts for a rollback.`,LogicalBackupElasticsearch:`fails while it runs when the bucket, base path, region, or endpoint changes. Its delete then cannot reach the old location.`,LogicalBackupRDBMS:`fails while it runs when the bucket, base path, region, or endpoint changes. Its delete then leaves the dump in the old location.`,PluginRequest:`passes the change to the cluster of its claim, which restarts its pods.`},SecondaryStorageConfig:{CamundaCluster:`restarts all its pods and the pods of its Optimize. A new endpoint, host, port, or database is a new storage backend for the cluster.`,CamundaClusterClaim:`passes the change to its cluster, which restarts its pods.`,LogicalBackupElasticsearch:`fails while it runs when the endpoint changes.`,PluginRequest:`passes the change to the cluster of its claim, which restarts its pods.`},DatabaseServerConfig:{Database:`waits until the operator checks the server again. On a new server it creates its database and roles there. The database on the old server stays.`,DatabaseConfig:`checks the server again. A cluster or management plane that uses this database config restarts its pods.`,DatabaseServer:`reads the changed contract for the rollback that runs.`,PointInTimeRestore:`fails when the host or the port changes before its recovery step.`},DatabaseConfig:{SecondaryStorageConfig:`checks the database config again. A cluster that uses this secondary storage restarts its pods. A new database name is a new storage backend for that cluster.`,CamundaManagementCluster:`restarts its pods: Identity, Console, and Web Modeler for the Keycloak or the Identity database, the Web Modeler REST API for the Web Modeler database. Keycloak gets the new database settings.`,CamundaManagementClusterClaim:`passes the change to its management plane, which restarts its pods.`,PluginRequest:`passes the change to the management plane of its claim, which restarts its pods.`},ManagementAuthConfig:{CamundaOptimize:`restarts its pods.`,CamundaClusterClaim:`passes the change to the Optimize of its cluster, which restarts its pods.`}}})))()}function M({secret:e}){return(0,N.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-1`,children:[(0,N.jsx)(`span`,{className:`font-mono text-sm [overflow-wrap:anywhere]`,children:e.namespace?`${e.namespace}/${e.name}`:e.name}),(0,N.jsx)(`dl`,{className:`grid grid-cols-[max-content_1fr] gap-x-3 gap-y-0.5 text-xs`,children:e.keys.map(([e,t])=>(0,N.jsxs)(`div`,{className:`contents`,children:[(0,N.jsx)(`dt`,{className:`text-muted`,children:e}),(0,N.jsx)(`dd`,{className:`font-mono text-text-2 [overflow-wrap:anywhere]`,children:t})]},e))})]})}var N;function P(){return(P=e((()=>{N=n(),M.__docgenInfo={description:`SecretRefView shows one Secret reference: the Secret, with its namespace
when it has one, and each key that the contract reads. It shows names
and keys only, never a value.`,methods:[],displayName:`SecretRefView`,props:{secret:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  namespace?: string;
  name: string;
  keys: readonly (readonly [label: string, key: string])[];
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!1}},{key:`name`,value:{name:`string`,required:!0}},{key:`keys`,value:{name:`unknown`,required:!0}}]}},description:``}}}})))()}function F({target:e,sources:t,mode:n,actions:r}){let i=E(t,e.kind),o=i.find(t=>t.metadata.name===e.name&&(e.namespace===void 0||t.metadata.namespace===e.namespace)),u=xe.find(t=>t.kind===e.kind),d=ye({live:o,isNew:!1,listed:i.filter(t=>t.metadata.namespace===e.namespace).map(e=>e.metadata.name??``),draftOf:we,listHref:`/setup/connections?tab=${u.value}`,hrefOf:(t,n)=>Ce({kind:e.kind,namespace:n,name:t})}),[f,p]=(0,L.useState)(!1),{toast:m,show:le,dismiss:he}=ne(),g=ue(e.kind,h(t)),_=pe(e,t),v=me(t).get(ve(e))??[],{base:y,draft:b,reviewing:x,locked:S}=d,C=o??y,w=C&&_e(C,Se(t),t.planes),T=r!==void 0&&w!==void 0&&de(w),D=x?.kind===`delete`;x&&!f&&(!T||g.length>0||D&&(v.length>0||_.length>0))&&d.cancel();let O=t.errors.map(e=>(0,I.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind));if(!C)return(0,I.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[O,!h(t).includes(e.kind)&&(0,I.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`The contract `,e.name,` does not exist.`]})]});let k=ee(C.status?.conditions),A=De(e.kind,b),j=g.length>0?ce(g):void 0,N=y?ke(y,b):0,P=n=>h(t).includes(n)?void 0:E(t,n).filter(t=>t.metadata.namespace===e.namespace).map(e=>e.metadata.name??``);return(0,I.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,I.jsxs)(`header`,{className:`flex flex-col gap-2`,children:[(0,I.jsx)(`p`,{className:`text-sm text-muted`,children:u.label}),(0,I.jsxs)(`div`,{className:`flex flex-wrap items-center gap-3`,children:[(0,I.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em] [overflow-wrap:anywhere]`,children:e.name}),(0,I.jsx)(ie,{condition:k})]}),e.namespace!==void 0&&(0,I.jsx)(`p`,{className:`font-mono text-sm text-muted`,children:e.namespace})]}),T&&!n&&(0,I.jsx)(`p`,{role:`alert`,className:`text-sm text-bad`,children:`Changes are off: the fleet manager could not read its settings.`}),O,!o&&(0,I.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`The contract `,e.name,` was deleted.`]}),(0,I.jsxs)(`div`,{className:`grid gap-4 lg:grid-cols-2`,children:[(0,I.jsx)(l,{label:`Status`,children:k?(0,I.jsxs)(s,{children:[(0,I.jsx)(c,{label:`Reason`,children:k.reason||`None`}),(0,I.jsx)(c,{label:`Message`,children:k.message||`None`})]}):(0,I.jsx)(`p`,{className:`text-sm text-muted`,children:`No status yet`})}),(0,I.jsx)(l,{label:`Writer`,children:(0,I.jsxs)(`div`,{className:`flex flex-col gap-1 text-sm`,children:[w&&(0,I.jsx)(`p`,{children:fe(w)}),w&&!de(w)&&(0,I.jsxs)(`p`,{className:`text-muted`,children:[`Managed by `,fe(w),`. Change it there.`]})]})})]}),T?(0,I.jsx)(`fieldset`,{disabled:S||!o||!n,className:`flex min-w-0 flex-col gap-4`,children:(0,I.jsxs)(a,{title:`Settings`,children:[j&&(0,I.jsx)(`p`,{className:`text-sm text-muted`,children:j}),(0,I.jsx)(Oe,{kind:e.kind,namespace:e.namespace,draft:b,errors:A,namespaces:t.namespaces.map(e=>e.metadata?.name??``),databaseConfigs:P(`DatabaseConfig`),databaseServerConfigs:P(`DatabaseServerConfig`),onChange:e=>d.setDraft(()=>e)})]})},`settings-${d.generation}`):(0,I.jsx)(l,{label:`Settings`,children:(0,I.jsx)(s,{children:be(C).map(e=>(0,I.jsx)(c,{label:e.label,note:e.note,children:e.secret?(0,I.jsx)(M,{secret:e.secret}):e.value},e.label))})}),(0,I.jsx)(l,{label:`Used by`,children:(0,I.jsx)(Ae,{users:v,unread:g,waiting:_})}),T&&o&&(0,I.jsx)(`fieldset`,{disabled:S||!n,className:`flex min-w-0 flex-col gap-4`,children:(0,I.jsx)(a,{title:`Delete the contract`,children:g.length>0?(0,I.jsx)(`p`,{className:`text-sm text-muted`,children:se(g)}):v.length>0?(0,I.jsx)(`p`,{className:`text-sm text-muted`,children:`Delete is possible when no resource uses this contract.`}):_.length>0?(0,I.jsx)(`p`,{className:`text-sm text-muted`,children:ge(_)}):(0,I.jsxs)(te,{name:e.name,confirmLabel:`Review the delete`,onConfirm:()=>d.review({change:Te(o),kind:`delete`}),children:[`The contract `,e.name,` is deleted.`]})})},`delete-${d.generation}`),T&&x&&n?(0,I.jsx)(`section`,{"aria-label":D?`Review the delete`:`Review the contract`,className:`rounded-md border border-border bg-panel p-5`,children:(0,I.jsx)(ae,{change:x.change,mode:n,diffLabel:`Changes to the contract`,effects:D?[`The contract ${e.name} is deleted.`,...n===`git`?[`In Git mode the delete is a commit that removes the file of the contract. GitOps must prune the ${e.kind} to delete it from the cluster.`]:[]]:je(e.kind,v),confirm:D?{label:`Delete the contract`,gitLabel:`Push the delete to Git`,danger:!0}:{label:`Save`},savedMessage:D?`The contract ${e.name} is deleted.`:`The contract ${e.name} is saved.`,preview:r.previewChange,apply:r.applyChange,show:(t,n)=>{le(t,n),d.written(t,e.name,e.namespace)},onCancel:d.cancel,onWriting:p})}):T&&N>0&&!S&&y&&(0,I.jsx)(oe,{count:N,blocked:j??A[0]?.message,onDiscard:d.discard,onReview:()=>d.review({change:Ee(y,b),kind:`save`})}),m&&(0,I.jsx)(`div`,{className:`fixed right-4 bottom-4 z-20`,children:(0,I.jsx)(re,{result:m,onDismiss:he})})]})}var I,L;function R(){return(R=e((()=>{I=n(),L=t(),u(),le(),m(),p(),f(),d(),o(),i(),y(),he(),T(),O(),D(),x(),r(),j(),P(),w(),C(),k(),F.__docgenInfo={description:`ConnectionDetail is the page of one contract: its own Ready, its writer,
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
}`,signature:{properties:[{key:`previewChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<Preview>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`Preview`}],raw:`Promise<Preview>`}},required:!0}},{key:`applyChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}},required:!0}}]}},description:``}}}})))()}var z,Me,B,V,Ne,Pe,H,Fe,U,W,G,K,q,J,Y,Ie,X,Z,Q,$,Le;function Re(){return(Re=e((()=>{z=n(),v(),R(),b(),{expect:Me,fn:B,userEvent:V,within:Ne}=__STORYBOOK_MODULE_TEST__,Pe=g({"team-a":[{name:`postgres-admin`,keys:[`username`,`password`]},{name:`orders-db`,keys:[`user`,`pass`]}]}),H={previewChange:B(async e=>({objects:[...e.apply,...e.delete??[]].map(e=>{let t=`metadata`in e?{...e.metadata,apiVersion:e.apiVersion,kind:e.kind}:e;return{ref:{apiVersion:t.apiVersion,kind:t.kind,namespace:t.namespace,name:t.name},diff:[{op:`added`,text:`host: pg-2.team-a`}]}}),errors:[]})),applyChange:B(async()=>({ok:!0,mode:`direct`}))},Fe={title:`setup/connections/ConnectionDetail`,component:F,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`shared-es`},sources:{...S(),errors:[]}},decorators:[e=>(0,z.jsx)(_.Provider,{value:Pe,children:(0,z.jsx)(e,{})})]},U={name:`Created by hand`},W={name:`Managed by an Elasticsearch cluster`,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`orders-es`}}},G={name:`Written by a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`orders-backup`}}},K={name:`Kept from a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`old-backup`}}},q={name:`Management Identity of a plane`,args:{target:{kind:`ManagementAuthConfig`,name:`plane-a-optimize`}}},J={name:`Database server with default keys`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`}}},Y={name:`Not ready`,args:{target:{kind:`DatabaseConfig`,namespace:`team-a`,name:`orders-db`},sources:{...S(),databaseConfigs:S().databaseConfigs.map(e=>e.metadata.name===`orders-db`?{...e,status:{conditions:[{type:`Ready`,status:`False`,reason:`MissingSecret`,message:`Secret "orders-db" has no key "pass"`,lastTransitionTime:`2026-11-10T09:00:00Z`}]}}:e),errors:[]}}},Ie={...S(),databaseConfigs:[],errors:[]},X={name:`Editing a contract created by hand`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},mode:`direct`,actions:H}},Z={name:`Review of a save`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},mode:`direct`,actions:H},play:async({canvasElement:e})=>{let t=Ne(e),n=t.getByLabelText(`Host`);await V.clear(n),await V.type(n,`pg-2.team-a`),await V.click(t.getByRole(`button`,{name:`Review`})),await Me(await t.findByRole(`region`,{name:`Review the contract`})).toBeVisible()}},Q={name:`Delete of an unused contract`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},sources:Ie,mode:`direct`,actions:H}},$={name:`Delete refused for a used contract`,args:{target:{kind:`DatabaseConfig`,namespace:`team-a`,name:`orders-db`},sources:{...S(),secondaryStorageConfigs:[...S().secondaryStorageConfigs,{apiVersion:`core.camunda.io/v1`,kind:`SecondaryStorageConfig`,metadata:{namespace:`team-a`,name:`orders-rdbms`},spec:{type:`rdbms`,rdbms:{databaseConfigRef:`orders-db`}}}],errors:[]},mode:`direct`,actions:H}},Le=[`CreatedByHand`,`ManagedByAnElasticsearchCluster`,`WrittenByARegistration`,`KeptFromARegistration`,`ManagementIdentityOfAPlane`,`DatabaseServerWithDefaultKeys`,`NotReady`,`EditingAContractCreatedByHand`,`ReviewOfASave`,`DeleteOfAnUnusedContract`,`DeleteRefusedForAUsedContract`],U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
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