import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{n as r}from"./json-Ch039IR8.js";import{t as i}from"./policy-CfVZb04C.js";import{n as a,r as ee,t as o}from"./toast-CpgQvkri.js";import{n as s,t as te}from"./change-review-CWvDfmxG.js";import{n as c,t as l}from"./change-bar-Qn1SP7yi.js";import"./claim-edit-Bah1C5B-.js";import{o as u}from"./unread-Do34-4tt.js";import{n as d}from"./cluster-edit-4ujzjBfr.js";import{i as f,n as p,r as m}from"./namespaced-secret-ref-picker-Bvvo1-23.js";import{a as ne,i as h,o as g,t as _}from"./form-parts-lFdE9SQi.js";import{n as re,t as v}from"./use-setup-editor-DeBPAPn1.js";import{_ as ie,c as ae,f as oe,g as se,h as ce,l as y,m as b,n as x,t as S,u as C,v as w,y as le}from"./fixtures-DZz7XbO3.js";import{a as T,c as E,i as ue,o as D,r as O,t as k}from"./tabs-C4wTFBPX.js";import{n as A,t as j}from"./contract-form-THDKEaQA.js";import{n as M,t as N}from"./impact-ZlqpnSOa.js";function P({tab:e,sources:t,mode:n,actions:i}){let a=k.find(t=>t.value===e),s=a.kind,c=s!==`ManagementAuthConfig`,d=T(t,s),f=e=>c?`${e.metadata.namespace??``}/${e.metadata.name??``}`:e.metadata.name??``,p=e=>d.filter(t=>!c||t.metadata.namespace===e).map(e=>e.metadata.name??``),[m,g]=(0,I.useState)(``),v=()=>y(s,d.find(e=>f(e)===m),{name:``,namespace:``,spec:w(s)}),[b,x]=(0,I.useState)(``),S=re({live:void 0,isNew:!0,listed:p(b),draftOf:()=>({name:``,spec:v().spec}),listHref:O(a.value),hrefOf:(e,t)=>ue({kind:s,namespace:t,name:e}),onReset:()=>x(v().namespace)}),{reviewing:E,locked:D}=S,A={...S.draft,namespace:b},{toast:M,show:P,dismiss:L}=ee(),R=[...se(s,A.name,p(A.namespace),c?void 0:le(A.name,t.planes,t.planeClaims)),...ie(s,A.namespace),...oe(s,A)],z=e=>R.find(t=>t.path===e)?.message,B=ae({spec:w(s)},A)+(A.name===``?0:1)+(A.namespace===``?0:1),V=t.namespaces.map(e=>e.metadata?.name??``).sort(r),H=e=>u(t).includes(e)?void 0:T(t,e).filter(e=>e.metadata.namespace===A.namespace).map(e=>e.metadata.name??``),U=d.map(f).sort(r);return(0,F.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,F.jsxs)(`header`,{className:`flex flex-col gap-2`,children:[(0,F.jsx)(`p`,{className:`text-sm text-muted`,children:a.label}),(0,F.jsxs)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em]`,children:[`New `,ce(s),` contract`]})]}),!n&&(0,F.jsx)(`p`,{role:`alert`,className:`text-sm text-bad`,children:`Changes are off: the fleet manager could not read its settings.`}),t.errors.map(e=>(0,F.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind)),(0,F.jsxs)(`fieldset`,{disabled:D||!n,className:`flex min-w-0 flex-col gap-4`,children:[(0,F.jsxs)(_,{title:`Basics`,children:[(0,F.jsx)(h,{label:`Copy from`,value:m,options:[[``,`Nothing, start empty`],...U.map(e=>[e,e])],onChange:e=>{g(e);let t=y(s,d.find(t=>f(t)===e),A);x(t.namespace),S.setDraft(()=>({name:t.name,spec:t.spec}))}}),(0,F.jsx)(ne,{label:`Name`,value:A.name,mono:!0,hint:`Lowercase letters, digits, and hyphens. It cannot change later.`,error:z(`name`),onChange:e=>S.setDraft(t=>({...t,name:e}))}),c&&(0,F.jsx)(h,{label:`Namespace`,value:A.namespace,options:[[``,`Pick a namespace`],...V.map(e=>[e,e])],error:z(`namespace`),onChange:x})]}),(0,F.jsx)(I.Fragment,{children:(0,F.jsx)(_,{title:`Settings`,children:c&&A.namespace===``?(0,F.jsx)(`p`,{className:`text-sm text-muted`,children:`Pick the namespace of the contract first.`}):(0,F.jsx)(j,{kind:s,namespace:A.namespace,draft:A,errors:R,namespaces:V,databaseConfigs:H(`DatabaseConfig`),databaseServerConfigs:H(`DatabaseServerConfig`),choose:!0,onChange:e=>S.setDraft(t=>({...t,spec:e.spec}))})})},`${S.generation}-${m}`)]}),E&&n?(0,F.jsx)(`section`,{"aria-label":`Review the contract`,className:`rounded-md border border-border bg-panel p-5`,children:(0,F.jsx)(te,{change:E.change,mode:n,diffLabel:`Changes to the contract`,effects:[N],confirm:{label:`Create`},savedMessage:`The contract ${A.name} is created.`,preview:i.previewChange,apply:i.applyChange,show:(e,t)=>{P(e,t),S.written(e,A.name,c?A.namespace:void 0)},onCancel:S.cancel})}):B>0&&!D&&(0,F.jsx)(l,{count:B,blocked:R[0]?.message,onDiscard:S.discard,onReview:()=>S.review({change:C(s,A.namespace,A.name,A),kind:`save`})}),M&&(0,F.jsx)(`div`,{className:`fixed right-4 bottom-4 z-20`,children:(0,F.jsx)(o,{result:M,onDismiss:L})})]})}var F,I;function L(){return(L=e((()=>{F=n(),I=t(),d(),c(),s(),a(),g(),v(),i(),A(),b(),M(),E(),D(),P.__docgenInfo={description:`ContractCreate is the create page of a contract of the kind of tab on
sources. It starts with the new spec of the kind, or as a copy of the
spec of the contract that Copy from names. The create shows a review with
a dry run before the write, and the page moves to the new contract once
the list shows it. Without mode the form is disabled.`,methods:[],displayName:`ContractCreate`,props:{tab:{required:!0,tsType:{name:`union`,raw:`| "object-storage"
| "secondary-storage"
| "database-servers"
| "databases"
| "management-identity"`,elements:[{name:`literal`,value:`"object-storage"`},{name:`literal`,value:`"secondary-storage"`},{name:`literal`,value:`"database-servers"`},{name:`literal`,value:`"databases"`},{name:`literal`,value:`"management-identity"`}]},description:``},sources:{required:!0,tsType:{name:`intersection`,raw:`ConnectionSources & { errors: ReadError[] } & SourceReads`,elements:[{name:`intersection`,raw:`ContractUserSources & {
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
}`,signature:{properties:[{key:`objectStorageConfigs`,value:{name:`Array`,elements:[{name:`ObjectStorageConfig`}],raw:`ObjectStorageConfig[]`,required:!0}},{key:`databaseServerConfigs`,value:{name:`Array`,elements:[{name:`DatabaseServerConfig`}],raw:`DatabaseServerConfig[]`,required:!0}},{key:`managementAuthConfigs`,value:{name:`Array`,elements:[{name:`ManagementAuthConfig`}],raw:`ManagementAuthConfig[]`,required:!0}},{key:`requests`,value:{name:`Array`,elements:[{name:`PluginRequest`}],raw:`PluginRequest[]`,required:!0}},{key:`planeClaims`,value:{name:`Array`,elements:[{name:`CamundaManagementClusterClaim`}],raw:`CamundaManagementClusterClaim[]`,required:!0}},{key:`registrations`,value:{name:`Array`,elements:[{name:`PluginRegistration`}],raw:`PluginRegistration[]`,required:!0}}]}}]},{name:`signature`,type:`object`,raw:`{ errors: ReadError[] }`,signature:{properties:[{key:`errors`,value:{name:`Array`,elements:[{name:`ReadError`}],raw:`ReadError[]`,required:!0}}]}},{name:`SourceReads`}]},description:``},mode:{required:!1,tsType:{name:`WriteMode`},description:``},actions:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  previewChange: (change: Change) => Promise<Preview>;
  applyChange: (change: Change) => Promise<WriteResult>;
}`,signature:{properties:[{key:`previewChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<Preview>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`Preview`}],raw:`Promise<Preview>`}},required:!0}},{key:`applyChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}},required:!0}}]}},description:``}}}})))()}var R,z,B,V,H,U,W,G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{R=n(),m(),L(),x(),{expect:z,fn:B,userEvent:V,within:H}=__STORYBOOK_MODULE_TEST__,U=f({"team-a":[{name:`gcs`,keys:[`key.json`]},{name:`postgres-admin`,keys:[`username`,`password`]}],platform:[{name:`plane-a-optimize-client`,keys:[`client-secret`]}]}),W={title:`setup/connections/ContractCreate`,component:P,args:{tab:`object-storage`,sources:{...S(),errors:[]},mode:`direct`,actions:{previewChange:B(async()=>({objects:[],errors:[]})),applyChange:B(async()=>({ok:!0,mode:`direct`}))}},decorators:[e=>(0,R.jsx)(p.Provider,{value:U,children:(0,R.jsx)(e,{})})]},G=async({canvasElement:e})=>{await V.selectOptions(H(e).getByLabelText(`Namespace`,{exact:!0}),`team-a`)},K={name:`Object storage`,play:async e=>{await G(e);let t=H(e.canvasElement);await V.click(t.getByRole(`radio`,{name:`GCS`})),await z(t.getByLabelText(`Bucket name`)).toBeVisible(),await z(t.queryByLabelText(`Region`)).toBeNull()}},q={name:`Secondary storage`,args:{tab:`secondary-storage`},play:G},J={name:`Database server`,args:{tab:`database-servers`},play:G},Y={name:`Database`,args:{tab:`databases`},play:G},X={name:`Management Identity`,args:{tab:`management-identity`}},Z={name:`Changes off`,args:{mode:void 0}},Q=[`ObjectStorage`,`SecondaryStorage`,`DatabaseServer`,`Database`,`ManagementIdentity`,`ChangesOff`],K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
  name: "Object storage",
  play: async context => {
    await inTeamA(context);
    const canvas = within(context.canvasElement);
    await userEvent.click(canvas.getByRole("radio", {
      name: "GCS"
    }));
    await expect(canvas.getByLabelText("Bucket name")).toBeVisible();
    await expect(canvas.queryByLabelText("Region")).toBeNull();
  }
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  name: "Secondary storage",
  args: {
    tab: "secondary-storage"
  },
  play: inTeamA
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "Database server",
  args: {
    tab: "database-servers"
  },
  play: inTeamA
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "Database",
  args: {
    tab: "databases"
  },
  play: inTeamA
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "Management Identity",
  args: {
    tab: "management-identity"
  }
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "Changes off",
  args: {
    mode: undefined
  }
}`,...Z.parameters?.docs?.source}}}})))()}$();export{Z as ChangesOff,Y as Database,J as DatabaseServer,X as ManagementIdentity,K as ObjectStorage,q as SecondaryStorage,Q as __namedExportsOrder,W as default};