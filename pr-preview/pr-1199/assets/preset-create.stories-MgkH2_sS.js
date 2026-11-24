import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{n as r,t as i}from"./radio-D6t0wyze.js";import{n as a,r as o,t as s}from"./toast-qMr1e2C4.js";import{n as c,t as l}from"./change-review-BpprsTYY.js";import{n as u,t as d}from"./change-bar-CpRItSjI.js";import"./claim-edit-DCAfMYFm.js";import{n as f}from"./cluster-edit-BX4ZtZsT.js";import{h as p,t as m}from"./usage-CjhE3RAI.js";import{a as h,i as g,o as _,t as v}from"./form-parts-CJyo4NtJ.js";import{n as y,t as b}from"./use-setup-editor-CSJClHbx.js";import{n as x,t as S}from"./cannot-set-section-CCM-V-qc.js";import{n as C,t as w}from"./fixtures-CF8JYqCq.js";import{a as T,n as E,r as D,s as O,t as k}from"./kinds-C3Kxnrdb.js";import{a as A,i as j,n as M,o as N,r as P,t as F}from"./draft-BILk3XgT.js";import{n as I,t as L}from"./preset-editor-CpTVikLv.js";import{t as R}from"./sources-BLjkzVoA.js";function z({sources:e,mode:t,actions:n}){let[r,a]=(0,V.useState)(`CamundaClusterPreset`),[c,u]=(0,V.useState)(``),[f,m]=(0,V.useState)([]),_=O(r,e),b=_.map(e=>e.metadata.name??``),x=e=>N(_.find(t=>t.metadata.name===e)).body,C=y({live:void 0,isNew:!0,listed:b,draftOf:()=>({name:``,preset:{body:x(c),description:``}}),listHref:k,hrefOf:e=>T(r,e),onReset:()=>m([])}),{draft:w,reviewing:D,locked:P}=C,{toast:I,show:R,dismiss:z}=o(),H=j(w.name,b)[0]?.message,U=A(void 0,w.preset)+(w.name===``?0:1),W=e.errors.some(e=>e.kind===`Environment`)?void 0:p(e.environments,r,w.name),G=e=>C.setDraft(t=>({...t,preset:{...t.preset,body:e}}));return(0,B.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,B.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em]`,children:`New preset`}),!t&&(0,B.jsx)(`p`,{role:`alert`,className:`text-sm text-bad`,children:`Changes are off: the fleet manager could not read its settings.`}),e.errors.map(e=>(0,B.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind)),(0,B.jsxs)(`fieldset`,{disabled:P||!t,className:`flex min-w-0 flex-col gap-4`,children:[(0,B.jsxs)(v,{title:`Basics`,children:[(0,B.jsxs)(`fieldset`,{className:`flex flex-col gap-2`,children:[(0,B.jsx)(`legend`,{className:`mb-1 text-sm font-medium text-text`,children:`Kind`}),E.map(e=>(0,B.jsxs)(`label`,{className:`flex min-h-6 items-center gap-2 text-sm`,children:[(0,B.jsx)(i,{name:`preset-kind`,checked:r===e.kind,onChange:()=>{a(e.kind),u(``),m([]),G({})}}),e.label]},e.kind))]}),(0,B.jsx)(g,{label:`Copy from`,value:c,options:[[``,`Nothing, start empty`],...b.map(e=>[e,e])],onChange:e=>{u(e),m([]),G(x(e))}}),(0,B.jsx)(h,{label:`Name`,value:w.name,mono:!0,hint:`Lowercase letters, digits, and hyphens. It cannot change later.`,error:H,onChange:e=>C.setDraft(t=>({...t,name:e}))})]}),(0,B.jsx)(L,{kind:r,draft:w.preset,secrets:{},secretsUnread:[],noSecretsText:`To pick a Secret, open the page of the preset once an environment allows it.`,onDraft:e=>C.setDraft(t=>({...t,preset:e(t.preset)})),onErrors:m},`${C.generation}-${r}-${c}`)]}),(0,B.jsx)(S,{kind:r}),D&&t?(0,B.jsx)(`section`,{"aria-label":`Review the preset`,className:`rounded-md border border-border bg-panel p-5`,children:(0,B.jsx)(l,{change:D.change,mode:t,diffLabel:`Changes to the preset`,effects:M(W),confirm:{label:`Create`},savedMessage:`The preset ${w.name} is created.`,preview:n.previewChange,apply:n.applyChange,show:(e,t)=>{R(e,t),C.written(e,w.name)},onCancel:C.cancel})}):U>0&&!P&&(0,B.jsx)(d,{count:U,blocked:H??f[0],onDiscard:C.discard,onReview:()=>C.review({change:F(r,w.name,w.preset),kind:`save`})}),I&&(0,B.jsx)(`div`,{className:`fixed right-4 bottom-4 z-20`,children:(0,B.jsx)(s,{result:I,onDismiss:z})})]})}var B,V;function H(){return(H=e((()=>{B=n(),V=t(),f(),u(),c(),a(),_(),b(),r(),m(),x(),P(),D(),I(),R(),z.__docgenInfo={description:`PresetCreate is the create page of a preset on sources. The admin picks
the kind, then starts empty or as a copy of the body of a preset of that
kind. A new kind starts empty again. The create shows a review with a dry
run before the write, and the page moves to the new preset once its list
shows it. Without mode the form is disabled.`,methods:[],displayName:`PresetCreate`,props:{sources:{required:!0,tsType:{name:`intersection`,raw:`PresetSources & { errors: ReadError[] }`,elements:[{name:`signature`,type:`object`,raw:`{
  -readonly [K in keyof typeof PRESET_SOURCE_KINDS]: Resource<
    (typeof PRESET_SOURCE_KINDS)[K]
  >[];
}`,signature:{properties:[{key:{name:`unknown`,required:!0},value:{name:`Array`,elements:[{name:`Resource`,elements:[{name:`unknown[K]`,raw:`(typeof PRESET_SOURCE_KINDS)[K]`}],raw:`Resource<
  (typeof PRESET_SOURCE_KINDS)[K]
>`}],raw:`Resource<
  (typeof PRESET_SOURCE_KINDS)[K]
>[]`}}]}},{name:`signature`,type:`object`,raw:`{ errors: ReadError[] }`,signature:{properties:[{key:`errors`,value:{name:`Array`,elements:[{name:`ReadError`}],raw:`ReadError[]`,required:!0}}]}}]},description:``},mode:{required:!1,tsType:{name:`WriteMode`},description:``},actions:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  previewChange: (change: Change) => Promise<Preview>;
  applyChange: (change: Change) => Promise<WriteResult>;
}`,signature:{properties:[{key:`previewChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<Preview>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`Preview`}],raw:`Promise<Preview>`}},required:!0}},{key:`applyChange`,value:{name:`signature`,type:`function`,raw:`(change: Change) => Promise<WriteResult>`,signature:{arguments:[{type:{name:`Change`},name:`change`}],return:{name:`Promise`,elements:[{name:`WriteResult`}],raw:`Promise<WriteResult>`}},required:!0}}]}},description:``}}}})))()}var U,W,G,K,q,J,Y,X,Z;function Q(){return(Q=e((()=>{w(),H(),{expect:U,fn:W,userEvent:G,within:K}=__STORYBOOK_MODULE_TEST__,q={title:`setup/presets/PresetCreate`,component:z,args:{sources:{...C(),errors:[]},mode:`direct`,actions:{previewChange:W(async()=>({objects:[],errors:[]})),applyChange:W(async()=>({ok:!0,mode:`direct`}))}}},J={name:`Empty preset`},Y={name:`Copy of an Elasticsearch preset`,play:async({canvas:e})=>{await G.click(e.getByRole(`radio`,{name:`Elasticsearch`})),await G.selectOptions(e.getByRole(`combobox`,{name:`Copy from`}),`standard`);let t=e.getByRole(`region`,{name:`Elasticsearch settings`});await U(t).toBeVisible();let n=K(t).getByRole(`group`,{name:`replicas`});await U(K(n).getByRole(`spinbutton`)).toHaveValue(3)}},X={name:`Changes off`,args:{mode:void 0}},Z=[`EmptyPreset`,`CopyOfAnElasticsearchPreset`,`ChangesOff`],J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "Empty preset"
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "Copy of an Elasticsearch preset",
  play: async ({
    canvas
  }) => {
    await userEvent.click(canvas.getByRole("radio", {
      name: "Elasticsearch"
    }));
    await userEvent.selectOptions(canvas.getByRole("combobox", {
      name: "Copy from"
    }), "standard");
    const settings = canvas.getByRole("region", {
      name: "Elasticsearch settings"
    });
    await expect(settings).toBeVisible();
    const replicas = within(settings).getByRole("group", {
      name: "replicas"
    });
    await expect(within(replicas).getByRole("spinbutton")).toHaveValue(3);
  }
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "Changes off",
  args: {
    mode: undefined
  }
}`,...X.parameters?.docs?.source}}}})))()}Q();export{X as ChangesOff,Y as CopyOfAnElasticsearchPreset,J as EmptyPreset,Z as __namedExportsOrder,q as default};