import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./data-table-COK6UJMl.js";import{n as i,t as a}from"./link-DOLMGIR2.js";import{r as o,t as s}from"./button-Zk9PyGdU.js";import{o as c,s as l}from"./status-B-qMFQRy.js";import{n as u,t as d}from"./status-chip-yZ5v6ww0.js";import{r as f}from"./unread-Do34-4tt.js";import{n as p,t as m}from"./fixtures-C7LiHu3T.js";import{a as h,i as g,s as _}from"./model-5K9uttpd.js";import{t as v}from"./sources-Bzoo39de.js";function y({rows:e,errors:t,stale:n=[]}){let a=_({errors:t,stale:n});return(0,b.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,b.jsxs)(`div`,{className:`flex flex-wrap items-center justify-between gap-3`,children:[(0,b.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em]`,children:`Platform`}),(0,b.jsx)(s,{asChild:!0,children:(0,b.jsx)(i,{href:`/setup/new-platform-config`,children:`New platform config`})})]}),t.map(e=>(0,b.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind)),a.length>0&&(0,b.jsx)(`p`,{className:`text-sm text-muted`,children:f(a)}),(0,b.jsx)(r,{label:`Platform configs`,emptyText:e.length===0?`No platform config yet. A cluster needs one.`:void 0,columns:[{key:`name`,label:`Name`,sortable:!0},{key:`method`,label:`Authentication`,sortable:!0},{key:`users`,label:`Users`,sortable:!0},{key:`status`,label:`Status`,sortable:!0}],rows:e.map(e=>({id:e.name,label:e.name,href:`/setup/platform/${encodeURIComponent(e.name)}`,search:e.name,sort:{name:e.name,method:x[e.method],users:e.users??-1,status:c(e.ready).label},cells:{name:(0,b.jsx)(`span`,{className:`font-mono`,children:e.name}),method:x[e.method],users:e.users??`Not known`,status:(0,b.jsx)(d,{condition:e.ready})}}))})]})}var b,x;function S(){return(S=e((()=>{b=t(),a(),n(),l(),u(),o(),g(),v(),x={basic:`Basic`,oidc:`OIDC`},y.__docgenInfo={description:`PlatformList is the list page of the platform configs. Each of errors
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
}`,signature:{properties:[{key:`status`,value:{name:`union`,raw:`"True" | "False" | "Unknown"`,elements:[{name:`literal`,value:`"True"`},{name:`literal`,value:`"False"`},{name:`literal`,value:`"Unknown"`}],required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]},required:!1}}]}}],raw:`PlatformRow[]`},description:``},errors:{required:!0,tsType:{name:`Array`,elements:[{name:`ReadError`}],raw:`ReadError[]`},description:``},stale:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}}}}})))()}var C,w,T,E,D,O;function k(){return(k=e((()=>{m(),g(),S(),C={title:`setup/platform/PlatformList`,component:y,args:{rows:h(p()),errors:[]}},w={name:`Configs with users, without users, and a missing Secret`},T={name:`No config`,args:{rows:[]}},E={...p(),errors:[{kind:`DatabaseServer`,message:`forbidden`}],stale:[`Environment`]},D={name:`A list that cannot be read`,args:{rows:h(E),errors:E.errors,stale:E.stale}},O=[`Configs`,`NoConfig`,`UsersNotKnown`],w.parameters={...w.parameters,docs:{...w.parameters?.docs,source:{originalSource:`{
  name: "Configs with users, without users, and a missing Secret"
}`,...w.parameters?.docs?.source}}},T.parameters={...T.parameters,docs:{...T.parameters?.docs,source:{originalSource:`{
  name: "No config",
  args: {
    rows: []
  }
}`,...T.parameters?.docs?.source}}},D.parameters={...D.parameters,docs:{...D.parameters?.docs,source:{originalSource:`{
  name: "A list that cannot be read",
  args: {
    rows: platformRows(unread),
    errors: unread.errors,
    stale: unread.stale
  }
}`,...D.parameters?.docs?.source}}}})))()}k();export{w as Configs,T as NoConfig,D as UsersNotKnown,O as __namedExportsOrder,C as default};