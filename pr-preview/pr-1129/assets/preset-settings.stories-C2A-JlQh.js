import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./fixtures-BWj48hoD.js";import{n as r,t as i}from"./preset-settings-DF7MDbIn.js";var a,o,s,c,l,u;function d(){return(d=e((()=>{n(),r(),a=t(),o={title:`setup/presets/PresetSettings`,component:i,args:{kind:`CamundaClusterPreset`,body:a.clusterPresets[0].spec.cluster}},s={name:`Cluster preset`},c={name:`Elasticsearch preset`,args:{kind:`ElasticsearchClusterPreset`,body:a.elasticsearchPresets[0].spec.cluster}},l={name:`PostgreSQL preset`,args:{kind:`DatabaseServerPreset`,body:a.databasePresets[1].spec.server}},u=[`ClusterPreset`,`ElasticsearchPreset`,`PostgreSQLPreset`],s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
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