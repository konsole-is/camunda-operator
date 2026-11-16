import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{i as t,n}from"./pod-table-BUw5YnBB.js";var r,i,a,o,s,c,l;function u(){return(u=e((()=>{t(),r=2**30,i=[`orders-zeebe-0`,`orders-zeebe-1`,`orders-zeebe-2`].map((e,t)=>({metadata:{name:e},spec:{nodeName:`pool-a-${t+1}`},status:{phase:`Running`,conditions:[{type:`Ready`,status:t===2?`False`:`True`}],containerStatuses:[{ready:t!==2,restartCount:t*3,state:t===2?{waiting:{reason:`CrashLoopBackOff`}}:void 0}]}})),a=i.map((e,t)=>({pod:e.metadata.name,cpu:{usedCores:.3+t*.3,requestCores:.5,limitCores:1},memory:{usedBytes:(1.5+t*1.2)*r,limitBytes:4*r}})),o={title:`cluster/PodTable`,component:n,args:{pods:i,usage:a}},s={name:`With usage`},c={name:`Usage unavailable`,args:{usage:`unavailable`}},l=[`WithUsage`,`UsageUnavailable`],s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "With usage"
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Usage unavailable",
  args: {
    usage: "unavailable"
  }
}`,...c.parameters?.docs?.source}}}})))()}u();export{c as UsageUnavailable,s as WithUsage,l as __namedExportsOrder,o as default};