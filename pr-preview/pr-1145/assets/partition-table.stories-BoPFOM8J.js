import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./partition-table-CjfYBULA.js";import{n as r,r as i}from"./fixtures-Dr1JTW37.js";var a,o,s,c;function l(){return(l=e((()=>{i(),t(),a={title:`cluster/PartitionTable`,component:n},o={name:`Health from Prometheus`,args:{partitions:r}},s={name:`No Prometheus`,args:{partitions:r.map(e=>({...e,health:`unknown`}))}},c=[`HealthFromPrometheus`,`NoPrometheus`],o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
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