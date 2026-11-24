import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{P as ee,t as r}from"./policy-BJNT8nX8.js";import{r as i,t as te}from"./typed-confirm-CqmOIrho.js";import{a as ne,i as a,n as o,o as s}from"./claim-tab-BHivgop0.js";import{n as c,r as re,t as ie}from"./toast-CrHHXeuP.js";import{n as l,t as ae}from"./status-chip-C-piPCzS.js";import{n as u,t as oe}from"./change-review-DDHLTuah.js";import{n as d,t as se}from"./change-bar-Dzb-NCuR.js";import"./claim-edit-Dn2HGNu3.js";import{a as ce,i as le,o as f}from"./unread-Do34-4tt.js";import{n as p}from"./cluster-edit-DI71Fbu7.js";import{C as ue,D as de,O as fe,T as pe,b as me,t as he,w as ge,x as _e,y as ve}from"./usage-CjhE3RAI.js";import{i as m,n as h,r as g}from"./namespaced-secret-ref-picker-FgHGtSkU.js";import{o as _,t as ye}from"./form-parts-DHR7z0N2.js";import{n as be,t as v}from"./use-setup-editor-DYV5oLNa.js";import{a as xe,b as Se,c as Ce,d as we,f as Te,m as y,n as b,o as x,p as Ee,t as S}from"./fixtures-CvEPNoLG.js";import{a as C,c as w,i as De,o as T,r as Oe,t as ke}from"./tabs-D2UsMOSj.js";import{i as Ae,r as E}from"./connection-list--QDXsCLs.js";import{n as D,t as je}from"./contract-form-y5fNFrJY.js";import{n as O,r as Me}from"./impact-ZlqpnSOa.js";import{n as k,t as Ne}from"./users-table-CdwZQNCF.js";function A({secret:e}){return(0,j.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-1`,children:[(0,j.jsx)(`span`,{className:`font-mono text-sm [overflow-wrap:anywhere]`,children:e.namespace?`${e.namespace}/${e.name}`:e.name}),(0,j.jsx)(`dl`,{className:`grid grid-cols-[max-content_1fr] gap-x-3 gap-y-0.5 text-xs`,children:e.keys.map(([e,t])=>(0,j.jsxs)(`div`,{className:`contents`,children:[(0,j.jsx)(`dt`,{className:`text-muted`,children:e}),(0,j.jsx)(`dd`,{className:`font-mono text-text-2 [overflow-wrap:anywhere]`,children:t})]},e))})]})}var j;function M(){return(M=e((()=>{j=n(),A.__docgenInfo={description:`SecretRefView shows one Secret reference: the Secret, with its namespace
when it has one, and each key that the contract reads. It shows names
and keys only, never a value.`,methods:[],displayName:`SecretRefView`,props:{secret:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  namespace?: string;
  name: string;
  keys: readonly (readonly [label: string, key: string])[];
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!1}},{key:`name`,value:{name:`string`,required:!0}},{key:`keys`,value:{name:`unknown`,required:!0}}]}},description:``}}}})))()}function N({target:e,sources:t,mode:n,actions:r}){let i=C(t,e.kind),s=i.find(t=>t.metadata.name===e.name&&(e.namespace===void 0||t.metadata.namespace===e.namespace)),c=ke.find(t=>t.kind===e.kind),l=be({live:s,isNew:!1,listed:i.filter(t=>t.metadata.namespace===e.namespace).map(e=>e.metadata.name??``),draftOf:Ee,listHref:Oe(c.value),hrefOf:(t,n)=>De({kind:e.kind,namespace:n,name:t})}),[u,d]=(0,F.useState)(!1),{toast:p,show:he,dismiss:m}=re(),h=ue(e.kind,f(t)),g=pe(e,t,f(t)),_=me(t).get(ve(e))??[],{base:v,draft:y,reviewing:b,locked:x}=l,S=s??v,w=S&&_e(S,Ae(t),t.planes),T=r!==void 0&&w!==void 0&&de(w),E=b?.kind===`delete`;b&&!u&&(!T||h.length>0||E&&(_.length>0||g.length>0))&&l.cancel();let D=t.errors.map(e=>(0,P.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind));if(!S)return(0,P.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[D,!f(t).includes(e.kind)&&(0,P.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`The contract `,e.name,` does not exist.`]})]});let O=ee(S.status?.conditions),k=Te(e.kind,y),j=h.length>0?le(h):void 0,M=v?Ce(v,y):0,N=n=>f(t).includes(n)?void 0:C(t,n).filter(t=>t.metadata.namespace===e.namespace).map(e=>e.metadata.name??``);return(0,P.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,P.jsxs)(`header`,{className:`flex flex-col gap-2`,children:[(0,P.jsx)(`p`,{className:`text-sm text-muted`,children:c.label}),(0,P.jsxs)(`div`,{className:`flex flex-wrap items-center gap-3`,children:[(0,P.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em] [overflow-wrap:anywhere]`,children:e.name}),(0,P.jsx)(ae,{condition:O})]}),e.namespace!==void 0&&(0,P.jsx)(`p`,{className:`font-mono text-sm text-muted`,children:e.namespace})]}),T&&!n&&(0,P.jsx)(`p`,{role:`alert`,className:`text-sm text-bad`,children:`Changes are off: the fleet manager could not read its settings.`}),D,!s&&(0,P.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`The contract `,e.name,` was deleted.`]}),(0,P.jsxs)(`div`,{className:`grid gap-4 lg:grid-cols-2`,children:[(0,P.jsx)(o,{label:`Status`,children:O?(0,P.jsxs)(ne,{children:[(0,P.jsx)(a,{label:`Reason`,children:O.reason||`None`}),(0,P.jsx)(a,{label:`Message`,children:O.message||`None`})]}):(0,P.jsx)(`p`,{className:`text-sm text-muted`,children:`No status yet`})}),(0,P.jsx)(o,{label:`Writer`,children:(0,P.jsxs)(`div`,{className:`flex flex-col gap-1 text-sm`,children:[w&&(0,P.jsx)(`p`,{children:fe(w)}),w&&!de(w)&&(0,P.jsxs)(`p`,{className:`text-muted`,children:[`Managed by `,fe(w),`. Change it there.`]})]})})]}),T?(0,P.jsx)(`fieldset`,{disabled:x||!s||!n,className:`flex min-w-0 flex-col gap-4`,children:(0,P.jsxs)(ye,{title:`Settings`,children:[j&&(0,P.jsx)(`p`,{className:`text-sm text-muted`,children:j}),(0,P.jsx)(je,{kind:e.kind,namespace:e.namespace,draft:y,errors:k,namespaces:t.namespaces.map(e=>e.metadata?.name??``),databaseConfigs:N(`DatabaseConfig`),databaseServerConfigs:N(`DatabaseServerConfig`),onChange:e=>l.setDraft(()=>e)})]})},`settings-${l.generation}`):(0,P.jsx)(o,{label:`Settings`,children:(0,P.jsx)(ne,{children:xe(S).map(e=>(0,P.jsx)(a,{label:e.label,note:e.note,children:e.secret?(0,P.jsx)(A,{secret:e.secret}):e.value},e.label))})}),(0,P.jsx)(o,{label:`Used by`,children:(0,P.jsx)(Ne,{users:_,unread:h,waiting:g})}),T&&s&&(0,P.jsx)(`fieldset`,{disabled:x||!n,className:`flex min-w-0 flex-col gap-4`,children:(0,P.jsx)(ye,{title:`Delete the contract`,children:h.length>0?(0,P.jsx)(`p`,{className:`text-sm text-muted`,children:ce(h)}):_.length>0?(0,P.jsx)(`p`,{className:`text-sm text-muted`,children:`Delete is possible when no resource uses this contract.`}):g.length>0?(0,P.jsx)(`p`,{className:`text-sm text-muted`,children:ge(g)}):(0,P.jsxs)(te,{name:e.name,confirmLabel:`Review the delete`,onConfirm:()=>l.review({change:we(s),kind:`delete`}),children:[`The contract `,e.name,` is deleted.`]})})},`delete-${l.generation}`),T&&b&&n?(0,P.jsx)(`section`,{"aria-label":E?`Review the delete`:`Review the contract`,className:`rounded-md border border-border bg-panel p-5`,children:(0,P.jsx)(oe,{change:b.change,mode:n,diffLabel:`Changes to the contract`,effects:E?[`The contract ${e.name} is deleted.`,...n===`git`?[`In Git mode the delete is a commit that removes the file of the contract. GitOps must prune the ${e.kind} to delete it from the cluster.`]:[]]:Me(e.kind,_),confirm:E?{label:`Delete the contract`,gitLabel:`Push the delete to Git`,danger:!0}:{label:`Save`},savedMessage:E?`The contract ${e.name} is deleted.`:`The contract ${e.name} is saved.`,preview:r.previewChange,apply:r.applyChange,show:(t,n)=>{he(t,n),l.written(t,e.name,e.namespace)},onCancel:l.cancel,onWriting:d})}):T&&M>0&&!x&&v&&(0,P.jsx)(se,{count:M,blocked:j??k[0]?.message,onDiscard:l.discard,onReview:()=>l.review({change:Se(v,y),kind:`save`})}),p&&(0,P.jsx)(`div`,{className:`fixed right-4 bottom-4 z-20`,children:(0,P.jsx)(ie,{result:p,onDismiss:m})})]})}var P,F;function I(){return(I=e((()=>{P=n(),F=t(),s(),p(),d(),u(),l(),c(),i(),_(),v(),he(),E(),D(),y(),x(),r(),O(),M(),w(),T(),k(),N.__docgenInfo={description:`ConnectionDetail is the page of one contract: its own Ready, its writer,
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
}`,signature:{properties:[{key:`previewChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<Preview>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`Preview`}],raw:`Promise<Preview>`}},required:!0}},{key:`applyChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}},required:!0}}]}},description:``}}}})))()}var L,R,z,B,Pe,Fe,V,Ie,H,U,W,G,K,q,J,Le,Y,X,Z,Q,Re;function $(){return($=e((()=>{L=n(),g(),I(),b(),{expect:R,fn:z,userEvent:B,within:Pe}=__STORYBOOK_MODULE_TEST__,Fe=m({"team-a":[{name:`postgres-admin`,keys:[`username`,`password`]},{name:`orders-db`,keys:[`user`,`pass`]}]}),V={previewChange:z(async e=>({objects:[...e.apply,...e.delete??[]].map(e=>{let t=`metadata`in e?{...e.metadata,apiVersion:e.apiVersion,kind:e.kind}:e;return{ref:{apiVersion:t.apiVersion,kind:t.kind,namespace:t.namespace,name:t.name},diff:[{op:`added`,text:`host: pg-2.team-a`}]}}),errors:[]})),applyChange:z(async()=>({ok:!0,mode:`direct`}))},Ie={title:`setup/connections/ConnectionDetail`,component:N,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`shared-es`},sources:{...S(),errors:[]}},decorators:[e=>(0,L.jsx)(h.Provider,{value:Fe,children:(0,L.jsx)(e,{})})]},H={name:`Created by hand`},U={name:`Managed by an Elasticsearch cluster`,args:{target:{kind:`SecondaryStorageConfig`,namespace:`team-a`,name:`orders-es`}}},W={name:`Written by a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`orders-backup`}}},G={name:`Kept from a registration`,args:{target:{kind:`ObjectStorageConfig`,namespace:`team-a`,name:`old-backup`}}},K={name:`Management Identity of a plane`,args:{target:{kind:`ManagementAuthConfig`,name:`plane-a-optimize`}}},q={name:`Database server with default keys`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`}}},J={name:`Not ready`,args:{target:{kind:`DatabaseConfig`,namespace:`team-a`,name:`orders-db`},sources:{...S(),databaseConfigs:S().databaseConfigs.map(e=>e.metadata.name===`orders-db`?{...e,status:{conditions:[{type:`Ready`,status:`False`,reason:`MissingSecret`,message:`Secret "orders-db" has no key "pass"`,lastTransitionTime:`2026-11-10T09:00:00Z`}]}}:e),errors:[]}}},Le={...S(),databaseConfigs:[],errors:[]},Y={name:`Editing a contract created by hand`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},mode:`direct`,actions:V}},X={name:`Review of a save`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},mode:`direct`,actions:V},play:async({canvasElement:e})=>{let t=Pe(e),n=t.getByLabelText(`Host`);await B.clear(n),await B.type(n,`pg-2.team-a`),await B.click(t.getByRole(`button`,{name:`Review`})),await R(await t.findByRole(`region`,{name:`Review the contract`})).toBeVisible()}},Z={name:`Delete of an unused contract`,args:{target:{kind:`DatabaseServerConfig`,namespace:`team-a`,name:`postgres`},sources:Le,mode:`direct`,actions:V}},Q={name:`Delete refused for a used contract`,args:{target:{kind:`DatabaseConfig`,namespace:`team-a`,name:`orders-db`},sources:{...S(),secondaryStorageConfigs:[...S().secondaryStorageConfigs,{apiVersion:`core.camunda.io/v1`,kind:`SecondaryStorageConfig`,metadata:{namespace:`team-a`,name:`orders-rdbms`},spec:{type:`rdbms`,rdbms:{databaseConfigRef:`orders-db`}}}],errors:[]},mode:`direct`,actions:V}},Re=[`CreatedByHand`,`ManagedByAnElasticsearchCluster`,`WrittenByARegistration`,`KeptFromARegistration`,`ManagementIdentityOfAPlane`,`DatabaseServerWithDefaultKeys`,`NotReady`,`EditingAContractCreatedByHand`,`ReviewOfASave`,`DeleteOfAnUnusedContract`,`DeleteRefusedForAUsedContract`],H.parameters={...H.parameters,docs:{...H.parameters?.docs,source:{originalSource:`{
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
}`,...Q.parameters?.docs?.source}}}})))()}$();export{H as CreatedByHand,q as DatabaseServerWithDefaultKeys,Z as DeleteOfAnUnusedContract,Q as DeleteRefusedForAUsedContract,Y as EditingAContractCreatedByHand,G as KeptFromARegistration,U as ManagedByAnElasticsearchCluster,K as ManagementIdentityOfAPlane,J as NotReady,X as ReviewOfASave,W as WrittenByARegistration,Re as __namedExportsOrder,Ie as default};