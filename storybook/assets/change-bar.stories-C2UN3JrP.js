import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./circle-alert-DfT8pSui.js";import{n as i,t as a}from"./button-BYOBHKht.js";function o({count:e,blocked:t,errors:n=[],reviewRef:i,onDiscard:o,onReview:c}){if(e===0)return null;let l=n.filter(e=>e.path===void 0),u=[...t?[t]:[],...n.length>0?[`Fix ${n.length} ${n.length===1?`field`:`fields`} before review.`]:[],...l.map(e=>e.message)];return(0,s.jsxs)(`section`,{"aria-label":`Unsaved changes`,className:`sticky bottom-4 z-30 flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg border border-border bg-panel px-4 py-3 shadow-lg`,children:[(0,s.jsxs)(`div`,{className:`flex min-w-0 flex-1 flex-col gap-1`,children:[(0,s.jsxs)(`p`,{className:`flex items-center gap-2 text-sm font-medium text-text`,children:[(0,s.jsx)(`span`,{"aria-hidden":!0,className:`size-2 shrink-0 rounded-full bg-info`}),e,` unsaved `,e===1?`change`:`changes`]}),u.length>0&&(0,s.jsx)(`ul`,{className:`flex flex-col gap-0.5 text-sm text-text-2`,children:u.map((e,t)=>(0,s.jsxs)(`li`,{className:`flex items-start gap-1.5`,children:[(0,s.jsx)(r,{"aria-hidden":!0,className:`mt-0.5 size-3.5 shrink-0 text-bad`}),e]},t))})]}),(0,s.jsxs)(`div`,{className:`flex shrink-0 gap-2`,children:[(0,s.jsx)(a,{variant:`ghost`,onClick:()=>o?.(),children:`Discard`}),(0,s.jsx)(a,{ref:i,disabled:!!t||n.length>0,onClick:()=>c?.(),children:`Review`})]})]})}var s;function c(){return(c=e((()=>{s=t(),n(),i(),o.__docgenInfo={description:`ChangeBar says how many changes wait for review and offers Discard and
Review. It renders nothing while count is 0. Review is disabled while
blocked is set or errors is not empty. It shows blocked and the errors
without a path; an error with a path shows next to its field. reviewRef
gets the Review button.`,methods:[],displayName:`ChangeBar`,props:{count:{required:!0,tsType:{name:`number`},description:``},blocked:{required:!1,tsType:{name:`string`},description:``},errors:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}},reviewRef:{required:!1,tsType:{name:`Ref`,elements:[{name:`HTMLButtonElement`}],raw:`Ref<HTMLButtonElement>`},description:``},onDiscard:{required:!1,tsType:{name:`signature`,type:`function`,raw:`() => void`,signature:{arguments:[],return:{name:`void`}}},description:``},onReview:{required:!1,tsType:{name:`signature`,type:`function`,raw:`() => void`,signature:{arguments:[],return:{name:`void`}}},description:``}}}})))()}var l,u,d,f,p,m,h;function g(){return(g=e((()=>{c(),{fn:l}=__STORYBOOK_MODULE_TEST__,u={title:`fleet/ChangeBar`,component:o,args:{count:1,onDiscard:l(),onReview:l()}},d={name:`One change`},f={name:`Three changes`,args:{count:3}},p={name:`Blocked by errors`,args:{count:2,errors:[{path:`spec.releaseRef`,message:`Release camunda-8-10-0 does not exist.`},{message:`The section access does not allow the registration aws-alb.`}]}},m={name:`Blocked`,args:{blocked:`The default release must be one of the allowed releases.`}},h=[`OneChange`,`ThreeChanges`,`BlockedByErrors`,`Blocked`],d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "One change"
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "Three changes",
  args: {
    count: 3
  }
}`,...f.parameters?.docs?.source}}},p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Blocked by errors",
  args: {
    count: 2,
    errors: [{
      path: "spec.releaseRef",
      message: "Release camunda-8-10-0 does not exist."
    }, {
      message: "The section access does not allow the registration aws-alb."
    }]
  }
}`,...p.parameters?.docs?.source}}},m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "Blocked",
  args: {
    blocked: "The default release must be one of the allowed releases."
  }
}`,...m.parameters?.docs?.source}}}})))()}g();export{m as Blocked,p as BlockedByErrors,d as OneChange,f as ThreeChanges,h as __namedExportsOrder,u as default};