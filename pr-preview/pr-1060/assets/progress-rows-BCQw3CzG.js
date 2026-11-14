import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./status-chip-BcRp7iaj.js";import{n as i,t as a}from"./status-dot-uJ6OSt3k.js";function o({ready:e,rows:t,done:n}){return(0,s.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-4`,children:[e&&(0,s.jsxs)(`div`,{className:`flex flex-col gap-1`,children:[(0,s.jsx)(r,{condition:e}),e.message&&(0,s.jsx)(`p`,{className:`text-sm text-text-2`,children:e.message})]}),(0,s.jsx)(`ol`,{className:`flex flex-col divide-y divide-divider rounded-md border border-border`,children:t.map(e=>(0,s.jsxs)(`li`,{className:`flex flex-col gap-1 px-3 py-2 sm:flex-row sm:items-center sm:gap-4`,children:[(0,s.jsxs)(`div`,{className:`min-w-0 flex-1`,children:[(0,s.jsx)(`p`,{className:`text-sm font-medium text-text`,children:e.label}),e.detail&&(0,s.jsx)(`p`,{className:`text-xs text-muted`,children:e.detail})]}),(0,s.jsx)(`div`,{className:`flex flex-wrap items-center gap-3`,children:e.checks.map(e=>(0,s.jsxs)(`span`,{className:`inline-flex items-center gap-1.5 text-xs text-text-2`,children:[(0,s.jsx)(a,{label:e.label,condition:e.condition}),(0,s.jsx)(`span`,{"aria-hidden":!0,children:e.label})]},e.label))}),e.summary&&(0,s.jsx)(`p`,{className:`text-xs text-muted sm:w-56`,children:e.summary})]},e.id))}),n]})}var s;function c(){return(c=e((()=>{s=t(),n(),i(),o.__docgenInfo={description:`ProgressRows shows the progress after a create: on top the Ready
condition of the claim with its message, then one row for each request
and component, with the state of each of its checks and its summary
headline. done, when given, shows under the rows, for example a link to
the new cluster.`,methods:[],displayName:`ProgressRows`,props:{ready:{required:!1,tsType:{name:`signature`,type:`object`,raw:`{
  status: ConditionStatus;
  reason?: string;
  message?: string;
}`,signature:{properties:[{key:`status`,value:{name:`union`,raw:`"True" | "False" | "Unknown"`,elements:[{name:`literal`,value:`"True"`},{name:`literal`,value:`"False"`},{name:`literal`,value:`"Unknown"`}],required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]}},description:``},rows:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  id: string;
  label: string;
  detail?: string;
  summary?: string;
  checks: { label: string; condition?: ConditionLike }[];
}`,signature:{properties:[{key:`id`,value:{name:`string`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`detail`,value:{name:`string`,required:!1}},{key:`summary`,value:{name:`string`,required:!1}},{key:`checks`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ label: string; condition?: ConditionLike }`,signature:{properties:[{key:`label`,value:{name:`string`,required:!0}},{key:`condition`,value:{name:`signature`,type:`object`,raw:`{
  status: ConditionStatus;
  reason?: string;
  message?: string;
}`,signature:{properties:[{key:`status`,value:{name:`union`,raw:`"True" | "False" | "Unknown"`,elements:[{name:`literal`,value:`"True"`},{name:`literal`,value:`"False"`},{name:`literal`,value:`"Unknown"`}],required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]},required:!1}}]}}],raw:`{ label: string; condition?: ConditionLike }[]`,required:!0}}]}}],raw:`ProgressRow[]`},description:``},done:{required:!1,tsType:{name:`ReactNode`},description:``}}}})))()}export{c as n,o as t};