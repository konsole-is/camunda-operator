import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./yaml-editor-Cc7fxD5T.js";var r,i,a,o,s;function c(){return(c=e((()=>{t(),{fn:r}=__STORYBOOK_MODULE_TEST__,i={title:`fleet/YamlEditor`,component:n,args:{label:`Extra environment sources`,onChange:r()}},a={name:`Node affinity`,args:{label:`Node affinity`,defaultValue:{requiredDuringSchedulingIgnoredDuringExecution:{nodeSelectorTerms:[{matchExpressions:[{key:`pool`,operator:`In`,values:[`camunda`]}]}]}}}},o={name:`Empty`},s=[`NodeAffinity`,`Empty`],a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "Node affinity",
  args: {
    label: "Node affinity",
    defaultValue: {
      requiredDuringSchedulingIgnoredDuringExecution: {
        nodeSelectorTerms: [{
          matchExpressions: [{
            key: "pool",
            operator: "In",
            values: ["camunda"]
          }]
        }]
      }
    }
  }
}`,...a.parameters?.docs?.source}}},o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "Empty"
}`,...o.parameters?.docs?.source}}}})))()}c();export{o as Empty,a as NodeAffinity,s as __namedExportsOrder,i as default};