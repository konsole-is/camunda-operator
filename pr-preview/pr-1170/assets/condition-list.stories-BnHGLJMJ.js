import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./condition-list-D-wPDCQP.js";var i,a,o,s,c;function l(){return(l=e((()=>{i=t(),n(),a={title:`fleet/ConditionList`,component:r,args:{now:`2026-10-15T12:00:00Z`,label:`Optimize conditions`,conditions:[]},decorators:[e=>(0,i.jsx)(`div`,{className:`max-w-80`,children:(0,i.jsx)(e,{})})]},o={name:`One app degraded`,args:{conditions:[{type:`Ready`,status:`False`,reason:`Degraded`,message:`The importer has 0 of 1 pods ready.`,lastTransitionTime:`2026-10-15T11:48:00Z`},{type:`WebappReady`,status:`True`,reason:`Healthy`,message:``,lastTransitionTime:`2026-10-14T09:00:00Z`},{type:`ImporterReady`,status:`False`,reason:`Degraded`,message:`0 of 1 pods ready`,lastTransitionTime:`2026-10-15T11:48:00Z`}]}},s={name:`No conditions`},c=[`OneAppDegraded`,`NoConditions`],o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "One app degraded",
  args: {
    conditions: [{
      type: "Ready",
      status: "False",
      reason: "Degraded",
      message: "The importer has 0 of 1 pods ready.",
      lastTransitionTime: "2026-10-15T11:48:00Z"
    }, {
      type: "WebappReady",
      status: "True",
      reason: "Healthy",
      message: "",
      lastTransitionTime: "2026-10-14T09:00:00Z"
    }, {
      type: "ImporterReady",
      status: "False",
      reason: "Degraded",
      message: "0 of 1 pods ready",
      lastTransitionTime: "2026-10-15T11:48:00Z"
    }]
  }
}`,...o.parameters?.docs?.source}}},s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "No conditions"
}`,...s.parameters?.docs?.source}}}})))()}l();export{s as NoConditions,o as OneAppDegraded,c as __namedExportsOrder,a as default};