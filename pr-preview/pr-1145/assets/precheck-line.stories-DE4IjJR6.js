import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./precheck-line-B6ckrjkU.js";var r,i,a,o;function s(){return(s=e((()=>{t(),r={title:`fleet/PrecheckLine`,component:n},i={name:`SlotConflict`,args:{claim:{status:{conditions:[{type:`Ready`,status:`False`,reason:`SlotConflict`,message:`spec.cluster.externalUrl is filled by section "access", so the claim cannot set it`}]}}}},a={name:`With the Claim tab link`,args:{claim:{status:{conditions:[{type:`Ready`,status:`False`,reason:`WiringCycle`,message:`the inputs of the sections access and identity wait for each other`}]}},href:`/fleet/clusters/team-a/orders?tab=claim`}},o=[`SlotConflict`,`WithClaimTabLink`],i.parameters={...i.parameters,docs:{...i.parameters?.docs,source:{originalSource:`{
  name: "SlotConflict",
  args: {
    claim: {
      status: {
        conditions: [{
          type: "Ready",
          status: "False",
          reason: "SlotConflict",
          message: 'spec.cluster.externalUrl is filled by section "access", so the claim cannot set it'
        }]
      }
    }
  }
}`,...i.parameters?.docs?.source}}},a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "With the Claim tab link",
  args: {
    claim: {
      status: {
        conditions: [{
          type: "Ready",
          status: "False",
          reason: "WiringCycle",
          message: "the inputs of the sections access and identity wait for each other"
        }]
      }
    },
    href: "/fleet/clusters/team-a/orders?tab=claim"
  }
}`,...a.parameters?.docs?.source}}}})))()}s();export{i as SlotConflict,a as WithClaimTabLink,o as __namedExportsOrder,r as default};