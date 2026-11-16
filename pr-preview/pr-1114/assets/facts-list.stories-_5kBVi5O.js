import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{c as t,f as n,l as r,n as i,p as a,r as o}from"./rows-BweABuUg.js";import{n as s,t as c}from"./fixtures-BWj48hoD.js";var l,u,d,f,p;function m(){return(m=e((()=>{a(),c(),t(),i(),l=s(),u={title:`setup/presets/FactsList`,component:n,args:{facts:o(`CamundaClusterPreset`,r(l.clusterPresets[0]))}},d={name:`Cluster preset`},f={name:`PostgreSQL preset`,args:{facts:o(`DatabaseServerPreset`,r(l.databasePresets[1]))}},p=[`ClusterPreset`,`PostgreSqlPreset`],d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Cluster preset"
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "PostgreSQL preset",
  args: {
    facts: presetFacts("DatabaseServerPreset", presetBody(sources.databasePresets[1]))
  }
}`,...f.parameters?.docs?.source}}}})))()}m();export{d as ClusterPreset,f as PostgreSqlPreset,p as __namedExportsOrder,u as default};