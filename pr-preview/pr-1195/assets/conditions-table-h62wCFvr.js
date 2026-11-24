import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,o as r}from"./iframe-WoYcP6TC.js";import{a as i,n as a,s as o}from"./status-B-qMFQRy.js";import{a as s,c,i as l,n as u,o as d,s as f,t as p}from"./table-CjGQ_c5K.js";import{n as m,t as h}from"./age-f6c5OgJq.js";function g({conditions:e,groups:t=[],now:n,label:r=`Conditions`}){return e.length===0&&t.every(e=>e.conditions.length===0)?(0,v.jsx)(`p`,{className:`text-sm text-muted`,children:`No conditions`}):(0,v.jsxs)(p,{label:r,children:[(0,v.jsx)(d,{children:(0,v.jsxs)(f,{children:[(0,v.jsx)(s,{children:`Condition`}),(0,v.jsx)(s,{children:`Status`}),(0,v.jsx)(s,{children:`Reason`}),(0,v.jsx)(s,{children:`Message`}),(0,v.jsx)(s,{children:`Since last change`})]})}),e.length>0&&(0,v.jsx)(u,{children:e.map(e=>(0,v.jsx)(_,{condition:e,now:n},e.type))}),t.map(e=>(0,v.jsxs)(u,{children:[(0,v.jsx)(f,{children:(0,v.jsx)(`th`,{scope:`rowgroup`,colSpan:5,className:`px-2 pt-4 pb-1 text-left font-heading text-sm font-semibold text-text`,children:e.label})}),e.conditions.map(e=>(0,v.jsx)(_,{condition:e,now:n},e.type))]},e.label))]})}function _({condition:e,now:t}){return(0,v.jsxs)(f,{children:[(0,v.jsx)(`th`,{scope:`row`,className:`px-2 py-2 text-left font-medium whitespace-nowrap text-text`,children:e.type}),(0,v.jsx)(l,{className:n(`font-medium`,i[a[e.status]]),children:e.status}),(0,v.jsx)(l,{className:`text-text-2`,children:e.reason??`-`}),(0,v.jsx)(l,{className:`min-w-48 whitespace-normal text-text-2`,children:e.message||`-`}),(0,v.jsx)(l,{className:`text-muted`,children:h(e.lastTransitionTime,t)})]})}var v;function y(){return(y=e((()=>{v=t(),c(),r(),m(),o(),g.__docgenInfo={description:`ConditionsTable lists conditions with their type, status, reason, message,
and the time since their last change. The conditions without a group come first, then each group under
its label, in the order given. now is the time that the ages count to.`,methods:[],displayName:`ConditionsTable`,props:{conditions:{required:!0,tsType:{name:`Array`,elements:[{name:`intersection`,raw:`ConditionLike & {
  type: string;
  lastTransitionTime?: string | Date;
}`,elements:[{name:`signature`,type:`object`,raw:`{
  status: ConditionStatus;
  reason?: string;
  message?: string;
}`,signature:{properties:[{key:`status`,value:{name:`union`,raw:`"True" | "False" | "Unknown"`,elements:[{name:`literal`,value:`"True"`},{name:`literal`,value:`"False"`},{name:`literal`,value:`"Unknown"`}],required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]}},{name:`signature`,type:`object`,raw:`{
  type: string;
  lastTransitionTime?: string | Date;
}`,signature:{properties:[{key:`type`,value:{name:`string`,required:!0}},{key:`lastTransitionTime`,value:{name:`union`,raw:`string | Date`,elements:[{name:`string`},{name:`Date`}],required:!1}}]}}]}],raw:`ConditionRow[]`},description:``},groups:{required:!1,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ label: string; conditions: ConditionRow[] }`,signature:{properties:[{key:`label`,value:{name:`string`,required:!0}},{key:`conditions`,value:{name:`Array`,elements:[{name:`intersection`,raw:`ConditionLike & {
  type: string;
  lastTransitionTime?: string | Date;
}`,elements:[{name:`signature`,type:`object`,raw:`{
  status: ConditionStatus;
  reason?: string;
  message?: string;
}`,signature:{properties:[{key:`status`,value:{name:`union`,raw:`"True" | "False" | "Unknown"`,elements:[{name:`literal`,value:`"True"`},{name:`literal`,value:`"False"`},{name:`literal`,value:`"Unknown"`}],required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]}},{name:`signature`,type:`object`,raw:`{
  type: string;
  lastTransitionTime?: string | Date;
}`,signature:{properties:[{key:`type`,value:{name:`string`,required:!0}},{key:`lastTransitionTime`,value:{name:`union`,raw:`string | Date`,elements:[{name:`string`},{name:`Date`}],required:!1}}]}}]}],raw:`ConditionRow[]`,required:!0}}]}}],raw:`{ label: string; conditions: ConditionRow[] }[]`},description:``,defaultValue:{value:`[]`,computed:!1}},now:{required:!0,tsType:{name:`string`},description:``},label:{required:!1,tsType:{name:`string`},description:``,defaultValue:{value:`"Conditions"`,computed:!1}}}}})))()}export{y as n,g as t};