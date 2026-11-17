import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./condition-badge-BlFQeKxu.js";import{i as r,t as i}from"./fixtures-fUctxLbh.js";var a,o,s,c,l,u;function d(){return(d=e((()=>{t(),r(),a={title:`setup/plugins/ConditionBadge`,component:n,args:{type:`Accepted`,condition:i(`Accepted`,`True`,`Healthy`,`All checks passed`)}},o={name:`True`},s={name:`False with a reason`,args:{condition:i(`Accepted`,`False`,`SectionMissing`,`These sections do not exist: gone`)}},c={name:`No status`,args:{type:`Ready`,condition:void 0}},l={name:`Not known`,args:{type:`Ready`,unknown:!0}},u=[`True`,`FalseWithAReason`,`NoStatus`,`NotKnown`],o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "True"
}`,...o.parameters?.docs?.source}}},s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "False with a reason",
  args: {
    condition: condition("Accepted", "False", "SectionMissing", "These sections do not exist: gone")
  }
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "No status",
  args: {
    type: "Ready",
    condition: undefined
  }
}`,...c.parameters?.docs?.source}}},l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Not known",
  args: {
    type: "Ready",
    unknown: true
  }
}`,...l.parameters?.docs?.source}}}})))()}d();export{s as FalseWithAReason,c as NoStatus,l as NotKnown,o as True,u as __namedExportsOrder,a as default};