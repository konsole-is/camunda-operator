import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./data-table-BpAD51L1.js";import{n as i,t as a}from"./link-DOLMGIR2.js";import{r as o,t as s}from"./button-CTXMyhLZ.js";import{o as c,s as l}from"./status-B-qMFQRy.js";import{n as u,t as d}from"./status-chip-BxjnwU2y.js";import{r as f}from"./unread-Do34-4tt.js";import{n as p,t as m}from"./fixtures-C7LiHu3T.js";import{a as h,l as g,o as _,s as v}from"./model-CWElwuCt.js";import{t as y}from"./sources-Bzoo39de.js";function b({rows:e,errors:t,stale:n=[]}){let a=g({errors:t,stale:n});return(0,x.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,x.jsxs)(`div`,{className:`flex flex-wrap items-center justify-between gap-3`,children:[(0,x.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em]`,children:`Platform`}),(0,x.jsx)(s,{asChild:!0,children:(0,x.jsx)(i,{href:`/setup/new-platform-config`,children:`New platform config`})})]}),t.map(e=>(0,x.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind)),a.length>0&&(0,x.jsx)(`p`,{className:`text-sm text-muted`,children:f(a)}),(0,x.jsx)(r,{label:`Platform configs`,emptyText:e.length===0?`No platform config yet. A cluster needs one.`:void 0,columns:[{key:`name`,label:`Name`,sortable:!0},{key:`method`,label:`Authentication`,sortable:!0},{key:`users`,label:`Users`,sortable:!0},{key:`status`,label:`Status`,sortable:!0}],rows:e.map(e=>({id:e.name,label:e.name,href:_(e.name),search:e.name,sort:{name:e.name,method:S[e.method],users:e.users??-1,status:c(e.ready).label},cells:{name:(0,x.jsx)(`span`,{className:`font-mono`,children:e.name}),method:S[e.method],users:e.users??`Not known`,status:(0,x.jsx)(d,{condition:e.ready})}}))})]})}var x,S;function C(){return(C=e((()=>{x=t(),a(),n(),l(),u(),o(),h(),y(),S={basic:`Basic`,oidc:`OIDC`},b.__docgenInfo={description:`PlatformList is the list page of the platform configs. Each of errors
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
}`,signature:{properties:[{key:`status`,value:{name:`union`,raw:`"True" | "False" | "Unknown"`,elements:[{name:`literal`,value:`"True"`},{name:`literal`,value:`"False"`},{name:`literal`,value:`"Unknown"`}],required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]},required:!1}}]}}],raw:`PlatformRow[]`},description:``},errors:{required:!0,tsType:{name:`Array`,elements:[{name:`ReadError`}],raw:`ReadError[]`},description:``},stale:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}}}}})))()}var w,T,E,D,O,k;function A(){return(A=e((()=>{m(),h(),C(),w={title:`setup/platform/PlatformList`,component:b,args:{rows:v(p()),errors:[]}},T={name:`Configs with users, without users, and a missing Secret`},E={name:`No config`,args:{rows:[]}},D={...p(),errors:[{kind:`DatabaseServer`,message:`forbidden`}],stale:[`Environment`]},O={name:`A list that cannot be read`,args:{rows:v(D),errors:D.errors,stale:D.stale}},k=[`Configs`,`NoConfig`,`UsersNotKnown`],T.parameters={...T.parameters,docs:{...T.parameters?.docs,source:{originalSource:`{
  name: "Configs with users, without users, and a missing Secret"
}`,...T.parameters?.docs?.source}}},E.parameters={...E.parameters,docs:{...E.parameters?.docs,source:{originalSource:`{
  name: "No config",
  args: {
    rows: []
  }
}`,...E.parameters?.docs?.source}}},O.parameters={...O.parameters,docs:{...O.parameters?.docs,source:{originalSource:`{
  name: "A list that cannot be read",
  args: {
    rows: platformRows(unread),
    errors: unread.errors,
    stale: unread.stale
  }
}`,...O.parameters?.docs?.source}}}})))()}A();export{T as Configs,E as NoConfig,O as UsersNotKnown,k as __namedExportsOrder,w as default};