import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./fixtures-CF8JYqCq.js";import{o as r,r as i}from"./draft-BILk3XgT.js";import{n as a,t as o}from"./preset-editor-Df6KdPWV.js";var s,c,l,u,d,f,p,m,h,g,_,v,y,b;function x(){return(x=e((()=>{i(),n(),a(),{expect:s,fn:c,within:l}=__STORYBOOK_MODULE_TEST__,u=t(),d=u.clusterPresets[0],f=u.elasticsearchPresets[0],p=u.databasePresets[1],m={title:`setup/presets/PresetEditor`,component:o,args:{kind:`CamundaClusterPreset`,draft:r(d),live:d.spec.cluster,secrets:{},secretsUnread:[],onDraft:c(),onErrors:c()}},h={name:`Cluster preset`},g={name:`Elasticsearch preset`,args:{kind:`ElasticsearchClusterPreset`,draft:r(f),live:f.spec.cluster},play:async({canvas:e})=>{let t=e.getByRole(`region`,{name:`Elasticsearch settings`});await s(l(t).getByRole(`group`,{name:`replicas`})).toBeVisible(),await s(e.getByRole(`list`,{name:`Merge rules of Elasticsearch`})).toBeVisible()}},_={name:`PostgreSQL preset`,args:{kind:`DatabaseServerPreset`,draft:r(p),live:p.spec.server}},v={name:`A missing Secret key`,args:{kind:`ElasticsearchClusterPreset`,draft:{description:``,body:{...f.spec.cluster,extraEnv:[{name:`S3_SECRET`,valueFrom:{secretKeyRef:{name:`snapshot-credentials`,key:`secret`}}}]}},secrets:{"team-a":[{name:`snapshot-credentials`,keys:[`secret`]}],"team-b":[{name:`snapshot-credentials`,keys:[`access`]}]}}},y={name:`Secrets not known`,args:{kind:`ElasticsearchClusterPreset`,draft:r(f),live:f.spec.cluster,secretsUnread:[`ElasticsearchCluster`]}},b=[`ClusterPreset`,`ElasticsearchPreset`,`PostgreSQLPreset`,`AMissingSecretKey`,`SecretsNotKnown`],h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
  name: "Cluster preset"
}`,...h.parameters?.docs?.source}}},g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "Elasticsearch preset",
  args: {
    kind: "ElasticsearchClusterPreset",
    draft: presetDraftOf(es),
    live: es.spec.cluster
  },
  play: async ({
    canvas
  }) => {
    const settings = canvas.getByRole("region", {
      name: "Elasticsearch settings"
    });
    await expect(within(settings).getByRole("group", {
      name: "replicas"
    })).toBeVisible();
    await expect(canvas.getByRole("list", {
      name: "Merge rules of Elasticsearch"
    })).toBeVisible();
  }
}`,...g.parameters?.docs?.source}}},_.parameters={..._.parameters,docs:{..._.parameters?.docs,source:{originalSource:`{
  name: "PostgreSQL preset",
  args: {
    kind: "DatabaseServerPreset",
    draft: presetDraftOf(pg),
    live: pg.spec.server
  }
}`,..._.parameters?.docs?.source}}},v.parameters={...v.parameters,docs:{...v.parameters?.docs,source:{originalSource:`{
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
}`,...v.parameters?.docs?.source}}},y.parameters={...y.parameters,docs:{...y.parameters?.docs,source:{originalSource:`{
  name: "Secrets not known",
  args: {
    kind: "ElasticsearchClusterPreset",
    draft: presetDraftOf(es),
    live: es.spec.cluster,
    secretsUnread: ["ElasticsearchCluster"]
  }
}`,...y.parameters?.docs?.source}}}})))()}x();export{v as AMissingSecretKey,h as ClusterPreset,g as ElasticsearchPreset,_ as PostgreSQLPreset,y as SecretsNotKnown,b as __namedExportsOrder,m as default};