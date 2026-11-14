import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./metric-chart-BsApJfWc.js";var r,i,a,o,s,c;function l(){return(l=e((()=>{t(),r=Date.parse(`2026-10-15T11:00:00Z`),i=[{name:`orders-zeebe-0`,points:Array.from({length:30},(e,t)=>[r+t*12e4,400+Math.round(120*Math.sin(t/4))])},{name:`orders-zeebe-1`,points:Array.from({length:30},(e,t)=>[r+t*12e4,300+Math.round(90*Math.cos(t/5))])}],a={title:`fleet/MetricChart`,component:n,args:{title:`Exported records per second`,series:[]}},o={name:`Series`,args:{series:i}},s={name:`No data`},c=[`Series`,`NoData`],o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "Series",
  args: {
    series
  }
}`,...o.parameters?.docs?.source}}},s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "No data"
}`,...s.parameters?.docs?.source}}}})))()}l();export{s as NoData,o as Series,c as __namedExportsOrder,a as default};