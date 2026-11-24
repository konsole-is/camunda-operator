import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./data-table-DztCRfRo.js";import{o as i,s as a}from"./status-B-qMFQRy.js";import{n as o,t as s}from"./status-chip-C2MWFfhS.js";import{r as c}from"./unread-Do34-4tt.js";import{n as l,t as u}from"./fixtures-C7LiHu3T.js";import{i as d,o as f,r as p}from"./model-DyJE0Dqt.js";import{t as m}from"./sources-Bzoo39de.js";function h({rows:e,errors:t,stale:n=[]}){let a=f({errors:t,stale:n});return(0,g.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,g.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em]`,children:`Platform`}),t.map(e=>(0,g.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind)),a.length>0&&(0,g.jsx)(`p`,{className:`text-sm text-muted`,children:c(a)}),(0,g.jsx)(r,{label:`Platform configs`,emptyText:e.length===0?`No platform config yet. A cluster needs one. The guide shows how to apply one.`:void 0,columns:[{key:`name`,label:`Name`,sortable:!0},{key:`method`,label:`Authentication`,sortable:!0},{key:`users`,label:`Users`,sortable:!0},{key:`status`,label:`Status`,sortable:!0}],rows:e.map(e=>({id:e.name,label:e.name,href:`/setup/platform/${encodeURIComponent(e.name)}`,search:e.name,sort:{name:e.name,method:_[e.method],users:e.users??-1,status:i(e.ready).label},cells:{name:(0,g.jsx)(`span`,{className:`font-mono`,children:e.name}),method:_[e.method],users:e.users??`Not known`,status:(0,g.jsx)(s,{condition:e.ready})}}))})]})}var g,_;function v(){return(v=e((()=>{g=t(),n(),a(),o(),p(),m(),_={basic:`Basic`,oidc:`OIDC`},h.__docgenInfo={description:`PlatformList is the list page of the platform configs. Each of errors
shows above the table, and the rows stay. stale holds the kinds whose
watch is not fresh (freshKey), as usePlatformSources
gives them.`,methods:[],displayName:`PlatformList`,props:{rows:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  method: "basic" | "oidc";
  users?: number;
  ready?: Condition;
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`method`,value:{name:`union`,raw:`"basic" | "oidc"`,elements:[{name:`literal`,value:`"basic"`},{name:`literal`,value:`"oidc"`}],required:!0}},{key:`users`,value:{name:`number`,required:!1}},{key:`ready`,value:{name:`signature`,type:`object`,raw:`{
  status: "True" | "False" | "Unknown";
  reason?: string;
  message?: string;
}`,signature:{properties:[{key:`status`,value:{name:`union`,raw:`"True" | "False" | "Unknown"`,elements:[{name:`literal`,value:`"True"`},{name:`literal`,value:`"False"`},{name:`literal`,value:`"Unknown"`}],required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]},required:!1}}]}}],raw:`PlatformRow[]`},description:``},errors:{required:!0,tsType:{name:`Array`,elements:[{name:`ReadError`}],raw:`ReadError[]`},description:``},stale:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}}}}})))()}var y,b,x,S,C,w;function T(){return(T=e((()=>{u(),p(),v(),y={title:`setup/platform/PlatformList`,component:h,args:{rows:d(l()),errors:[]}},b={name:`Configs with users, without users, and a missing Secret`},x={name:`No config`,args:{rows:[]}},S={...l(),errors:[{kind:`DatabaseServer`,message:`forbidden`}],stale:[`Environment`]},C={name:`A list that cannot be read`,args:{rows:d(S),errors:S.errors,stale:S.stale}},w=[`Configs`,`NoConfig`,`UsersNotKnown`],b.parameters={...b.parameters,docs:{...b.parameters?.docs,source:{originalSource:`{
  name: "Configs with users, without users, and a missing Secret"
}`,...b.parameters?.docs?.source}}},x.parameters={...x.parameters,docs:{...x.parameters?.docs,source:{originalSource:`{
  name: "No config",
  args: {
    rows: []
  }
}`,...x.parameters?.docs?.source}}},C.parameters={...C.parameters,docs:{...C.parameters?.docs,source:{originalSource:`{
  name: "A list that cannot be read",
  args: {
    rows: platformRows(unread),
    errors: unread.errors,
    stale: unread.stale
  }
}`,...C.parameters?.docs?.source}}}})))()}T();export{b as Configs,x as NoConfig,C as UsersNotKnown,w as __namedExportsOrder,y as default};