import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{n as r,t as i}from"./take-over-choice-DsFAxZ0B.js";function a(e){let[t,n]=(0,s.useState)(e.chosen);return(0,o.jsx)(`div`,{className:`max-w-2xl`,children:(0,o.jsx)(i,{...e,chosen:t,onChoose:t=>{e.onChoose(t),n(t)}})})}var o,s,c,l,u,d,f,p,m,h,g,_,v,y,b,x,S,C;function w(){return(w=e((()=>{o=n(),s=t(),r(),{fn:c}=__STORYBOOK_MODULE_TEST__,l={apiVersion:`fleet.camunda.io/v1alpha1`,kind:`Environment`,name:`staging`},u=e=>({apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,namespace:`team-a`,name:e}),d={name:`kubectl-client-side-apply`,operation:`Update`,kind:`other`,known:!0},f={name:`argocd-controller`,operation:`Apply`,kind:`gitops`,known:!0},p={name:`CamundaClusterClaim/cluster/0b5c7a2e-1f0e-4f43-9d0a-6a0c2f1e9b11`,operation:`Apply`,kind:`operator`,known:!0},m=(e,...t)=>({path:e,steps:e.split(`.`).map(e=>({field:e})),managers:t}),h={title:`fleet/TakeOverChoice`,component:i,args:{ownership:[{ref:l,changed:[m(`spec.displayName`,d)],cleared:[]}],chosen:!1,onChoose:c()},render:e=>(0,o.jsx)(a,{...e},JSON.stringify(e))},g={name:`Changed field`},_={name:`Cleared fields`,args:{ownership:[{ref:l,changed:[],cleared:[m(`spec.color`,d),m(`spec.sections.backup-storage.required`,d)]}]}},v={name:`GitOps tool`,args:{ownership:[{ref:l,changed:[m(`spec.displayName`,f)],cleared:[m(`spec.color`,f)]}]}},y={name:`Operator field`,args:{ownership:[{ref:u(`orders`),changed:[m(`spec.releaseRef`,p)],cleared:[]}]}},b={name:`A batch of two claims`,args:{ownership:[{ref:u(`orders`),changed:[m(`spec.releaseRef`,d)],cleared:[]},{ref:u(`billing`),changed:[m(`spec.releaseRef`,{name:`my-script`,operation:`Apply`,kind:`other`,known:!1})],cleared:[]}]}},x={name:`Several managers on one field`,args:{ownership:[{ref:l,changed:[m(`spec.displayName`,d,{name:`kubectl-edit`,operation:`Update`,kind:`other`,known:!0})],cleared:[]}]}},S={name:`A path that names two fields`,args:{ownership:[{ref:l,changed:[m(`spec.a.b`,d)],cleared:[{...m(`spec.a.b`,d),steps:[{field:`spec`},{field:`a.b`}]}]}]}},C=[`Changed`,`Cleared`,`GitOps`,`Operator`,`Batch`,`SeveralManagers`,`AmbiguousPath`],g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "Changed field"
}`,...g.parameters?.docs?.source}}},_.parameters={..._.parameters,docs:{..._.parameters?.docs,source:{originalSource:`{
  name: "Cleared fields",
  args: {
    ownership: [{
      ref: environment,
      changed: [],
      cleared: [field("spec.color", kubectl), field("spec.sections.backup-storage.required", kubectl)]
    }]
  }
}`,..._.parameters?.docs?.source}}},v.parameters={...v.parameters,docs:{...v.parameters?.docs,source:{originalSource:`{
  name: "GitOps tool",
  args: {
    ownership: [{
      ref: environment,
      changed: [field("spec.displayName", argo)],
      cleared: [field("spec.color", argo)]
    }]
  }
}`,...v.parameters?.docs?.source}}},y.parameters={...y.parameters,docs:{...y.parameters?.docs,source:{originalSource:`{
  name: "Operator field",
  args: {
    ownership: [{
      ref: claim("orders"),
      changed: [field("spec.releaseRef", controller)],
      cleared: []
    }]
  }
}`,...y.parameters?.docs?.source}}},b.parameters={...b.parameters,docs:{...b.parameters?.docs,source:{originalSource:`{
  name: "A batch of two claims",
  args: {
    ownership: [{
      ref: claim("orders"),
      changed: [field("spec.releaseRef", kubectl)],
      cleared: []
    }, {
      ref: claim("billing"),
      changed: [field("spec.releaseRef", {
        name: "my-script",
        operation: "Apply",
        kind: "other",
        known: false
      })],
      cleared: []
    }]
  }
}`,...b.parameters?.docs?.source}}},x.parameters={...x.parameters,docs:{...x.parameters?.docs,source:{originalSource:`{
  name: "Several managers on one field",
  args: {
    ownership: [{
      ref: environment,
      changed: [field("spec.displayName", kubectl, {
        name: "kubectl-edit",
        operation: "Update",
        kind: "other",
        known: true
      })],
      cleared: []
    }]
  }
}`,...x.parameters?.docs?.source}}},S.parameters={...S.parameters,docs:{...S.parameters?.docs,source:{originalSource:`{
  name: "A path that names two fields",
  args: {
    ownership: [{
      ref: environment,
      changed: [field("spec.a.b", kubectl)],
      cleared: [{
        ...field("spec.a.b", kubectl),
        steps: [{
          field: "spec"
        }, {
          field: "a.b"
        }]
      }]
    }]
  }
}`,...S.parameters?.docs?.source}}}})))()}w();export{S as AmbiguousPath,b as Batch,g as Changed,_ as Cleared,v as GitOps,y as Operator,x as SeveralManagers,C as __namedExportsOrder,h as default};