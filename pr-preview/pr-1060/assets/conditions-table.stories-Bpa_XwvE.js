import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./conditions-table-spU3FKIy.js";var r,i,a,o,s;function c(){return(c=e((()=>{t(),r={title:`fleet/ConditionsTable`,component:n,args:{now:`2026-10-15T12:00:00Z`,conditions:[]}},i={name:`Not ready condition`,args:{conditions:[{type:`AdminSecretReady`,status:`False`,reason:`SecretMissing`,message:`The Secret orders-admin does not exist.`,lastTransitionTime:`2026-10-15T11:48:00Z`},{type:`MirroredSecretsReady`,status:`True`,reason:`Healthy`,message:``,lastTransitionTime:`2026-10-14T09:00:00Z`}]}},a={name:`Groups`,args:{conditions:[{type:`MirroredSecretsReady`,status:`True`,reason:`Healthy`,lastTransitionTime:`2026-10-14T09:00:00Z`}],groups:[{label:`Claim`,conditions:[{type:`RequestsReady`,status:`True`,reason:`Healthy`,lastTransitionTime:`2026-10-15T11:00:00Z`},{type:`ClusterReady`,status:`False`,reason:`Converging`,message:`Zeebe has 1 of 3 brokers ready.`,lastTransitionTime:`2026-10-15T11:58:00Z`},{type:`AttachmentsReady`,status:`Unknown`,reason:`Waiting`,lastTransitionTime:`2026-10-15T11:58:00Z`}]}]}},o={name:`Empty`},s=[`NotReadyCondition`,`Groups`,`Empty`],i.parameters={...i.parameters,docs:{...i.parameters?.docs,source:{originalSource:`{
  name: "Not ready condition",
  args: {
    conditions: [{
      type: "AdminSecretReady",
      status: "False",
      reason: "SecretMissing",
      message: "The Secret orders-admin does not exist.",
      lastTransitionTime: "2026-10-15T11:48:00Z"
    }, {
      type: "MirroredSecretsReady",
      status: "True",
      reason: "Healthy",
      message: "",
      lastTransitionTime: "2026-10-14T09:00:00Z"
    }]
  }
}`,...i.parameters?.docs?.source}}},a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "Groups",
  args: {
    conditions: [{
      type: "MirroredSecretsReady",
      status: "True",
      reason: "Healthy",
      lastTransitionTime: "2026-10-14T09:00:00Z"
    }],
    groups: [{
      label: "Claim",
      conditions: [{
        type: "RequestsReady",
        status: "True",
        reason: "Healthy",
        lastTransitionTime: "2026-10-15T11:00:00Z"
      }, {
        type: "ClusterReady",
        status: "False",
        reason: "Converging",
        message: "Zeebe has 1 of 3 brokers ready.",
        lastTransitionTime: "2026-10-15T11:58:00Z"
      }, {
        type: "AttachmentsReady",
        status: "Unknown",
        reason: "Waiting",
        lastTransitionTime: "2026-10-15T11:58:00Z"
      }]
    }]
  }
}`,...a.parameters?.docs?.source}}},o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "Empty"
}`,...o.parameters?.docs?.source}}}})))()}c();export{o as Empty,a as Groups,i as NotReadyCondition,s as __namedExportsOrder,r as default};