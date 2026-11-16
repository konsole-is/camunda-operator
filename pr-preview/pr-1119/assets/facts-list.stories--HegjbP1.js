import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./facts-list-etzU6myW.js";import{n as r,t as i}from"./fixtures-CJZMGr6g.js";import{i as a,r as o}from"./kinds-PprTpInh.js";import{n as s,r as c}from"./rows-DXkgZm4W.js";var l,u,d,f,p;function m(){return(m=e((()=>{t(),i(),o(),s(),l=r(),u={title:`setup/presets/FactsList`,component:n,args:{facts:c(`CamundaClusterPreset`,a(l.clusterPresets[0]))}},d={name:`Cluster preset`},f={name:`PostgreSQL preset`,args:{facts:c(`DatabaseServerPreset`,a(l.databasePresets[1]))}},p=[`ClusterPreset`,`PostgreSqlPreset`],d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Cluster preset"
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "PostgreSQL preset",
  args: {
    facts: presetFacts("DatabaseServerPreset", presetBody(sources.databasePresets[1]))
  }
}`,...f.parameters?.docs?.source}}}})))()}m();export{d as ClusterPreset,f as PostgreSqlPreset,p as __namedExportsOrder,u as default};