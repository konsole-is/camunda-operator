import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./status-dot-__1o5J_N.js";var r,i,a,o,s,c,l;function u(){return(u=e((()=>{t(),r={title:`fleet/StatusDot`,component:n,args:{label:`Zeebe`}},i={name:`Ready`,args:{condition:{status:`True`}}},a={name:`Not ready`,args:{condition:{status:`False`,reason:`PodsNotReady`}}},o={name:`Unknown`,args:{condition:{status:`Unknown`}}},s={name:`No status`},c={name:`Not used`,args:{label:`Backups`,off:!0}},l=[`Ready`,`NotReady`,`Unknown`,`NoStatus`,`NotUsed`],i.parameters={...i.parameters,docs:{...i.parameters?.docs,source:{originalSource:`{
  name: "Ready",
  args: {
    condition: {
      status: "True"
    }
  }
}`,...i.parameters?.docs?.source}}},a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "Not ready",
  args: {
    condition: {
      status: "False",
      reason: "PodsNotReady"
    }
  }
}`,...a.parameters?.docs?.source}}},o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "Unknown",
  args: {
    condition: {
      status: "Unknown"
    }
  }
}`,...o.parameters?.docs?.source}}},s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "No status"
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Not used",
  args: {
    label: "Backups",
    off: true
  }
}`,...c.parameters?.docs?.source}}}})))()}u();export{s as NoStatus,a as NotReady,c as NotUsed,i as Ready,o as Unknown,l as __namedExportsOrder,r as default};