import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{i as t,t as n}from"./session-kHBeJaik.js";import{n as r,t as i}from"./claim-review-story-DdqpT7zE.js";import{o as a,r as o}from"./fixtures-DU7FxC4G.js";var s,c,l,u,d,f,p,m;function h(){return(h=e((()=>{n(),r(),o(),s=[{field:`spec`},{field:`cluster`},{field:`zeebe`},{field:`replicas`}],c={title:`fleet/ClaimReview`,component:i,args:{claim:{...a,metadata:{...a.metadata,resourceVersion:`41`}},changes:[{steps:s,value:5}],mode:`direct`,impacts:[`Broker pods restart.`]}},l={name:`Direct`},u={name:`Git`,args:{mode:`git`}},d={name:`With caution`,args:{caution:`A backup of orders runs now. The rollout makes it fail.`}},f={name:`Preview refused`,args:{changes:[{steps:s,value:0}],impacts:[],preview:{objects:[],errors:[{code:`Invalid`,message:`Invalid value: 0: must be greater than or equal to 1`,path:`spec.cluster.zeebe.replicas`}]}}},p={name:`Stale`,args:{preview:{objects:[],errors:[{code:`Stale`,message:t}]}}},m=[`Direct`,`Git`,`WithCaution`,`PreviewRefused`,`Stale`],l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Direct"
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "Git",
  args: {
    mode: "git"
  }
}`,...u.parameters?.docs?.source}}},d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "With caution",
  args: {
    caution: "A backup of orders runs now. The rollout makes it fail."
  }
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "Preview refused",
  args: {
    changes: [{
      steps: replicas,
      value: 0
    }],
    impacts: [],
    preview: {
      objects: [],
      errors: [{
        code: "Invalid",
        message: "Invalid value: 0: must be greater than or equal to 1",
        path: "spec.cluster.zeebe.replicas"
      }]
    }
  }
}`,...f.parameters?.docs?.source}}},p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Stale",
  args: {
    preview: {
      objects: [],
      errors: [{
        code: "Stale",
        message: STALE_CLAIM_MESSAGE
      }]
    }
  }
}`,...p.parameters?.docs?.source}}}})))()}h();export{l as Direct,u as Git,f as PreviewRefused,p as Stale,d as WithCaution,m as __namedExportsOrder,c as default};