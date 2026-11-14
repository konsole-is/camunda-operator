import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./change-bar-B6irPBhQ.js";var r,i,a,o,s,c,l;function u(){return(u=e((()=>{t(),{fn:r}=__STORYBOOK_MODULE_TEST__,i={title:`fleet/ChangeBar`,component:n,args:{count:1,onDiscard:r(),onReview:r()}},a={name:`One change`},o={name:`Three changes`,args:{count:3}},s={name:`Blocked by errors`,args:{count:2,errors:[{path:`spec.releaseRef`,message:`Release camunda-8-10-0 does not exist.`},{message:`The section access does not allow the registration aws-alb.`}]}},c={name:`Blocked`,args:{blocked:`The default release must be one of the allowed releases.`}},l=[`OneChange`,`ThreeChanges`,`BlockedByErrors`,`Blocked`],a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "One change"
}`,...a.parameters?.docs?.source}}},o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "Three changes",
  args: {
    count: 3
  }
}`,...o.parameters?.docs?.source}}},s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
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
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Blocked",
  args: {
    blocked: "The default release must be one of the allowed releases."
  }
}`,...c.parameters?.docs?.source}}}})))()}u();export{c as Blocked,s as BlockedByErrors,a as OneChange,o as ThreeChanges,l as __namedExportsOrder,i as default};