import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./fixtures-CF8JYqCq.js";import{n as r,t as i}from"./rotate-action-DzKWJ0px.js";function a(e,t){return{...s.clusters[0],metadata:{namespace:e,name:t}}}var o,s,c,l,u,d,f,p,m;function h(){return(h=e((()=>{n(),r(),{fn:o}=__STORYBOOK_MODULE_TEST__,s=t(),c={...s,clusters:[s.clusters[0],a(`team-billing`,`billing`),a(`team-search`,`search`)]},l={title:`setup/presets/RotateAction`,component:i,args:{preset:s.clusterPresets[0],sources:c,unread:[],mode:`direct`,disabled:!1,actions:{previewChange:async()=>({objects:[{ref:{apiVersion:`core.camunda.io/v1`,kind:`CamundaClusterPreset`,name:`medium`},diff:[{op:`same`,text:`auth:`},{op:`same`,text:`  basic:`},{op:`added`,text:`    passwordRotation: 2026-11-15T10:42Z`}]}],errors:[]}),applyChange:async()=>({ok:!0,mode:`direct`})},show:o()}},u={name:`Three clusters`},d={name:`Git mode`,args:{mode:`git`}},f={name:`No cluster to rotate`,args:{sources:{...s,clusters:[]}}},p={name:`Clusters that cannot be read`,args:{unread:[`CamundaCluster`]}},m=[`ThreeClusters`,`GitMode`,`NoClusterToRotate`,`ClustersNotRead`],u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "Three clusters"
}`,...u.parameters?.docs?.source}}},d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Git mode",
  args: {
    mode: "git"
  }
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "No cluster to rotate",
  args: {
    sources: {
      ...sources,
      clusters: []
    }
  }
}`,...f.parameters?.docs?.source}}},p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Clusters that cannot be read",
  args: {
    unread: ["CamundaCluster"]
  }
}`,...p.parameters?.docs?.source}}}})))()}h();export{p as ClustersNotRead,d as GitMode,f as NoClusterToRotate,u as ThreeClusters,m as __namedExportsOrder,l as default};