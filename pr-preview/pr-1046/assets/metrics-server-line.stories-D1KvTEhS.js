import{n as e}from"./rolldown-runtime-2s6bX4vc.js";import{a as t}from"./iframe-C-WtZ9Oh.js";import{n,t as r}from"./createLucideIcon-DTc4q1oi.js";var i,a;function o(){return(o=e((()=>{n(),i={name:`circle-alert`,size:24,node:[[`circle`,{cx:`12`,cy:`12`,r:`10`,key:`1mglay`}],[`line`,{x1:`12`,x2:`12`,y1:`8`,y2:`12`,key:`1pkeuh`}],[`line`,{x1:`12`,x2:`12.01`,y1:`16`,y2:`16`,key:`4dfq90`}]],aliases:[`alert-circle`]},i.node,a=r(i)})))()}var s,c;function l(){return(l=e((()=>{n(),s={name:`circle-check`,size:24,node:[[`circle`,{cx:`12`,cy:`12`,r:`10`,key:`1mglay`}],[`path`,{d:`m16 9-5.5 5.5L8 12`,key:`xofnsj`}]],aliases:[`check-circle-2`]},s.node,c=r(s)})))()}var u,d;function f(){return(f=e((()=>{n(),u={name:`circle-slash`,size:24,node:[[`circle`,{cx:`12`,cy:`12`,r:`10`,key:`1mglay`}],[`line`,{x1:`9`,x2:`15`,y1:`15`,y2:`9`,key:`1dfufj`}]]},u.node,d=r(u)})))()}function p({state:e}){if(`error`in e)return(0,m.jsxs)(`p`,{role:`alert`,className:`flex items-start gap-2 text-sm text-bad`,children:[(0,m.jsx)(a,{"aria-hidden":!0,className:`mt-0.5 size-4 shrink-0`}),`Could not check for metrics-server: `,e.error]});let t=e.found?c:d;return(0,m.jsxs)(`p`,{className:`flex items-start gap-2 text-sm`,children:[(0,m.jsx)(t,{"aria-hidden":!0,className:`mt-0.5 size-4 shrink-0 ${e.found?`text-ok`:`text-muted`}`}),(0,m.jsxs)(`span`,{children:[(0,m.jsx)(`span`,{className:`text-text`,children:e.found?`metrics-server found`:`metrics-server not found`}),`. `,(0,m.jsx)(`span`,{className:`text-muted`,children:e.found?`Pages show live CPU and memory.`:`Pages hide the live CPU and memory.`})]})]})}var m;function h(){return(h=e((()=>{m=t(),o(),l(),f(),p.__docgenInfo={description:`MetricsServerLine says whether metrics-server serves the live CPU and
memory that the pages show, or why the check failed.`,methods:[],displayName:`MetricsServerLine`,props:{state:{required:!0,tsType:{name:`union`,raw:`{ found: boolean } | { error: string }`,elements:[{name:`signature`,type:`object`,raw:`{ found: boolean }`,signature:{properties:[{key:`found`,value:{name:`boolean`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{ error: string }`,signature:{properties:[{key:`error`,value:{name:`string`,required:!0}}]}}]},description:``}}}})))()}var g,_,v,y,b;function x(){return(x=e((()=>{h(),g={title:`setup/MetricsServerLine`,component:p,args:{state:{found:!0}}},_={name:`Found`},v={name:`Not found`,args:{state:{found:!1}}},y={name:`Check failed`,args:{state:{error:`connect ECONNREFUSED 10.96.0.1:443`}}},b=[`Found`,`NotFound`,`CheckFailed`],_.parameters={..._.parameters,docs:{..._.parameters?.docs,source:{originalSource:`{
  name: "Found"
}`,..._.parameters?.docs?.source}}},v.parameters={...v.parameters,docs:{...v.parameters?.docs,source:{originalSource:`{
  name: "Not found",
  args: {
    state: {
      found: false
    }
  }
}`,...v.parameters?.docs?.source}}},y.parameters={...y.parameters,docs:{...y.parameters?.docs,source:{originalSource:`{
  name: "Check failed",
  args: {
    state: {
      error: "connect ECONNREFUSED 10.96.0.1:443"
    }
  }
}`,...y.parameters?.docs?.source}}}})))()}x();export{y as CheckFailed,_ as Found,v as NotFound,b as __namedExportsOrder,g as default};