import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{j as ee,t as r}from"./policy-DFk5vpS8.js";import{r as i,t as te}from"./typed-confirm-BqKaSbCR.js";import{a,i as o,n as s,r as c}from"./claim-tab-BPTPcCWG.js";import{n as l,r as ne,t as re}from"./toast-DwtDwj8X.js";import{n as u,t as ie}from"./status-chip-CIY7L-46.js";import{n as ae,t as oe}from"./change-review-1L65ksHP.js";import{_ as se,b as d,g as ce,h as le,t as f,x as ue,y as de}from"./usage-XicndySu.js";import{a as fe,o as p,r as m}from"./rows-DuAU-Tbp.js";import{n as pe,t as me}from"./change-bar-Daxvf2RZ.js";import"./claim-edit-R3Qldxo3.js";import{n as he}from"./cluster-edit-Dldp8a3_.js";import{i as h,n as g,r as _}from"./namespaced-secret-ref-picker-IG4isFGr.js";import{o as v,t as ge}from"./form-parts-C9UWBnjX.js";import{n as _e,t as y}from"./use-setup-editor-CGmiVEU3.js";import{a as ve,n as b,o as x,t as S}from"./fixtures-PmE5ozkW.js";import{a as ye,c as C,i as be,l as w,o as xe,r as T,s as Se}from"./connection-list-DJgVyRfY.js";import{a as E,c as D,i as Ce,n as we,o as Te,r as Ee,s as De,t as Oe}from"./draft-Bq_FG_pp.js";import{n as O,t as ke}from"./users-table-Gn3DHNlh.js";function Ae(e,t){let n=t.map(t=>`${t.kind} ${t.namespace}/${t.name} ${k[e][t.kind]??`reads the changed contract.`}`);return n.length>0?n:[`No resource uses this contract.`]}var k;function A(){return(A=e((()=>{k={ObjectStorageConfig:{CamundaCluster:`restarts its pods, all but Connectors.`,CamundaClusterClaim:`passes the change to its cluster, which restarts its pods.`,ElasticsearchCluster:`registers its snapshot repository again. A change of the workload identity restarts its pods.`,DatabaseServer:`applies its archive again. A new bucket, base path, region, or endpoint moves the archive: a backup from before the move no longer counts for a rollback.`,LogicalBackupElasticsearch:`fails while it runs when the bucket, base path, region, or endpoint changes. Its delete then cannot reach the old location.`,LogicalBackupRDBMS:`fails while it runs when the bucket, base path, region, or endpoint changes. Its delete then leaves the dump in the old location.`,PluginRequest:`passes the change to the cluster of its claim, which restarts its pods.`},SecondaryStorageConfig:{CamundaCluster:`restarts all its pods and the pods of its Optimize. A new endpoint, host, port, or database is a new storage backend for the cluster.`,CamundaClusterClaim:`passes the change to its cluster, which restarts its pods.`,LogicalBackupElasticsearch:`fails while it runs when the endpoint changes.`,PluginRequest:`passes the change to the cluster of its claim, which restarts its pods.`},DatabaseServerConfig:{Database:`waits until the operator checks the server again. On a new server it creates its database and roles there. The database on the old server stays.`,DatabaseConfig:`checks the server again. A cluster or management plane that uses this database config restarts its pods.`,DatabaseServer:`reads the changed contract for the rollback that runs.`,PointInTimeRestore:`fails when the host or the port changes before its recovery step.`},DatabaseConfig:{SecondaryStorageConfig:`checks the database config again. A cluster that uses this secondary storage restarts its pods. A new database name is a new storage backend for that cluster.`,CamundaManagementCluster:`restarts its pods: Identity, Console, and Web Modeler for the Keycloak or the Identity database, the Web Modeler REST API for the Web Modeler database. Keycloak gets the new database settings.`,CamundaManagementClusterClaim:`passes the change to its management plane, which restarts its pods.`,PluginRequest:`passes the change to the management plane of its claim, which restarts its pods.`},ManagementAuthConfig:{CamundaOptimize:`restarts its pods.`,CamundaClusterClaim:`passes the change to the Optimize of its cluster, which restarts its pods.`}}})))()}function j({secret:e}){return(0,M.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-1`,children:[(0,M.jsx)(`span`,{className:`font-mono text-sm [overflow-wrap:anywhere]`,children:e.namespace?`${e.namespace}/${e.name}`:e.name}),(0,M.jsx)(`dl`,{className:`grid grid-cols-[max-content_1fr] gap-x-3 gap-y-0.5 text-xs`,children:e.keys.map(([e,t])=>(0,M.jsxs)(`div`,{className:`contents`,children:[(0,M.jsx)(`dt`,{className:`text-muted`,children:e}),(0,M.jsx)(`dd`,{className:`font-mono text-text-2 [overflow-wrap:anywhere]`,children:t})]},e))})]})}var M;function N(){return(N=e((()=>{M=n(),j.__docgenInfo={description:`SecretRefView shows one Secret reference: the Secret, with its namespace
when it has one, and each key that the contract reads. It shows names
and keys only, never a value.`,methods:[],displayName:`SecretRefView`,props:{secret:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  namespace?: string;
  name: string;
  keys: readonly (readonly [label: string, key: string])[];
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!1}},{key:`name`,value:{name:`string`,required:!0}},{key:`keys`,value:{name:`unknown`,required:!0}}]}},description:``}}}})))()}function P({target:e,sources:t,mode:n,actions:r}){let i=Se(t,e.kind),a=i.find(t=>t.metadata.name===e.name&&(e.namespace===void 0||t.metadata.namespace===e.namespace)),l=ye.find(t=>t.kind===e.kind),u=_e({live:a,isNew:!1,listed:i.filter(t=>t.metadata.namespace===e.namespace).map(e=>e.metadata.name??``),draftOf:Ce,listHref:`/setup/connections?tab=${l.value}`,hrefOf:(t,n)=>xe({kind:e.kind,namespace:n,name:t})}),[ae,f]=(0,I.useState)(!1),{toast:m,show:pe,dismiss:he}=ne(),h=de(e.kind,p(t)),g=ce(t).get(le(e))??[],{base:_,draft:v,reviewing:y,locked:b}=u,x=a??_,S=x&&se(x,be(t),t.planes),C=r!==void 0&&S!==void 0&&d(S),w=y?.kind===`delete`;y&&!ae&&(!C||h.length>0||w&&g.length>0)&&u.cancel();let T=t.errors.map(e=>(0,F.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind));if(!x)return(0,F.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[T,!p(t).includes(e.kind)&&(0,F.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`The contract `,e.name,` does not exist.`]})]});let E=ee(x.status?.conditions),D=Ee(e.kind,v),O=h.length>0?`A change is possible when the fleet manager can read ${h.join(`, `)}.`:void 0,k=_?Oe(_,v):0,A=n=>p(t).includes(n)?void 0:Se(t,n).filter(t=>t.metadata.namespace===e.namespace).map(e=>e.metadata.name??``);return(0,F.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,F.jsxs)(`header`,{className:`flex flex-col gap-2`,children:[(0,F.jsx)(`p`,{className:`text-sm text-muted`,children:l.label}),(0,F.jsxs)(`div`,{className:`flex flex-wrap items-center gap-3`,children:[(0,F.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em] [overflow-wrap:anywhere]`,children:e.name}),(0,F.jsx)(ie,{condition:E})]}),e.namespace!==void 0&&(0,F.jsx)(`p`,{className:`font-mono text-sm text-muted`,children:e.namespace})]}),C&&!n&&(0,F.jsx)(`p`,{role:`alert`,className:`text-sm text-bad`,children:`Changes are off: the fleet manager could not read its settings.`}),T,!a&&(0,F.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`The contract `,e.name,` was deleted.`]}),(0,F.jsxs)(`div`,{className:`grid gap-4 lg:grid-cols-2`,children:[(0,F.jsx)(s,{label:`Status`,children:E?(0,F.jsxs)(o,{children:[(0,F.jsx)(c,{label:`Reason`,children:E.reason||`None`}),(0,F.jsx)(c,{label:`Message`,children:E.message||`None`})]}):(0,F.jsx)(`p`,{className:`text-sm text-muted`,children:`No status yet`})}),(0,F.jsx)(s,{label:`Writer`,children:(0,F.jsxs)(`div`,{className:`flex flex-col gap-1 text-sm`,children:[S&&(0,F.jsx)(`p`,{children:ue(S)}),S&&!d(S)&&(0,F.jsxs)(`p`,{className:`text-muted`,children:[`Managed by `,ue(S),`. Change it there.`]})]})})]}),C?(0,F.jsx)(`fieldset`,{disabled:b||!a||!n,className:`flex min-w-0 flex-col gap-4`,children:(0,F.jsxs)(ge,{title:`Settings`,children:[O&&(0,F.jsx)(`p`,{className:`text-sm text-muted`,children:O}),(0,F.jsx)(De,{kind:e.kind,namespace:e.namespace,draft:v,errors:D,namespaces:t.namespaces.map(e=>e.metadata?.name??``),databaseConfigs:A(`DatabaseConfig`),databaseServerConfigs:A(`DatabaseServerConfig`),onChange:e=>u.setDraft(()=>e)})]})},`settings-${u.generation}`):(0,F.jsx)(s,{label:`Settings`,children:(0,F.jsx)(o,{children:ve(x).map(e=>(0,F.jsx)(c,{label:e.label,note:e.note,children:e.secret?(0,F.jsx)(j,{secret:e.secret}):e.value},e.label))})}),(0,F.jsx)(s,{label:`Used by`,children:(0,F.jsx)(ke,{users:g,unread:h})}),C&&a&&(0,F.jsx)(`fieldset`,{disabled:b||!n,className:`flex min-w-0 flex-col gap-4`,children:(0,F.jsx)(ge,{title:`Delete the contract`,children:h.length>0?(0,F.jsx)(`p`,{className:`text-sm text-muted`,children:fe(h)}):g.length>0?(0,F.jsx)(`p`,{className:`text-sm text-muted`,children:`Delete is possible when no resource uses this contract.`}):(0,F.jsxs)(te,{name:e.name,confirmLabel:`Review the delete`,onConfirm:()=>u.review({change:we(a),kind:`delete`}),children:[`The contract `,e.name,` is deleted.`]})})},`delete-${u.generation}`),C&&y&&n?(0,F.jsx)(`section`,{"aria-label":w?`Review the delete`:`Review the contract`,className:`rounded-md border border-border bg-panel p-5`,children:(0,F.jsx)(oe,{change:y.change,mode:n,diffLabel:`Changes to the contract`,effects:w?[`The contract ${e.name} is deleted.`,...n===`git`?[`In Git mode the delete is a commit that removes the file of the contract. GitOps must prune the ${e.kind} to delete it from the cluster.`]:[]]:Ae(e.kind,g),confirm:w?{label:`Delete the contract`,gitLabel:`Push the delete to Git`,danger:!0}:{label:`Save`},savedMessage:w?`The contract ${e.name} is deleted.`:`The contract ${e.name} is saved.`,preview:r.previewChange,apply:r.applyChange,show:(t,n)=>{pe(t,n),u.written(t,e.name,e.namespace)},onCancel:u.cancel,onWriting:f})}):C&&k>0&&!b&&_&&(0,F.jsx)(me,{count:k,blocked:O??D[0]?.message,onDiscard:u.discard,onReview:()=>u.review({change:Te(_,v),kind:`save`})}),m&&(0,F.jsx)(`div`,{className:`fixed right-4 bottom-4 z-20`,children:(0,F.jsx)(re,{result:m,onDismiss:he})})]})}var F,I;function L(){return(L=e((()=>{F=n(),I=t(),a(),he(),pe(),ae(),u(),l(),i(),v(),m(),y(),f(),T(),D(),E(),x(),r(),A(),N(),w(),C(),O(),P.__docgenInfo={description:`ConnectionDetail is the page of one contract: its own Ready, its writer,
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
}`,signature:{properties:[{key:`previewChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<Preview>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`Preview`}],raw:`Promise<Preview>`}},required:!0}},{key:`applyChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}},required:!0}}]}},description:``}}}})))()}var R,z,B,V,je,Me,H,Ne,U,W,G,K,q,J,Y,Pe,X,Z,Q,$,Fe;function Ie(){return(Ie=e((()=>{R=n(),_(),L(),b(),{expect:z,fn:B,userEvent:V,within:je}=__STORYBOOK_MODULE_TEST__,Me=h({"team-a":[{name:`postgres-admin`,keys:[`username`,`password`]},{name:`orders-db`,keys:[`user`,`pass`]}]}),H={previewChange:B(async e=>({objects:[...e.apply,...e.delete??[]].map(e=>{let t=`metadata`in e?{...e.metadata,apiVersion:e.apiVersion,kind:e.kind}:e;return{ref:{apiVersion:t.apiVersion,kind:t.kind,namespace:t.namespace,name:t.name},diff:[{op:`added`,text:`host: pg-2.team-a`}]}}),errors:[]})),applyChange:B(async()=>({ok:!0,mode:`direct`}))},Ne={title:`setup/connections/ConnectionDetail`,component:P,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`shared-es`},sources:{...S(),errors:[]}},decorators:[e=>(0,R.jsx)(g.Provider,{value:Me,children:(0,R.jsx)(e,{})})]},U={name:`Created by hand`},W={name:`Managed by an Elasticsearch cluster`,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`orders-es`}}},G={name:`Written by a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`orders-backup`}}},K={name:`Kept from a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`old-backup`}}},q={name:`Management Identity of a plane`,args:{target:{kind:`ManagementAuthConfig`,name:`plane-a-optimize`}}},J={name:`Database server with default keys`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`}}},Y={name:`Not ready`,args:{target:{kind:`DatabaseConfig`,namespace:`team-a`,name:`orders-db`},sources:{...S(),databaseConfigs:S().databaseConfigs.map(e=>e.metadata.name===`orders-db`?{...e,status:{conditions:[{type:`Ready`,status:`False`,reason:`MissingSecret`,message:`Secret "orders-db" has no key "pass"`,lastTransitionTime:`2026-11-10T09:00:00Z`}]}}:e),errors:[]}}},Pe={...S(),databaseConfigs:[],errors:[]},X={name:`Editing a contract created by hand`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},mode:`direct`,actions:H}},Z={name:`Review of a save`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},mode:`direct`,actions:H},play:async({canvasElement:e})=>{let t=je(e),n=t.getByLabelText(`Host`);await V.clear(n),await V.type(n,`pg-2.team-a`),await V.click(t.getByRole(`button`,{name:`Review`})),await z(await t.findByRole(`region`,{name:`Review the contract`})).toBeVisible()}},Q={name:`Delete of an unused contract`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},sources:Pe,mode:`direct`,actions:H}},$={name:`Delete refused for a used contract`,args:{target:{kind:`DatabaseConfig`,namespace:`team-a`,name:`orders-db`},sources:{...S(),secondaryStorageConfigs:[...S().secondaryStorageConfigs,{apiVersion:`core.camunda.io/v1`,kind:`SecondaryStorageConfig`,metadata:{namespace:`team-a`,name:`orders-rdbms`},spec:{type:`rdbms`,rdbms:{databaseConfigRef:`orders-db`}}}],errors:[]},mode:`direct`,actions:H}},Fe=[`CreatedByHand`,`ManagedByAnElasticsearchCluster`,`WrittenByARegistration`,`KeptFromARegistration`,`ManagementIdentityOfAPlane`,`DatabaseServerWithDefaultKeys`,`NotReady`,`EditingAContractCreatedByHand`,`ReviewOfASave`,`DeleteOfAnUnusedContract`,`DeleteRefusedForAUsedContract`],U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
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