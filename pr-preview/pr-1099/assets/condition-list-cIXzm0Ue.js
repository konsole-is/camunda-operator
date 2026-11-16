import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./age-f6c5OgJq.js";import{n as i,t as a}from"./status-dot-BrpKHAiR.js";function o({conditions:e,label:t,now:n}){return e.length===0?(0,s.jsx)(`p`,{className:`text-sm text-muted`,children:`No conditions`}):(0,s.jsx)(`ul`,{"aria-label":t,className:`flex flex-col divide-y divide-border`,children:e.map(e=>(0,s.jsxs)(`li`,{className:`flex flex-col gap-0.5 py-2.5 first:pt-0 last:pb-0`,children:[(0,s.jsxs)(`div`,{className:`flex flex-wrap items-center justify-between gap-x-3 gap-y-0.5`,children:[(0,s.jsx)(`span`,{className:`font-mono text-xs text-text`,children:e.type}),(0,s.jsxs)(`span`,{className:`flex items-center gap-1.5 text-xs text-text-2`,children:[(0,s.jsx)(a,{label:e.type,condition:e}),e.reason]})]}),e.message&&(0,s.jsx)(`p`,{className:`text-xs break-words text-muted`,children:e.message}),e.lastTransitionTime&&(0,s.jsxs)(`p`,{className:`text-xs text-muted`,children:[`Changed `,r(e.lastTransitionTime,n),` ago`]})]},e.type))})}var s;function c(){return(c=e((()=>{s=t(),n(),i(),o.__docgenInfo={description:`ConditionList lists conditions in the order given, for a narrow column:
the type, a dot of the state with the reason, and under them the message
and the time since the last change, as far as the condition has them.
label names the list. now is the time that the ages count to.`,methods:[],displayName:`ConditionList`,props:{conditions:{required:!0,tsType:{name:`Array`,elements:[{name:`intersection`,raw:`ConditionLike & {
  type: string;
  lastTransitionTime?: string | Date;
}`,elements:[{name:`signature`,type:`object`,raw:`{
  status: ConditionStatus;
  reason?: string;
  message?: string;
}`,signature:{properties:[{key:`status`,value:{name:`union`,raw:`"True" | "False" | "Unknown"`,elements:[{name:`literal`,value:`"True"`},{name:`literal`,value:`"False"`},{name:`literal`,value:`"Unknown"`}],required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]}},{name:`signature`,type:`object`,raw:`{
  type: string;
  lastTransitionTime?: string | Date;
}`,signature:{properties:[{key:`type`,value:{name:`string`,required:!0}},{key:`lastTransitionTime`,value:{name:`union`,raw:`string | Date`,elements:[{name:`string`},{name:`Date`}],required:!1}}]}}]}],raw:`ConditionRow[]`},description:``},label:{required:!0,tsType:{name:`string`},description:``},now:{required:!0,tsType:{name:`string`},description:``}}}})))()}export{c as n,o as t};