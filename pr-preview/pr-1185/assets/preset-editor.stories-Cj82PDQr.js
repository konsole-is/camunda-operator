import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./fixtures-CF8JYqCq.js";import{r,t as i}from"./draft-0ayVOUrY.js";import{n as a,t as o}from"./preset-editor-BZzBBZ3l.js";var s,c,l,u,d,f,p,m,h,g,_,v;function y(){return(y=e((()=>{i(),n(),a(),{fn:s}=__STORYBOOK_MODULE_TEST__,c=t(),l=c.clusterPresets[0],u=c.elasticsearchPresets[0],d=c.databasePresets[1],f={title:`setup/presets/PresetEditor`,component:o,args:{kind:`CamundaClusterPreset`,draft:r(l),live:l.spec.cluster,secrets:{},secretsUnread:[],onDraft:s(),onErrors:s()}},p={name:`Cluster preset`},m={name:`Elasticsearch preset as YAML`,args:{kind:`ElasticsearchClusterPreset`,draft:r(u),live:u.spec.cluster}},h={name:`PostgreSQL preset as YAML`,args:{kind:`DatabaseServerPreset`,draft:r(d),live:d.spec.server}},g={name:`A missing Secret key`,args:{kind:`ElasticsearchClusterPreset`,draft:{description:``,body:{...u.spec.cluster,extraEnv:[{name:`S3_SECRET`,valueFrom:{secretKeyRef:{name:`snapshot-credentials`,key:`secret`}}}]}},secrets:{"team-a":[{name:`snapshot-credentials`,keys:[`secret`]}],"team-b":[{name:`snapshot-credentials`,keys:[`access`]}]}}},_={name:`Secrets not known`,args:{kind:`ElasticsearchClusterPreset`,draft:r(u),live:u.spec.cluster,secretsUnread:[`ElasticsearchCluster`]}},v=[`ClusterPreset`,`ElasticsearchPresetAsYAML`,`PostgreSQLPresetAsYAML`,`AMissingSecretKey`,`SecretsNotKnown`],p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Cluster preset"
}`,...p.parameters?.docs?.source}}},m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "Elasticsearch preset as YAML",
  args: {
    kind: "ElasticsearchClusterPreset",
    draft: presetDraftOf(es),
    live: es.spec.cluster
  }
}`,...m.parameters?.docs?.source}}},h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
  name: "PostgreSQL preset as YAML",
  args: {
    kind: "DatabaseServerPreset",
    draft: presetDraftOf(pg),
    live: pg.spec.server
  }
}`,...h.parameters?.docs?.source}}},g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "A missing Secret key",
  args: {
    kind: "ElasticsearchClusterPreset",
    draft: {
      description: "",
      body: {
        ...es.spec.cluster,
        extraEnv: [{
          name: "S3_SECRET",
          valueFrom: {
            secretKeyRef: {
              name: "snapshot-credentials",
              key: "secret"
            }
          }
        }]
      }
    },
    secrets: {
      "team-a": [{
        name: "snapshot-credentials",
        keys: ["secret"]
      }],
      "team-b": [{
        name: "snapshot-credentials",
        keys: ["access"]
      }]
    }
  }
}`,...g.parameters?.docs?.source}}},_.parameters={..._.parameters,docs:{..._.parameters?.docs,source:{originalSource:`{
  name: "Secrets not known",
  args: {
    kind: "ElasticsearchClusterPreset",
    draft: presetDraftOf(es),
    live: es.spec.cluster,
    secretsUnread: ["ElasticsearchCluster"]
  }
}`,..._.parameters?.docs?.source}}}})))()}y();export{g as AMissingSecretKey,p as ClusterPreset,m as ElasticsearchPresetAsYAML,h as PostgreSQLPresetAsYAML,_ as SecretsNotKnown,v as __namedExportsOrder,f as default};