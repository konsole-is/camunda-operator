import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{r as t,t as n}from"./pod-table-CnXWdE2G.js";var r,i,a,o,s,c,l;function u(){return(u=e((()=>{t(),r=2**30,i=[`orders-zeebe-0`,`orders-zeebe-1`,`orders-zeebe-2`].map((e,t)=>({metadata:{name:e,creationTimestamp:`2026-10-15T09:00:00Z`},status:{phase:`Running`,containerStatuses:[{ready:t!==2,restartCount:t}]}})),a=i.map((e,t)=>({pod:e.metadata.name,cpu:{usedCores:.3+t*.3,requestCores:.5,limitCores:1},memory:{usedBytes:(1.5+t*1.2)*r,limitBytes:4*r}})),o={title:`fleet/PodTable`,component:n,args:{pods:i,usage:a,now:`2026-10-15T12:00:00Z`}},s={name:`With usage`},c={name:`Usage unavailable`,args:{usage:`unavailable`}},l=[`WithUsage`,`UsageUnavailable`],s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "With usage"
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Usage unavailable",
  args: {
    usage: "unavailable"
  }
}`,...c.parameters?.docs?.source}}}})))()}u();export{c as UsageUnavailable,s as WithUsage,l as __namedExportsOrder,o as default};