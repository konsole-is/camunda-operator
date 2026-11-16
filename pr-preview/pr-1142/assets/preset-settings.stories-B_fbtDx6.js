import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./fixtures-CJZMGr6g.js";import{n as r,r as i}from"./preset-settings-Bo9gWvEr.js";var a,o,s,c,l,u;function d(){return(d=e((()=>{n(),i(),a=t(),o={title:`setup/presets/PresetSettings`,component:r,args:{kind:`CamundaClusterPreset`,body:a.clusterPresets[0].spec.cluster}},s={name:`Cluster preset`},c={name:`Elasticsearch preset`,args:{kind:`ElasticsearchClusterPreset`,body:a.elasticsearchPresets[0].spec.cluster}},l={name:`PostgreSQL preset`,args:{kind:`DatabaseServerPreset`,body:a.databasePresets[1].spec.server}},u=[`ClusterPreset`,`ElasticsearchPreset`,`PostgreSQLPreset`],s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Cluster preset"
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Elasticsearch preset",
  args: {
    kind: "ElasticsearchClusterPreset",
    body: sources.elasticsearchPresets[0].spec.cluster
  }
}`,...c.parameters?.docs?.source}}},l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "PostgreSQL preset",
  args: {
    kind: "DatabaseServerPreset",
    body: sources.databasePresets[1].spec.server
  }
}`,...l.parameters?.docs?.source}}}})))()}d();export{s as ClusterPreset,c as ElasticsearchPreset,l as PostgreSQLPreset,u as __namedExportsOrder,o as default};