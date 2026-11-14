import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./status-chip-CwiFx0YW.js";var r,i,a,o,s,c,l,u;function d(){return(d=e((()=>{t(),r={title:`fleet/StatusChip`,component:n},i={name:`Ready`,args:{condition:{status:`True`,reason:`Healthy`}}},a={name:`Not ready`,args:{condition:{status:`False`,reason:`ZeebeNotReady`,message:`1 of 3 Zeebe brokers are ready`}}},o={name:`Not ready without reason`,args:{condition:{status:`False`}}},s={name:`Unknown`,args:{condition:{status:`Unknown`,reason:`Reconciling`}}},c={name:`No status`,args:{}},l={name:`Loading`,args:{loading:!0}},u=[`Ready`,`NotReady`,`NotReadyWithoutReason`,`Unknown`,`NoStatus`,`Loading`],i.parameters={...i.parameters,docs:{...i.parameters?.docs,source:{originalSource:`{
  name: "Ready",
  args: {
    condition: {
      status: "True",
      reason: "Healthy"
    }
  }
}`,...i.parameters?.docs?.source}}},a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "Not ready",
  args: {
    condition: {
      status: "False",
      reason: "ZeebeNotReady",
      message: "1 of 3 Zeebe brokers are ready"
    }
  }
}`,...a.parameters?.docs?.source}}},o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "Not ready without reason",
  args: {
    condition: {
      status: "False"
    }
  }
}`,...o.parameters?.docs?.source}}},s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Unknown",
  args: {
    condition: {
      status: "Unknown",
      reason: "Reconciling"
    }
  }
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "No status",
  args: {}
}`,...c.parameters?.docs?.source}}},l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Loading",
  args: {
    loading: true
  }
}`,...l.parameters?.docs?.source}}}})))()}d();export{l as Loading,c as NoStatus,a as NotReady,o as NotReadyWithoutReason,i as Ready,s as Unknown,u as __namedExportsOrder,r as default};