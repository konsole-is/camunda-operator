import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{M as ee,t as r}from"./policy-BoTKJohV.js";import{r as i,t as te}from"./typed-confirm-YOaGYXYf.js";import{a,i as o,n as s,o as c}from"./claim-tab-vAQ9Owdf.js";import{n as l,r as ne,t as re}from"./toast-CYBfqCl-.js";import{n as u,t as ie}from"./status-chip-CElKWPAQ.js";import{n as d,t as ae}from"./change-review-D2trjVIF.js";import{n as f,t as oe}from"./change-bar-rLO5w_tj.js";import"./claim-edit-DhJQlIgF.js";import{a as se,i as ce,o as p}from"./unread-Do34-4tt.js";import{n as m}from"./cluster-edit-DaBH1DtL.js";import{C as le,T as ue,b as de,t as fe,w as pe,x as me,y as he}from"./usage-CoKcnzeE.js";import{i as h,n as g,r as _}from"./namespaced-secret-ref-picker-D4AY_TB4.js";import{o as v,t as y}from"./form-parts-BMi3ELzq.js";import{n as ge,t as b}from"./use-setup-editor-0nTDz69e.js";import{a as _e,n as x,o as S,t as C}from"./fixtures-PmE5ozkW.js";import{a as ve,c as w,i as ye,l as T,o as be,r as E,s as D}from"./connection-list-DJi9YcJO.js";import{a as O,c as k,i as xe,n as Se,o as Ce,r as we,s as Te,t as Ee}from"./draft-BYfATQFO.js";import{n as A,t as De}from"./users-table-DAsNBdKl.js";function Oe(e,t){let n=t.map(t=>`${t.kind} ${t.namespace}/${t.name} ${j[e][t.kind]??`reads the changed contract.`}`);return n.length>0?n:[`No resource uses this contract.`]}var j;function M(){return(M=e((()=>{j={ObjectStorageConfig:{CamundaCluster:`restarts its pods, all but Connectors.`,CamundaClusterClaim:`passes the change to its cluster, which restarts its pods.`,ElasticsearchCluster:`registers its snapshot repository again. A change of the workload identity restarts its pods.`,DatabaseServer:`applies its archive again. A new bucket, base path, region, or endpoint moves the archive: a backup from before the move no longer counts for a rollback.`,LogicalBackupElasticsearch:`fails while it runs when the bucket, base path, region, or endpoint changes. Its delete then cannot reach the old location.`,LogicalBackupRDBMS:`fails while it runs when the bucket, base path, region, or endpoint changes. Its delete then leaves the dump in the old location.`,PluginRequest:`passes the change to the cluster of its claim, which restarts its pods.`},SecondaryStorageConfig:{CamundaCluster:`restarts all its pods and the pods of its Optimize. A new endpoint, host, port, or database is a new storage backend for the cluster.`,CamundaClusterClaim:`passes the change to its cluster, which restarts its pods.`,LogicalBackupElasticsearch:`fails while it runs when the endpoint changes.`,PluginRequest:`passes the change to the cluster of its claim, which restarts its pods.`},DatabaseServerConfig:{Database:`waits until the operator checks the server again. On a new server it creates its database and roles there. The database on the old server stays.`,DatabaseConfig:`checks the server again. A cluster or management plane that uses this database config restarts its pods.`,DatabaseServer:`reads the changed contract for the rollback that runs.`,PointInTimeRestore:`fails when the host or the port changes before its recovery step.`},DatabaseConfig:{SecondaryStorageConfig:`checks the database config again. A cluster that uses this secondary storage restarts its pods. A new database name is a new storage backend for that cluster.`,CamundaManagementCluster:`restarts its pods: Identity, Console, and Web Modeler for the Keycloak or the Identity database, the Web Modeler REST API for the Web Modeler database. Keycloak gets the new database settings.`,CamundaManagementClusterClaim:`passes the change to its management plane, which restarts its pods.`,PluginRequest:`passes the change to the management plane of its claim, which restarts its pods.`},ManagementAuthConfig:{CamundaOptimize:`restarts its pods.`,CamundaClusterClaim:`passes the change to the Optimize of its cluster, which restarts its pods.`}}})))()}function N({secret:e}){return(0,P.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-1`,children:[(0,P.jsx)(`span`,{className:`font-mono text-sm [overflow-wrap:anywhere]`,children:e.namespace?`${e.namespace}/${e.name}`:e.name}),(0,P.jsx)(`dl`,{className:`grid grid-cols-[max-content_1fr] gap-x-3 gap-y-0.5 text-xs`,children:e.keys.map(([e,t])=>(0,P.jsxs)(`div`,{className:`contents`,children:[(0,P.jsx)(`dt`,{className:`text-muted`,children:e}),(0,P.jsx)(`dd`,{className:`font-mono text-text-2 [overflow-wrap:anywhere]`,children:t})]},e))})]})}var P;function F(){return(F=e((()=>{P=n(),N.__docgenInfo={description:`SecretRefView shows one Secret reference: the Secret, with its namespace
when it has one, and each key that the contract reads. It shows names
and keys only, never a value.`,methods:[],displayName:`SecretRefView`,props:{secret:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  namespace?: string;
  name: string;
  keys: readonly (readonly [label: string, key: string])[];
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!1}},{key:`name`,value:{name:`string`,required:!0}},{key:`keys`,value:{name:`unknown`,required:!0}}]}},description:``}}}})))()}function I({target:e,sources:t,mode:n,actions:r}){let i=D(t,e.kind),c=i.find(t=>t.metadata.name===e.name&&(e.namespace===void 0||t.metadata.namespace===e.namespace)),l=ve.find(t=>t.kind===e.kind),u=ge({live:c,isNew:!1,listed:i.filter(t=>t.metadata.namespace===e.namespace).map(e=>e.metadata.name??``),draftOf:xe,listHref:`/setup/connections?tab=${l.value}`,hrefOf:(t,n)=>be({kind:e.kind,namespace:n,name:t})}),[d,f]=(0,ke.useState)(!1),{toast:m,show:fe,dismiss:h}=ne(),g=le(e.kind,p(t)),_=de(t).get(he(e))??[],{base:v,draft:b,reviewing:x,locked:S}=u,C=c??v,w=C&&me(C,ye(t),t.planes),T=r!==void 0&&w!==void 0&&pe(w),E=x?.kind===`delete`;x&&!d&&(!T||g.length>0||E&&_.length>0)&&u.cancel();let O=t.errors.map(e=>(0,L.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind));if(!C)return(0,L.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[O,!p(t).includes(e.kind)&&(0,L.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`The contract `,e.name,` does not exist.`]})]});let k=ee(C.status?.conditions),A=we(e.kind,b),j=g.length>0?ce(g):void 0,M=v?Ee(v,b):0,P=n=>p(t).includes(n)?void 0:D(t,n).filter(t=>t.metadata.namespace===e.namespace).map(e=>e.metadata.name??``);return(0,L.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,L.jsxs)(`header`,{className:`flex flex-col gap-2`,children:[(0,L.jsx)(`p`,{className:`text-sm text-muted`,children:l.label}),(0,L.jsxs)(`div`,{className:`flex flex-wrap items-center gap-3`,children:[(0,L.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em] [overflow-wrap:anywhere]`,children:e.name}),(0,L.jsx)(ie,{condition:k})]}),e.namespace!==void 0&&(0,L.jsx)(`p`,{className:`font-mono text-sm text-muted`,children:e.namespace})]}),T&&!n&&(0,L.jsx)(`p`,{role:`alert`,className:`text-sm text-bad`,children:`Changes are off: the fleet manager could not read its settings.`}),O,!c&&(0,L.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`The contract `,e.name,` was deleted.`]}),(0,L.jsxs)(`div`,{className:`grid gap-4 lg:grid-cols-2`,children:[(0,L.jsx)(s,{label:`Status`,children:k?(0,L.jsxs)(a,{children:[(0,L.jsx)(o,{label:`Reason`,children:k.reason||`None`}),(0,L.jsx)(o,{label:`Message`,children:k.message||`None`})]}):(0,L.jsx)(`p`,{className:`text-sm text-muted`,children:`No status yet`})}),(0,L.jsx)(s,{label:`Writer`,children:(0,L.jsxs)(`div`,{className:`flex flex-col gap-1 text-sm`,children:[w&&(0,L.jsx)(`p`,{children:ue(w)}),w&&!pe(w)&&(0,L.jsxs)(`p`,{className:`text-muted`,children:[`Managed by `,ue(w),`. Change it there.`]})]})})]}),T?(0,L.jsx)(`fieldset`,{disabled:S||!c||!n,className:`flex min-w-0 flex-col gap-4`,children:(0,L.jsxs)(y,{title:`Settings`,children:[j&&(0,L.jsx)(`p`,{className:`text-sm text-muted`,children:j}),(0,L.jsx)(Te,{kind:e.kind,namespace:e.namespace,draft:b,errors:A,namespaces:t.namespaces.map(e=>e.metadata?.name??``),databaseConfigs:P(`DatabaseConfig`),databaseServerConfigs:P(`DatabaseServerConfig`),onChange:e=>u.setDraft(()=>e)})]})},`settings-${u.generation}`):(0,L.jsx)(s,{label:`Settings`,children:(0,L.jsx)(a,{children:_e(C).map(e=>(0,L.jsx)(o,{label:e.label,note:e.note,children:e.secret?(0,L.jsx)(N,{secret:e.secret}):e.value},e.label))})}),(0,L.jsx)(s,{label:`Used by`,children:(0,L.jsx)(De,{users:_,unread:g})}),T&&c&&(0,L.jsx)(`fieldset`,{disabled:S||!n,className:`flex min-w-0 flex-col gap-4`,children:(0,L.jsx)(y,{title:`Delete the contract`,children:g.length>0?(0,L.jsx)(`p`,{className:`text-sm text-muted`,children:se(g)}):_.length>0?(0,L.jsx)(`p`,{className:`text-sm text-muted`,children:`Delete is possible when no resource uses this contract.`}):(0,L.jsxs)(te,{name:e.name,confirmLabel:`Review the delete`,onConfirm:()=>u.review({change:Se(c),kind:`delete`}),children:[`The contract `,e.name,` is deleted.`]})})},`delete-${u.generation}`),T&&x&&n?(0,L.jsx)(`section`,{"aria-label":E?`Review the delete`:`Review the contract`,className:`rounded-md border border-border bg-panel p-5`,children:(0,L.jsx)(ae,{change:x.change,mode:n,diffLabel:`Changes to the contract`,effects:E?[`The contract ${e.name} is deleted.`,...n===`git`?[`In Git mode the delete is a commit that removes the file of the contract. GitOps must prune the ${e.kind} to delete it from the cluster.`]:[]]:Oe(e.kind,_),confirm:E?{label:`Delete the contract`,gitLabel:`Push the delete to Git`,danger:!0}:{label:`Save`},savedMessage:E?`The contract ${e.name} is deleted.`:`The contract ${e.name} is saved.`,preview:r.previewChange,apply:r.applyChange,show:(t,n)=>{fe(t,n),u.written(t,e.name,e.namespace)},onCancel:u.cancel,onWriting:f})}):T&&M>0&&!S&&v&&(0,L.jsx)(oe,{count:M,blocked:j??A[0]?.message,onDiscard:u.discard,onReview:()=>u.review({change:Ce(v,b),kind:`save`})}),m&&(0,L.jsx)(`div`,{className:`fixed right-4 bottom-4 z-20`,children:(0,L.jsx)(re,{result:m,onDismiss:h})})]})}var L,ke;function Ae(){return(Ae=e((()=>{L=n(),ke=t(),c(),m(),f(),d(),u(),l(),i(),v(),b(),fe(),E(),k(),O(),S(),r(),M(),F(),T(),w(),A(),I.__docgenInfo={description:`ConnectionDetail is the page of one contract: its own Ready, its writer,
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
}`,signature:{properties:[{key:`previewChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<Preview>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`Preview`}],raw:`Promise<Preview>`}},required:!0}},{key:`applyChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}},required:!0}}]}},description:``}}}})))()}var R,je,z,B,Me,Ne,V,Pe,H,U,W,G,K,q,J,Fe,Y,X,Z,Q,Ie;function $(){return($=e((()=>{R=n(),_(),Ae(),x(),{expect:je,fn:z,userEvent:B,within:Me}=__STORYBOOK_MODULE_TEST__,Ne=h({"team-a":[{name:`postgres-admin`,keys:[`username`,`password`]},{name:`orders-db`,keys:[`user`,`pass`]}]}),V={previewChange:z(async e=>({objects:[...e.apply,...e.delete??[]].map(e=>{let t=`metadata`in e?{...e.metadata,apiVersion:e.apiVersion,kind:e.kind}:e;return{ref:{apiVersion:t.apiVersion,kind:t.kind,namespace:t.namespace,name:t.name},diff:[{op:`added`,text:`host: pg-2.team-a`}]}}),errors:[]})),applyChange:z(async()=>({ok:!0,mode:`direct`}))},Pe={title:`setup/connections/ConnectionDetail`,component:I,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`shared-es`},sources:{...C(),errors:[]}},decorators:[e=>(0,R.jsx)(g.Provider,{value:Ne,children:(0,R.jsx)(e,{})})]},H={name:`Created by hand`},U={name:`Managed by an Elasticsearch cluster`,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`orders-es`}}},W={name:`Written by a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`orders-backup`}}},G={name:`Kept from a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`old-backup`}}},K={name:`Management Identity of a plane`,args:{target:{kind:`ManagementAuthConfig`,name:`plane-a-optimize`}}},q={name:`Database server with default keys`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`}}},J={name:`Not ready`,args:{target:{kind:`DatabaseConfig`,namespace:`team-a`,name:`orders-db`},sources:{...C(),databaseConfigs:C().databaseConfigs.map(e=>e.metadata.name===`orders-db`?{...e,status:{conditions:[{type:`Ready`,status:`False`,reason:`MissingSecret`,message:`Secret "orders-db" has no key "pass"`,lastTransitionTime:`2026-11-10T09:00:00Z`}]}}:e),errors:[]}}},Fe={...C(),databaseConfigs:[],errors:[]},Y={name:`Editing a contract created by hand`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},mode:`direct`,actions:V}},X={name:`Review of a save`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},mode:`direct`,actions:V},play:async({canvasElement:e})=>{let t=Me(e),n=t.getByLabelText(`Host`);await B.clear(n),await B.type(n,`pg-2.team-a`),await B.click(t.getByRole(`button`,{name:`Review`})),await je(await t.findByRole(`region`,{name:`Review the contract`})).toBeVisible()}},Z={name:`Delete of an unused contract`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},sources:Fe,mode:`direct`,actions:V}},Q={name:`Delete refused for a used contract`,args:{target:{kind:`DatabaseConfig`,namespace:`team-a`,name:`orders-db`},sources:{...C(),secondaryStorageConfigs:[...C().secondaryStorageConfigs,{apiVersion:`core.camunda.io/v1`,kind:`SecondaryStorageConfig`,metadata:{namespace:`team-a`,name:`orders-rdbms`},spec:{type:`rdbms`,rdbms:{databaseConfigRef:`orders-db`}}}],errors:[]},mode:`direct`,actions:V}},Ie=[`CreatedByHand`,`ManagedByAnElasticsearchCluster`,`WrittenByARegistration`,`KeptFromARegistration`,`ManagementIdentityOfAPlane`,`DatabaseServerWithDefaultKeys`,`NotReady`,`EditingAContractCreatedByHand`,`ReviewOfASave`,`DeleteOfAnUnusedContract`,`DeleteRefusedForAUsedContract`],H.parameters={...H.parameters,docs:{...H.parameters?.docs,source:{originalSource:`{
  name: "Created by hand"
}`,...H.parameters?.docs?.source}}},U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
  name: "Managed by an Elasticsearch cluster",
  args: {
    target: {
      kind: "SecondaryStorageConfig",
      namespace: "team-a",
      name: "orders-es"
    }
  }
}`,...U.parameters?.docs?.source}}},W.parameters={...W.parameters,docs:{...W.parameters?.docs,source:{originalSource:`{
  name: "Written by a registration",
  args: {
    target: {
      kind: "ObjectStorageConfig",
      namespace: "team-a",
      name: "orders-backup"
    }
  }
}`,...W.parameters?.docs?.source}}},G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
  name: "Kept from a registration",
  args: {
    target: {
      kind: "ObjectStorageConfig",
      namespace: "team-a",
      name: "old-backup"
    }
  }
}`,...G.parameters?.docs?.source}}},K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
  name: "Management Identity of a plane",
  args: {
    target: {
      kind: "ManagementAuthConfig",
      name: "plane-a-optimize"
    }
  }
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  name: "Database server with default keys",
  args: {
    target: {
      kind: "DatabaseServerConfig",
      namespace: "team-a",
      name: "postgres"
    }
  }
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
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
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
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
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
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
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
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
}`,...Z.parameters?.docs?.source}}},Q.parameters={...Q.parameters,docs:{...Q.parameters?.docs,source:{originalSource:`{
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
}`,...Q.parameters?.docs?.source}}}})))()}$();export{H as CreatedByHand,q as DatabaseServerWithDefaultKeys,Z as DeleteOfAnUnusedContract,Q as DeleteRefusedForAUsedContract,Y as EditingAContractCreatedByHand,G as KeptFromARegistration,U as ManagedByAnElasticsearchCluster,K as ManagementIdentityOfAPlane,J as NotReady,X as ReviewOfASave,W as WrittenByARegistration,Ie as __namedExportsOrder,Pe as default};