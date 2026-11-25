import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{n as r}from"./json-Ch039IR8.js";import{t as i}from"./policy-BJNT8nX8.js";import{n as a,r as o,t as s}from"./toast-BU0ZvRbB.js";import{n as c,t as ee}from"./change-review-DhW3sNom.js";import{n as l,t as te}from"./change-bar-BoZNojva.js";import"./claim-edit-DiW9Ybtk.js";import{o as u}from"./unread-Do34-4tt.js";import{n as d}from"./cluster-edit-Dj8Kx6VA.js";import{i as f,n as p,r as m}from"./namespaced-secret-ref-picker-qy2HqROA.js";import{a as h,i as g,o as _,t as v}from"./form-parts-BJv_aLsW.js";import{n as ne,t as y}from"./use-setup-editor--CN__9-L.js";import{_ as re,c as ie,f as ae,g as oe,h as b,l as se,m as x,n as S,t as C,u as w,v as T,y as ce}from"./fixtures-CvEPNoLG.js";import{a as E,c as D,i as O,o as k,r as A,t as le}from"./tabs-D2UsMOSj.js";import{n as j,t as M}from"./contract-form-BxTcmN3w.js";import{n as N,t as ue}from"./impact-ZlqpnSOa.js";function P({tab:e,sources:t,mode:n,actions:i}){let a=le.find(t=>t.value===e),c=a.kind,l=c!==`ManagementAuthConfig`,d=E(t,c),f=e=>l?`${e.metadata.namespace??``}/${e.metadata.name??``}`:e.metadata.name??``,p=e=>d.filter(t=>!l||t.metadata.namespace===e).map(e=>e.metadata.name??``),[m,_]=(0,I.useState)(``),[y,x]=(0,I.useState)(``),S=ne({live:void 0,isNew:!0,listed:p(y),draftOf:()=>({name:``,spec:T(c)}),listHref:A(a.value),hrefOf:(e,t)=>O({kind:c,namespace:t,name:e}),onReset:()=>{_(``),x(``)}}),{reviewing:C,locked:D}=S,k={...S.draft,namespace:y},{toast:j,show:N,dismiss:P}=o(),L=[...oe(c,k.name,p(k.namespace),l?void 0:ce(k.name,t.planes,t.planeClaims)),...re(c,k.namespace),...ae(c,k)],R=e=>L.find(t=>t.path===e)?.message,z=ie({spec:T(c)},k)+(k.name===``?0:1)+(k.namespace===``?0:1),B=t.namespaces.map(e=>e.metadata?.name??``).sort(r),V=e=>u(t).includes(e)?void 0:E(t,e).filter(e=>e.metadata.namespace===k.namespace).map(e=>e.metadata.name??``),H=d.map(f).sort(r);return(0,F.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,F.jsxs)(`header`,{className:`flex flex-col gap-2`,children:[(0,F.jsx)(`p`,{className:`text-sm text-muted`,children:a.label}),(0,F.jsxs)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em]`,children:[`New `,b(c),` contract`]})]}),!n&&(0,F.jsx)(`p`,{role:`alert`,className:`text-sm text-bad`,children:`Changes are off: the fleet manager could not read its settings.`}),t.errors.map(e=>(0,F.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind)),(0,F.jsxs)(`fieldset`,{disabled:D||!n,className:`flex min-w-0 flex-col gap-4`,children:[(0,F.jsxs)(v,{title:`Basics`,children:[(0,F.jsx)(g,{label:`Copy from`,value:m,options:[[``,`Nothing, start empty`],...H.map(e=>[e,e])],onChange:e=>{_(e);let t=se(c,d.find(t=>f(t)===e),k);x(t.namespace),S.setDraft(()=>({name:t.name,spec:t.spec}))}}),(0,F.jsx)(h,{label:`Name`,value:k.name,mono:!0,hint:`Lowercase letters, digits, and hyphens. It cannot change later.`,error:R(`name`),onChange:e=>S.setDraft(t=>({...t,name:e}))}),l&&(0,F.jsx)(g,{label:`Namespace`,value:k.namespace,options:[[``,`Pick a namespace`],...B.map(e=>[e,e])],error:R(`namespace`),onChange:x})]}),(0,F.jsx)(I.Fragment,{children:(0,F.jsx)(v,{title:`Settings`,children:l&&k.namespace===``?(0,F.jsx)(`p`,{className:`text-sm text-muted`,children:`Pick the namespace of the contract first.`}):(0,F.jsx)(M,{kind:c,namespace:k.namespace,draft:k,errors:L,namespaces:B,databaseConfigs:V(`DatabaseConfig`),databaseServerConfigs:V(`DatabaseServerConfig`),choose:!0,onChange:e=>S.setDraft(t=>({...t,spec:e.spec}))})})},`${S.generation}-${m}`)]}),C&&n?(0,F.jsx)(`section`,{"aria-label":`Review the contract`,className:`rounded-md border border-border bg-panel p-5`,children:(0,F.jsx)(ee,{change:C.change,mode:n,diffLabel:`Changes to the contract`,effects:[ue],confirm:{label:`Create`},savedMessage:`The contract ${k.name} is created.`,preview:i.previewChange,apply:i.applyChange,show:(e,t)=>{N(e,t),S.written(e,k.name,l?k.namespace:void 0)},onCancel:S.cancel})}):z>0&&!D&&(0,F.jsx)(te,{count:z,blocked:L[0]?.message,onDiscard:S.discard,onReview:()=>S.review({change:w(c,k.namespace,k.name,k),kind:`save`})}),j&&(0,F.jsx)(`div`,{className:`fixed right-4 bottom-4 z-20`,children:(0,F.jsx)(s,{result:j,onDismiss:P})})]})}var F,I;function L(){return(L=e((()=>{F=n(),I=t(),d(),l(),c(),a(),_(),y(),i(),j(),x(),N(),D(),k(),P.__docgenInfo={description:`ContractCreate is the create page of a contract of the kind of tab on
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
}`,signature:{properties:[{key:`previewChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<Preview>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`Preview`}],raw:`Promise<Preview>`}},required:!0}},{key:`applyChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}},required:!0}}]}},description:``}}}})))()}var R,z,B,V,H,U,W,G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{R=n(),m(),L(),S(),{expect:z,fn:B,userEvent:V,within:H}=__STORYBOOK_MODULE_TEST__,U=f({"team-a":[{name:`gcs`,keys:[`key.json`]},{name:`postgres-admin`,keys:[`username`,`password`]}],platform:[{name:`plane-a-optimize-client`,keys:[`client-secret`]}]}),W={title:`setup/connections/ContractCreate`,component:P,args:{tab:`object-storage`,sources:{...C(),errors:[]},mode:`direct`,actions:{previewChange:B(async()=>({objects:[],errors:[]})),applyChange:B(async()=>({ok:!0,mode:`direct`}))}},decorators:[e=>(0,R.jsx)(p.Provider,{value:U,children:(0,R.jsx)(e,{})})]},G=async({canvasElement:e})=>{await V.selectOptions(H(e).getByLabelText(`Namespace`,{exact:!0}),`team-a`)},K={name:`Object storage`,play:async e=>{await G(e);let t=H(e.canvasElement);await V.click(t.getByRole(`radio`,{name:`GCS`})),await z(t.getByLabelText(`Bucket name`)).toBeVisible(),await z(t.queryByLabelText(`Region`)).toBeNull()}},q={name:`Secondary storage`,args:{tab:`secondary-storage`},play:G},J={name:`Database server`,args:{tab:`database-servers`},play:G},Y={name:`Database`,args:{tab:`databases`},play:G},X={name:`Management Identity`,args:{tab:`management-identity`}},Z={name:`Changes off`,args:{mode:void 0}},Q=[`ObjectStorage`,`SecondaryStorage`,`DatabaseServer`,`Database`,`ManagementIdentity`,`ChangesOff`],K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
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