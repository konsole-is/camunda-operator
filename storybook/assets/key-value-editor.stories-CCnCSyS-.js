import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./key-value-editor-DeU3ueoJ.js";var r,i,a,o,s,c,l,u;function d(){return(d=e((()=>{t(),{fn:r}=__STORYBOOK_MODULE_TEST__,i={title:`fleet/KeyValueEditor`,component:n,args:{label:`Annotations`,onChange:r()}},a={name:`Labels`,args:{label:`Pod labels`,format:`label`,defaultValue:{team:`payments`,tier:`gold`}}},o={name:`Empty`},s={name:`No entries over an inherited value`,args:{label:`Pod labels`,format:`label`,emptyMap:`inherited`}},c={name:`Empty map over an inherited value`,args:{label:`Pod labels`,format:`label`,emptyMap:`inherited`,defaultValue:{}}},l={name:`Empty map as a value`,args:{label:`Value`,emptyMap:`unset`,defaultValue:{}}},u=[`Labels`,`Empty`,`NoEntriesOverInherited`,`EmptyMap`,`EmptyChoice`],a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "Labels",
  args: {
    label: "Pod labels",
    format: "label",
    defaultValue: {
      team: "payments",
      tier: "gold"
    }
  }
}`,...a.parameters?.docs?.source}}},o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "Empty"
}`,...o.parameters?.docs?.source}}},s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "No entries over an inherited value",
  args: {
    label: "Pod labels",
    format: "label",
    emptyMap: "inherited"
  }
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Empty map over an inherited value",
  args: {
    label: "Pod labels",
    format: "label",
    emptyMap: "inherited",
    defaultValue: {}
  }
}`,...c.parameters?.docs?.source}}},l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Empty map as a value",
  args: {
    label: "Value",
    emptyMap: "unset",
    defaultValue: {}
  }
}`,...l.parameters?.docs?.source}}}})))()}d();export{o as Empty,l as EmptyChoice,c as EmptyMap,a as Labels,s as NoEntriesOverInherited,u as __namedExportsOrder,i as default};