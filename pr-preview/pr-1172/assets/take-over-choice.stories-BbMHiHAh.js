import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{n as r,t as i}from"./take-over-choice-C3GooXAx.js";function a(e){let[t,n]=(0,s.useState)(e.chosen);return(0,o.jsx)(`div`,{className:`max-w-2xl`,children:(0,o.jsx)(i,{...e,chosen:t,onChoose:t=>{e.onChoose(t),n(t)}})})}var o,s,c,l,u,d,f,p,m,h,g,_,v,y,b,x,S,C,w,T,E,D;function O(){return(O=e((()=>{o=n(),s=t(),r(),{fn:c}=__STORYBOOK_MODULE_TEST__,l={apiVersion:`fleet.camunda.io/v1alpha1`,kind:`Environment`,name:`staging`},u=e=>({apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,namespace:`team-a`,name:e}),d={name:`kubectl-client-side-apply`,operation:`Update`,kind:`other`,known:!0},f={name:`argocd-controller`,operation:`Apply`,kind:`gitops`,known:!0},p={name:`CamundaClusterClaim/cluster/0b5c7a2e-1f0e-4f43-9d0a-6a0c2f1e9b11`,operation:`Apply`,kind:`operator`,known:!0},m={...p,name:`camunda-operator/databaseserver-recovery`},h={...p,name:`camunda-operator/pointintimerestore-recovery`},g=(e,...t)=>({path:e,steps:e.split(`.`).map(e=>({field:e})),managers:t}),_={title:`fleet/TakeOverChoice`,component:i,args:{ownership:[{ref:l,changed:[g(`spec.displayName`,d)],cleared:[]}],chosen:!1,onChoose:c()},render:e=>(0,o.jsx)(a,{...e},JSON.stringify(e))},v={name:`Changed field`},y={name:`Cleared fields`,args:{ownership:[{ref:l,changed:[],cleared:[g(`spec.color`,d),g(`spec.sections.backup-storage.required`,d)]}]}},b={name:`GitOps tool`,args:{ownership:[{ref:l,changed:[g(`spec.displayName`,f)],cleared:[g(`spec.color`,f)]}]}},x={name:`Operator field`,args:{ownership:[{ref:u(`orders`),changed:[g(`spec.releaseRef`,p)],cleared:[]}]}},S={name:`Cleared fields of the operator`,args:{ownership:[{ref:{apiVersion:`fleet.camunda.io/v1alpha1`,kind:`DatabaseServerConfig`,name:`main`},changed:[],cleared:[g(`spec.pitr.lastRecovery`,m),g(`spec.recovery`,h)]}]}},C={name:`A batch of two claims`,args:{ownership:[{ref:u(`orders`),changed:[g(`spec.releaseRef`,d)],cleared:[]},{ref:u(`billing`),changed:[g(`spec.releaseRef`,{name:`my-script`,operation:`Apply`,kind:`other`,known:!1})],cleared:[]}]}},w={name:`Several managers on one field`,args:{ownership:[{ref:l,changed:[g(`spec.displayName`,d,{name:`kubectl-edit`,operation:`Update`,kind:`other`,known:!0})],cleared:[]}]}},T={name:`A path that names two fields`,args:{ownership:[{ref:l,changed:[g(`spec.a.b`,d)],cleared:[{...g(`spec.a.b`,d),steps:[{field:`spec`},{field:`a.b`}]}]}]}},E={name:`Cleared fields after the choice`,args:{ownership:[{ref:u(`orders`),changed:[],cleared:[g(`spec.attachments.backupSchedule.retained.completed`,d),g(`spec.platformConfigRef`,d)]}],chosen:!0,taken:{objects:[{ref:u(`orders`),after:{apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,metadata:{name:`orders`,namespace:`team-a`},spec:{attachments:{backupSchedule:{retained:{completed:7}}}}},diff:[]}],errors:[]}}},D=[`Changed`,`Cleared`,`GitOps`,`Operator`,`OperatorCleared`,`Batch`,`SeveralManagers`,`AmbiguousPath`,`ClearedTaken`],v.parameters={...v.parameters,docs:{...v.parameters?.docs,source:{originalSource:`{
  name: "Changed field"
}`,...v.parameters?.docs?.source}}},y.parameters={...y.parameters,docs:{...y.parameters?.docs,source:{originalSource:`{
  name: "Cleared fields",
  args: {
    ownership: [{
      ref: environment,
      changed: [],
      cleared: [field("spec.color", kubectl), field("spec.sections.backup-storage.required", kubectl)]
    }]
  }
}`,...y.parameters?.docs?.source}}},b.parameters={...b.parameters,docs:{...b.parameters?.docs,source:{originalSource:`{
  name: "GitOps tool",
  args: {
    ownership: [{
      ref: environment,
      changed: [field("spec.displayName", argo)],
      cleared: [field("spec.color", argo)]
    }]
  }
}`,...b.parameters?.docs?.source}}},x.parameters={...x.parameters,docs:{...x.parameters?.docs,source:{originalSource:`{
  name: "Operator field",
  args: {
    ownership: [{
      ref: claim("orders"),
      changed: [field("spec.releaseRef", controller)],
      cleared: []
    }]
  }
}`,...x.parameters?.docs?.source}}},S.parameters={...S.parameters,docs:{...S.parameters?.docs,source:{originalSource:`{
  name: "Cleared fields of the operator",
  args: {
    ownership: [{
      ref: {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "DatabaseServerConfig",
        name: "main"
      },
      changed: [],
      cleared: [field("spec.pitr.lastRecovery", server), field("spec.recovery", restore)]
    }]
  }
}`,...S.parameters?.docs?.source}}},C.parameters={...C.parameters,docs:{...C.parameters?.docs,source:{originalSource:`{
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
}`,...C.parameters?.docs?.source}}},w.parameters={...w.parameters,docs:{...w.parameters?.docs,source:{originalSource:`{
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
}`,...w.parameters?.docs?.source}}},T.parameters={...T.parameters,docs:{...T.parameters?.docs,source:{originalSource:`{
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
}`,...T.parameters?.docs?.source}}},E.parameters={...E.parameters,docs:{...E.parameters?.docs,source:{originalSource:`{
  name: "Cleared fields after the choice",
  args: {
    ownership: [{
      ref: claim("orders"),
      changed: [],
      cleared: [field("spec.attachments.backupSchedule.retained.completed", kubectl), field("spec.platformConfigRef", kubectl)]
    }],
    chosen: true,
    taken: {
      objects: [{
        ref: claim("orders"),
        after: {
          apiVersion: "fleet.camunda.io/v1alpha1",
          kind: "CamundaClusterClaim",
          metadata: {
            name: "orders",
            namespace: "team-a"
          },
          spec: {
            attachments: {
              backupSchedule: {
                retained: {
                  completed: 7
                }
              }
            }
          }
        },
        diff: []
      }],
      errors: []
    }
  }
}`,...E.parameters?.docs?.source}}}})))()}O();export{T as AmbiguousPath,C as Batch,v as Changed,y as Cleared,E as ClearedTaken,b as GitOps,x as Operator,S as OperatorCleared,w as SeveralManagers,D as __namedExportsOrder,_ as default};