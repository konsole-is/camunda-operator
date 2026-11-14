import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{h as t,o as n}from"./labels-a4ptf10d.js";import{a as r,n as i}from"./topology-3Ta4rKpp.js";import{n as a,r as o}from"./topology-view-DWMSBiO0.js";function s(e,t,r,i=!1){return{kind:`Pod`,metadata:{name:e,labels:{[n]:t},deletionTimestamp:i?`2026-10-18T10:00:00Z`:void 0},status:r}}var c,l,u,d,f,p,m;function h(){return(h=e((()=>{t(),i(),o(),c={phase:`Running`,conditions:[{type:`Ready`,status:`True`}]},l=e=>({phase:`Pending`,containerStatuses:[{name:`zeebe`,image:`camunda/zeebe`,imageID:``,ready:!1,restartCount:4,state:{waiting:{reason:e}}}]}),u={title:`cluster/TopologyView`,component:a},d={name:`Standalone gateway`,args:{boxes:r({},[s(`orders-zeebe-0`,`zeebe`,c),s(`orders-zeebe-1`,`zeebe`,c),s(`orders-zeebe-2`,`zeebe`,c),s(`orders-gateway-7d9f-x2`,`gateway`,c)])}},f={name:`Embedded gateway`,args:{boxes:r({gateway:{mode:`Embedded`},connectors:{enabled:!0}},[s(`orders-zeebe-0`,`zeebe`,c),s(`orders-connectors-5c-a1`,`connectors`,c)])}},p={name:`Pods in every state`,args:{boxes:r({operate:{mode:`Standalone`}},[s(`orders-zeebe-0`,`zeebe`,c),s(`orders-zeebe-1`,`zeebe`,l(`CrashLoopBackOff`)),s(`orders-zeebe-2`,`zeebe`,l(`ContainerCreating`)),s(`orders-gateway-7d9f-x2`,`gateway`,{phase:`Failed`}),s(`orders-gateway-7d9f-y3`,`gateway`,c,!0)])}},m=[`StandaloneGateway`,`EmbeddedGateway`,`PodsInEveryState`],d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Standalone gateway",
  args: {
    boxes: topologyOf({} as CamundaClusterSpec, [pod("orders-zeebe-0", "zeebe", ready), pod("orders-zeebe-1", "zeebe", ready), pod("orders-zeebe-2", "zeebe", ready), pod("orders-gateway-7d9f-x2", "gateway", ready)])
  }
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "Embedded gateway",
  args: {
    boxes: topologyOf({
      gateway: {
        mode: "Embedded"
      },
      connectors: {
        enabled: true
      }
    } as CamundaClusterSpec, [pod("orders-zeebe-0", "zeebe", ready), pod("orders-connectors-5c-a1", "connectors", ready)])
  }
}`,...f.parameters?.docs?.source}}},p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Pods in every state",
  args: {
    boxes: topologyOf({
      operate: {
        mode: "Standalone"
      }
    } as CamundaClusterSpec, [pod("orders-zeebe-0", "zeebe", ready), pod("orders-zeebe-1", "zeebe", waiting("CrashLoopBackOff")), pod("orders-zeebe-2", "zeebe", waiting("ContainerCreating")), pod("orders-gateway-7d9f-x2", "gateway", {
      phase: "Failed"
    }), pod("orders-gateway-7d9f-y3", "gateway", ready, true)])
  }
}`,...p.parameters?.docs?.source}}}})))()}h();export{f as EmbeddedGateway,p as PodsInEveryState,d as StandaloneGateway,m as __namedExportsOrder,u as default};