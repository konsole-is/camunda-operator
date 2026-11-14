import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./createLucideIcon-B55dJ-eE.js";import{n as i,t as a}from"./circle-alert-DfT8pSui.js";import{n as o,t as s}from"./circle-check-CJp-Yl_v.js";var c,l;function u(){return(u=e((()=>{n(),c={name:`circle-slash`,size:24,node:[[`circle`,{cx:`12`,cy:`12`,r:`10`,key:`1mglay`}],[`line`,{x1:`9`,x2:`15`,y1:`15`,y2:`9`,key:`1dfufj`}]]},c.node,l=r(c)})))()}function d({state:e}){if(`error`in e)return(0,f.jsxs)(`p`,{role:`alert`,className:`flex items-start gap-2 text-sm text-bad`,children:[(0,f.jsx)(a,{"aria-hidden":!0,className:`mt-0.5 size-4 shrink-0`}),`Could not check for metrics-server: `,e.error]});let t=e.found?s:l;return(0,f.jsxs)(`p`,{className:`flex items-start gap-2 text-sm`,children:[(0,f.jsx)(t,{"aria-hidden":!0,className:`mt-0.5 size-4 shrink-0 ${e.found?`text-ok`:`text-muted`}`}),(0,f.jsxs)(`span`,{children:[(0,f.jsx)(`span`,{className:`text-text`,children:e.found?`metrics-server found`:`metrics-server not found`}),`. `,(0,f.jsx)(`span`,{className:`text-muted`,children:e.found?`Pages show live CPU and memory.`:`Pages hide the live CPU and memory.`})]})]})}var f;function p(){return(p=e((()=>{f=t(),i(),o(),u(),d.__docgenInfo={description:`MetricsServerLine says whether metrics-server serves the live CPU and
memory that the pages show, or why the check failed.`,methods:[],displayName:`MetricsServerLine`,props:{state:{required:!0,tsType:{name:`union`,raw:`{ found: boolean } | { error: string }`,elements:[{name:`signature`,type:`object`,raw:`{ found: boolean }`,signature:{properties:[{key:`found`,value:{name:`boolean`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{ error: string }`,signature:{properties:[{key:`error`,value:{name:`string`,required:!0}}]}}]},description:``}}}})))()}var m,h,g,_,v;function y(){return(y=e((()=>{p(),m={title:`setup/MetricsServerLine`,component:d,args:{state:{found:!0}}},h={name:`Found`},g={name:`Not found`,args:{state:{found:!1}}},_={name:`Check failed`,args:{state:{error:`connect ECONNREFUSED 10.96.0.1:443`}}},v=[`Found`,`NotFound`,`CheckFailed`],h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
  name: "Found"
}`,...h.parameters?.docs?.source}}},g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "Not found",
  args: {
    state: {
      found: false
    }
  }
}`,...g.parameters?.docs?.source}}},_.parameters={..._.parameters,docs:{..._.parameters?.docs,source:{originalSource:`{
  name: "Check failed",
  args: {
    state: {
      error: "connect ECONNREFUSED 10.96.0.1:443"
    }
  }
}`,..._.parameters?.docs?.source}}}})))()}y();export{_ as CheckFailed,h as Found,g as NotFound,v as __namedExportsOrder,m as default};