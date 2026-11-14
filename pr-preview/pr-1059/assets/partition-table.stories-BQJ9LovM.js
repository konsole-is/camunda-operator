import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,r as n}from"./fixtures-Dju11n-g.js";import{n as r,t as i}from"./partition-table-DVqulr-D.js";var a,o,s,c;function l(){return(l=e((()=>{n(),r(),a={title:`cluster/PartitionTable`,component:i},o={name:`Health from Prometheus`,args:{partitions:t}},s={name:`No Prometheus`,args:{partitions:t.map(e=>({...e,health:`unknown`}))}},c=[`HealthFromPrometheus`,`NoPrometheus`],o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "Health from Prometheus",
  args: {
    partitions: degraded
  }
}`,...o.parameters?.docs?.source}}},s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "No Prometheus",
  args: {
    partitions: degraded.map(p => ({
      ...p,
      health: "unknown"
    }))
  }
}`,...s.parameters?.docs?.source}}}})))()}l();export{o as HealthFromPrometheus,s as NoPrometheus,c as __namedExportsOrder,a as default};