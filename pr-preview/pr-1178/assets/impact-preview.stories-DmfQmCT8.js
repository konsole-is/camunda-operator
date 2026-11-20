import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./fixtures-CF8JYqCq.js";import{a as r,i,n as a,t as o}from"./impact-preview-BQIhq_Jr.js";function s(e){let t=structuredClone(l.spec);return e(t.cluster),t}var c,l,u,d,f,p,m,h,g,_,v,y;function b(){return(b=e((()=>{n(),i(),a(),c=t(),l=c.clusterPresets[0],u=c.elasticsearchPresets[0],d=e=>r({kind:`CamundaClusterPreset`,name:`medium`,before:l.spec,after:e,sources:c}),f={title:`setup/presets/ImpactPreview`,component:o,args:{impact:d(s(e=>{e.zeebe={...e.zeebe,replicas:5}}))}},p={name:`Pods roll`},m={name:`Connectors restart`,args:{impact:r({kind:`CamundaClusterPreset`,name:`medium`,before:l.spec,after:s(e=>{e.auth={basic:{...e.auth?.basic,passwordRotation:`2026-11-15T10:42Z`}}}),sources:{...c,clusters:c.clusters.map(e=>({...e,spec:{...e.spec,connectors:{enabled:!0}}}))}})}},h={name:`Unchanged`,args:{impact:d(l.spec)}},g={name:`Pods may roll`,args:{impact:r({kind:`ElasticsearchClusterPreset`,name:`standard`,before:u.spec,after:{cluster:{...u.spec.cluster,replicas:5}},sources:{...c,elasticsearchClusters:[{apiVersion:`core.camunda.io/v1`,kind:`ElasticsearchCluster`,metadata:{namespace:`search`,name:`logs`},spec:{presetRef:`standard`}}]}})}},_={name:`The two storage rules`,args:{impact:d(s(e=>{e.zeebe={...e.zeebe,storageSize:`16Gi`,storageClassName:`gp3`}}))}},v={name:`Users that cannot be read`,args:{impact:r({kind:`CamundaClusterPreset`,name:`medium`,before:l.spec,after:l.spec,sources:c,unread:[`CamundaCluster`]})}},y=[`PodsRoll`,`ConnectorsRestart`,`Unchanged`,`PodsMayRoll`,`TheTwoStorageRules`,`UsersNotRead`],p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Pods roll"
}`,...p.parameters?.docs?.source}}},m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "Connectors restart",
  args: {
    impact: presetImpact({
      kind: "CamundaClusterPreset",
      name: "medium",
      before: medium.spec,
      after: after(c => {
        c.auth = {
          basic: {
            ...c.auth?.basic,
            passwordRotation: "2026-11-15T10:42Z"
          }
        };
      }),
      sources: {
        ...sources,
        clusters: sources.clusters.map(c => ({
          ...c,
          spec: {
            ...c.spec,
            connectors: {
              enabled: true
            }
          }
        }))
      }
    })
  }
}`,...m.parameters?.docs?.source}}},h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
  name: "Unchanged",
  args: {
    impact: cluster(medium.spec)
  }
}`,...h.parameters?.docs?.source}}},g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "Pods may roll",
  args: {
    impact: presetImpact({
      kind: "ElasticsearchClusterPreset",
      name: "standard",
      before: es.spec,
      after: {
        cluster: {
          ...es.spec.cluster,
          replicas: 5
        }
      },
      sources: {
        ...sources,
        elasticsearchClusters: [{
          apiVersion: "core.camunda.io/v1",
          kind: "ElasticsearchCluster",
          metadata: {
            namespace: "search",
            name: "logs"
          },
          spec: {
            presetRef: "standard"
          }
        } as (typeof sources.elasticsearchClusters)[number]]
      }
    })
  }
}`,...g.parameters?.docs?.source}}},_.parameters={..._.parameters,docs:{..._.parameters?.docs,source:{originalSource:`{
  name: "The two storage rules",
  args: {
    impact: cluster(after(c => {
      c.zeebe = {
        ...c.zeebe,
        storageSize: "16Gi",
        storageClassName: "gp3"
      };
    }))
  }
}`,..._.parameters?.docs?.source}}},v.parameters={...v.parameters,docs:{...v.parameters?.docs,source:{originalSource:`{
  name: "Users that cannot be read",
  args: {
    impact: presetImpact({
      kind: "CamundaClusterPreset",
      name: "medium",
      before: medium.spec,
      after: medium.spec,
      sources,
      unread: ["CamundaCluster"]
    })
  }
}`,...v.parameters?.docs?.source}}}})))()}b();export{m as ConnectorsRestart,g as PodsMayRoll,p as PodsRoll,_ as TheTwoStorageRules,h as Unchanged,v as UsersNotRead,y as __namedExportsOrder,f as default};