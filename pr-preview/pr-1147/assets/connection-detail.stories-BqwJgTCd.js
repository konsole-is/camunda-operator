import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{r,t as ee}from"./typed-confirm-CJfiiwSq.js";import{n as i,r as te,t as ne}from"./toast-DahS-5oY.js";import{a,i as re,n as o,r as s}from"./claim-tab-oxolihSb.js";import{n as c,t as ie}from"./status-chip-flwC-OIy.js";import{n as l,t as ae}from"./change-review-CmbfOnJv.js";import{_ as oe,b as u,g as se,h as ce,t as d,x as f,y as le}from"./usage-CGQGB1z7.js";import{a as ue,o as p,r as de}from"./rows-COHgsQyc.js";import{n as m,t as fe}from"./change-bar-DaY2kKbE.js";import{n as pe}from"./cluster-edit-BQtkggmu.js";import{i as me,n as h,r as g}from"./namespaced-secret-ref-picker-Dq7T7XZu.js";import{o as _,t as he}from"./form-parts-BPcfyoRo.js";import{n as ge,t as v}from"./use-setup-editor-D573tPxU.js";import{a as _e,c as ve,n as y,o as b,t as x}from"./fixtures-Cz7ehlGJ.js";import{a as ye,c as S,i as be,l as C,o as xe,r as w,s as T}from"./connection-list-B8dHEuLM.js";import{a as E,c as D,i as Se,n as Ce,o as we,r as Te,s as Ee,t as De}from"./draft-CUjI6Hbl.js";import{n as O,t as Oe}from"./users-table-B9GNEXyu.js";function ke(e,t){let n=t.map(t=>`${t.kind} ${t.namespace}/${t.name} ${k[e][t.kind]??`reads the changed contract.`}`);return n.length>0?n:[`No resource uses this contract.`]}var k;function A(){return(A=e((()=>{k={ObjectStorageConfig:{CamundaCluster:`restarts its pods, all but Connectors.`,CamundaClusterClaim:`passes the change to its cluster, which restarts its pods.`,ElasticsearchCluster:`registers its snapshot repository again. A change of the workload identity restarts its pods.`,DatabaseServer:`applies its archive again. A new bucket, base path, region, or endpoint moves the archive: a backup from before the move no longer counts for a rollback.`,LogicalBackupElasticsearch:`fails while it runs when the bucket, base path, region, or endpoint changes. Its delete then cannot reach the old location.`,LogicalBackupRDBMS:`fails while it runs when the bucket, base path, region, or endpoint changes. Its delete then leaves the dump in the old location.`,PluginRequest:`passes the change to the cluster of its claim, which restarts its pods.`},SecondaryStorageConfig:{CamundaCluster:`restarts all its pods and the pods of its Optimize. A new endpoint, host, port, or database is a new storage backend for the cluster.`,CamundaClusterClaim:`passes the change to its cluster, which restarts its pods.`,LogicalBackupElasticsearch:`fails while it runs when the endpoint changes.`,PluginRequest:`passes the change to the cluster of its claim, which restarts its pods.`},DatabaseServerConfig:{Database:`waits until the operator checks the server again. On a new server it creates its database and roles there. The database on the old server stays.`,DatabaseConfig:`checks the server again. A cluster or management plane that uses this database config restarts its pods.`,DatabaseServer:`reads the changed contract for the rollback that runs.`,PointInTimeRestore:`fails when the host or the port changes before its recovery step.`},DatabaseConfig:{SecondaryStorageConfig:`checks the database config again. A cluster that uses this secondary storage restarts its pods. A new database name is a new storage backend for that cluster.`,CamundaManagementCluster:`restarts its pods: Identity, Console, and Web Modeler for the Keycloak or the Identity database, the Web Modeler REST API for the Web Modeler database. Keycloak gets the new database settings.`,CamundaManagementClusterClaim:`passes the change to its management plane, which restarts its pods.`,PluginRequest:`passes the change to the management plane of its claim, which restarts its pods.`},ManagementAuthConfig:{CamundaOptimize:`restarts its pods.`,CamundaClusterClaim:`passes the change to the Optimize of its cluster, which restarts its pods.`}}})))()}function j({secret:e}){return(0,M.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-1`,children:[(0,M.jsx)(`span`,{className:`font-mono text-sm [overflow-wrap:anywhere]`,children:e.namespace?`${e.namespace}/${e.name}`:e.name}),(0,M.jsx)(`dl`,{className:`grid grid-cols-[max-content_1fr] gap-x-3 gap-y-0.5 text-xs`,children:e.keys.map(([e,t])=>(0,M.jsxs)(`div`,{className:`contents`,children:[(0,M.jsx)(`dt`,{className:`text-muted`,children:e}),(0,M.jsx)(`dd`,{className:`font-mono text-text-2 [overflow-wrap:anywhere]`,children:t})]},e))})]})}var M;function N(){return(N=e((()=>{M=n(),j.__docgenInfo={description:`SecretRefView shows one Secret reference: the Secret, with its namespace
when it has one, and each key that the contract reads. It shows names
and keys only, never a value.`,methods:[],displayName:`SecretRefView`,props:{secret:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  namespace?: string;
  name: string;
  keys: readonly (readonly [label: string, key: string])[];
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!1}},{key:`name`,value:{name:`string`,required:!0}},{key:`keys`,value:{name:`unknown`,required:!0}}]}},description:``}}}})))()}function P({target:e,sources:t,mode:n,actions:r}){let i=T(t,e.kind),a=i.find(t=>t.metadata.name===e.name&&(e.namespace===void 0||t.metadata.namespace===e.namespace)),c=ye.find(t=>t.kind===e.kind),l=ge({live:a,isNew:!1,listed:i.filter(t=>t.metadata.namespace===e.namespace).map(e=>e.metadata.name??``),draftOf:Se,listHref:`/setup/connections?tab=${c.value}`,hrefOf:(t,n)=>xe({kind:e.kind,namespace:n,name:t})}),[d,de]=(0,I.useState)(!1),{toast:m,show:pe,dismiss:me}=te(),h=le(e.kind,p(t)),g=se(t).get(ce(e))??[],{base:_,draft:v,reviewing:y,locked:b}=l,x=a??_,S=x&&oe(x,be(t),t.planes),C=r!==void 0&&S!==void 0&&u(S),w=y?.kind===`delete`;y&&!d&&(!C||h.length>0||w&&g.length>0)&&l.cancel();let E=t.errors.map(e=>(0,F.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind));if(!x)return(0,F.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[E,!p(t).includes(e.kind)&&(0,F.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`The contract `,e.name,` does not exist.`]})]});let D=ve(x),O=Te(e.kind,v),k=h.length>0?`A change is possible when the fleet manager can read ${h.join(`, `)}.`:void 0,A=_?De(_,v):0,M=n=>p(t).includes(n)?void 0:T(t,n).filter(t=>t.metadata.namespace===e.namespace).map(e=>e.metadata.name??``);return(0,F.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,F.jsxs)(`header`,{className:`flex flex-col gap-2`,children:[(0,F.jsx)(`p`,{className:`text-sm text-muted`,children:c.label}),(0,F.jsxs)(`div`,{className:`flex flex-wrap items-center gap-3`,children:[(0,F.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em] [overflow-wrap:anywhere]`,children:e.name}),(0,F.jsx)(ie,{condition:D})]}),e.namespace!==void 0&&(0,F.jsx)(`p`,{className:`font-mono text-sm text-muted`,children:e.namespace})]}),C&&!n&&(0,F.jsx)(`p`,{role:`alert`,className:`text-sm text-bad`,children:`Changes are off: the fleet manager could not read its settings.`}),E,!a&&(0,F.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`The contract `,e.name,` was deleted.`]}),(0,F.jsxs)(`div`,{className:`grid gap-4 lg:grid-cols-2`,children:[(0,F.jsx)(o,{label:`Status`,children:D?(0,F.jsxs)(re,{children:[(0,F.jsx)(s,{label:`Reason`,children:D.reason||`None`}),(0,F.jsx)(s,{label:`Message`,children:D.message||`None`})]}):(0,F.jsx)(`p`,{className:`text-sm text-muted`,children:`No status yet`})}),(0,F.jsx)(o,{label:`Writer`,children:(0,F.jsxs)(`div`,{className:`flex flex-col gap-1 text-sm`,children:[S&&(0,F.jsx)(`p`,{children:f(S)}),S&&!u(S)&&(0,F.jsxs)(`p`,{className:`text-muted`,children:[`Managed by `,f(S),`. Change it there.`]})]})})]}),C?(0,F.jsx)(`fieldset`,{disabled:b||!a||!n,className:`flex min-w-0 flex-col gap-4`,children:(0,F.jsxs)(he,{title:`Settings`,children:[k&&(0,F.jsx)(`p`,{className:`text-sm text-muted`,children:k}),(0,F.jsx)(Ee,{kind:e.kind,namespace:e.namespace,draft:v,errors:O,namespaces:t.namespaces.map(e=>e.metadata?.name??``),databaseConfigs:M(`DatabaseConfig`),databaseServerConfigs:M(`DatabaseServerConfig`),onChange:e=>l.setDraft(()=>e)})]})},`settings-${l.generation}`):(0,F.jsx)(o,{label:`Settings`,children:(0,F.jsx)(re,{children:_e(x).map(e=>(0,F.jsx)(s,{label:e.label,note:e.note,children:e.secret?(0,F.jsx)(j,{secret:e.secret}):e.value},e.label))})}),(0,F.jsx)(o,{label:`Used by`,children:(0,F.jsx)(Oe,{users:g,unread:h})}),C&&a&&(0,F.jsx)(`fieldset`,{disabled:b||!n,className:`flex min-w-0 flex-col gap-4`,children:(0,F.jsx)(he,{title:`Delete the contract`,children:h.length>0?(0,F.jsx)(`p`,{className:`text-sm text-muted`,children:ue(h)}):g.length>0?(0,F.jsx)(`p`,{className:`text-sm text-muted`,children:`Delete is possible when no resource uses this contract.`}):(0,F.jsxs)(ee,{name:e.name,confirmLabel:`Review the delete`,onConfirm:()=>l.review({change:Ce(a),kind:`delete`}),children:[`The contract `,e.name,` is deleted.`]})})},`delete-${l.generation}`),C&&y&&n?(0,F.jsx)(`section`,{"aria-label":w?`Review the delete`:`Review the contract`,className:`rounded-md border border-border bg-panel p-5`,children:(0,F.jsx)(ae,{change:y.change,mode:n,diffLabel:`Changes to the contract`,effects:w?[`The contract ${e.name} is deleted.`,...n===`git`?[`In Git mode the delete is a commit that removes the file of the contract. GitOps must prune the ${e.kind} to delete it from the cluster.`]:[]]:ke(e.kind,g),confirm:w?{label:`Delete the contract`,gitLabel:`Push the delete to Git`,danger:!0}:{label:`Save`},savedMessage:w?`The contract ${e.name} is deleted.`:`The contract ${e.name} is saved.`,preview:r.previewChange,apply:r.applyChange,show:(t,n)=>{pe(t,n),l.written(t,e.name,e.namespace)},onCancel:l.cancel,onWriting:de})}):C&&A>0&&!b&&_&&(0,F.jsx)(fe,{count:A,blocked:k??O[0]?.message,onDiscard:l.discard,onReview:()=>l.review({change:we(_,v),kind:`save`})}),m&&(0,F.jsx)(`div`,{className:`fixed right-4 bottom-4 z-20`,children:(0,F.jsx)(ne,{result:m,onDismiss:me})})]})}var F,I;function L(){return(L=e((()=>{F=n(),I=t(),a(),pe(),m(),l(),c(),i(),r(),_(),de(),v(),d(),w(),D(),E(),b(),A(),N(),C(),S(),O(),P.__docgenInfo={description:`ConnectionDetail is the page of one contract: its own Ready, its writer,
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
}`,signature:{properties:[{key:`previewChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<Preview>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`Preview`}],raw:`Promise<Preview>`}},required:!0}},{key:`applyChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}},required:!0}}]}},description:``}}}})))()}var R,Ae,z,B,je,Me,V,Ne,H,U,W,G,K,q,J,Pe,Y,X,Z,Q,Fe;function $(){return($=e((()=>{R=n(),g(),L(),y(),{expect:Ae,fn:z,userEvent:B,within:je}=__STORYBOOK_MODULE_TEST__,Me=me({"team-a":[{name:`postgres-admin`,keys:[`username`,`password`]},{name:`orders-db`,keys:[`user`,`pass`]}]}),V={previewChange:z(async e=>({objects:[...e.apply,...e.delete??[]].map(e=>{let t=`metadata`in e?{...e.metadata,apiVersion:e.apiVersion,kind:e.kind}:e;return{ref:{apiVersion:t.apiVersion,kind:t.kind,namespace:t.namespace,name:t.name},diff:[{op:`added`,text:`host: pg-2.team-a`}]}}),errors:[]})),applyChange:z(async()=>({ok:!0,mode:`direct`}))},Ne={title:`setup/connections/ConnectionDetail`,component:P,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`shared-es`},sources:{...x(),errors:[]}},decorators:[e=>(0,R.jsx)(h.Provider,{value:Me,children:(0,R.jsx)(e,{})})]},H={name:`Created by hand`},U={name:`Managed by an Elasticsearch cluster`,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`orders-es`}}},W={name:`Written by a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`orders-backup`}}},G={name:`Kept from a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`old-backup`}}},K={name:`Management Identity of a plane`,args:{target:{kind:`ManagementAuthConfig`,name:`plane-a-optimize`}}},q={name:`Database server with default keys`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`}}},J={name:`Not ready`,args:{target:{kind:`DatabaseConfig`,namespace:`team-a`,name:`orders-db`},sources:{...x(),databaseConfigs:x().databaseConfigs.map(e=>e.metadata.name===`orders-db`?{...e,status:{conditions:[{type:`Ready`,status:`False`,reason:`MissingSecret`,message:`Secret "orders-db" has no key "pass"`,lastTransitionTime:`2026-11-10T09:00:00Z`}]}}:e),errors:[]}}},Pe={...x(),databaseConfigs:[],errors:[]},Y={name:`Editing a contract created by hand`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},mode:`direct`,actions:V}},X={name:`Review of a save`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},mode:`direct`,actions:V},play:async({canvasElement:e})=>{let t=je(e),n=t.getByLabelText(`Host`);await B.clear(n),await B.type(n,`pg-2.team-a`),await B.click(t.getByRole(`button`,{name:`Review`})),await Ae(await t.findByRole(`region`,{name:`Review the contract`})).toBeVisible()}},Z={name:`Delete of an unused contract`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},sources:Pe,mode:`direct`,actions:V}},Q={name:`Delete refused for a used contract`,args:{target:{kind:`DatabaseConfig`,namespace:`team-a`,name:`orders-db`},sources:{...x(),secondaryStorageConfigs:[...x().secondaryStorageConfigs,{apiVersion:`core.camunda.io/v1`,kind:`SecondaryStorageConfig`,metadata:{namespace:`team-a`,name:`orders-rdbms`},spec:{type:`rdbms`,rdbms:{databaseConfigRef:`orders-db`}}}],errors:[]},mode:`direct`,actions:V}},Fe=[`CreatedByHand`,`ManagedByAnElasticsearchCluster`,`WrittenByARegistration`,`KeptFromARegistration`,`ManagementIdentityOfAPlane`,`DatabaseServerWithDefaultKeys`,`NotReady`,`EditingAContractCreatedByHand`,`ReviewOfASave`,`DeleteOfAnUnusedContract`,`DeleteRefusedForAUsedContract`],H.parameters={...H.parameters,docs:{...H.parameters?.docs,source:{originalSource:`{
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
}`,...Q.parameters?.docs?.source}}}})))()}$();export{H as CreatedByHand,q as DatabaseServerWithDefaultKeys,Z as DeleteOfAnUnusedContract,Q as DeleteRefusedForAUsedContract,Y as EditingAContractCreatedByHand,G as KeptFromARegistration,U as ManagedByAnElasticsearchCluster,K as ManagementIdentityOfAPlane,J as NotReady,X as ReviewOfASave,W as WrittenByARegistration,Fe as __namedExportsOrder,Ne as default};