import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,o as n,r,s as i}from"./rows-B4jT8f7U.js";import{n as a,t as o}from"./fixtures-CF8JYqCq.js";import{i as s,r as c}from"./kinds-PprTpInh.js";var l,u,d,f,p;function m(){return(m=e((()=>{i(),o(),c(),t(),l=a(),u={title:`setup/presets/FactsList`,component:n,args:{facts:r(`CamundaClusterPreset`,s(l.clusterPresets[0]))}},d={name:`Cluster preset`},f={name:`PostgreSQL preset`,args:{facts:r(`DatabaseServerPreset`,s(l.databasePresets[1]))}},p=[`ClusterPreset`,`PostgreSqlPreset`],d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Cluster preset"
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "PostgreSQL preset",
  args: {
    facts: presetFacts("DatabaseServerPreset", presetBody(sources.databasePresets[1]))
  }
}`,...f.parameters?.docs?.source}}}})))()}m();export{d as ClusterPreset,f as PostgreSqlPreset,p as __namedExportsOrder,u as default};