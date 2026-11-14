import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./scheduling-editor-BMLFXvaz.js";var r,i,a,o,s;function c(){return(c=e((()=>{t(),{fn:r}=__STORYBOOK_MODULE_TEST__,i={title:`fleet/SchedulingEditor`,component:n,args:{label:`Tolerations`,onChange:r()}},a={name:`Tolerations`,args:{defaultValue:[{key:`dedicated`,operator:`Equal`,value:`camunda`,effect:`NoSchedule`},{key:`node.kubernetes.io/unreachable`,operator:`Exists`,effect:`NoExecute`,tolerationSeconds:300}]}},o={name:`Empty`},s=[`Tolerations`,`Empty`],a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "Tolerations",
  args: {
    defaultValue: [{
      key: "dedicated",
      operator: "Equal",
      value: "camunda",
      effect: "NoSchedule"
    }, {
      key: "node.kubernetes.io/unreachable",
      operator: "Exists",
      effect: "NoExecute",
      tolerationSeconds: 300
    }]
  }
}`,...a.parameters?.docs?.source}}},o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "Empty"
}`,...o.parameters?.docs?.source}}}})))()}c();export{o as Empty,a as Tolerations,s as __namedExportsOrder,i as default};