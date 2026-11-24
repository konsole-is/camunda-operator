import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,r}from"./deletion-DlBOMHtV.js";import{c as i,o as a,r as ee,s as o,t as s}from"./delete-cluster-view-Td4lJ1U2.js";var c,l,u,d,f,p,m,h,g,_,v,y,b,x,S,C,w,T,E,D,O,k,A,j,M,N,P,F,I,L,R,z,B,V,H,U,W,G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{c=t(),n(),i(),ee(),{expect:l,fn:u,userEvent:d,within:f}=__STORYBOOK_MODULE_TEST__,p=(e,t)=>t,m=`8c1f6c1e-1d1a-4c5e-9d43-0f3b1c2d4e5f`,h={ownerReferences:[{controller:!0,uid:m}]},g=[{section:`access`,registration:`gateway`},{section:`backup-storage`,registration:`aws-s3`},{section:`secondary-storage`,registration:`elasticsearch`}],_=(e,t={})=>p(`CamundaClusterClaim`,{apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,metadata:{namespace:`team-orders`,name:`orders`,resourceVersion:`41`,uid:m},spec:{releaseRef:`camunda-8-9-1`,plugins:g,deletionPolicy:e},status:{resolvedPolicy:{deletionPolicy:`Orphan`},requests:g.map(e=>({...e,name:`orders-${e.section}`})),cluster:{name:`orders`},...t}}),v=(e,t,n,r)=>p(`PluginSection`,{metadata:{name:e},spec:{label:t,order:n,selection:`One`,appliesTo:[`CamundaClusterClaim`],...r&&{slot:r}}}),y=(e,t,n,r)=>p(`PluginRegistration`,{metadata:{name:e},spec:{label:t,sections:n,deletionPolicies:r}}),b=(e,t)=>p(`PluginRequest`,{metadata:{namespace:`team-orders`,name:e,...h},status:{resources:t}}),x=e=>({spec:{clusterRef:{name:e}}}),S=(e=`True`,t=`Healthy`)=>({observedGeneration:4,conditions:[{type:`Ready`,status:e,reason:t,observedGeneration:4},{type:`ZeebeReady`,status:`True`,reason:`Healthy`,observedGeneration:4}]}),C=e=>p(`CamundaCluster`,{metadata:{namespace:`team-orders`,name:`orders`,generation:4,...h},spec:{zeebe:{persistentVolumeClaimRetentionPolicy:{whenDeleted:`Retain`}}},status:e}),w=(e={})=>({claim:_(),cluster:C(S()),environments:[],sections:[v(`access`,`Access`,10,`externalAccess`),v(`backup-storage`,`Backup storage`,20,`backupStorage`),v(`secondary-storage`,`Secondary storage`,30,`secondaryStorage`)],registrations:[y(`gateway`,`Gateway`,[`access`],[`Delete`,`Orphan`]),y(`aws-s3`,`Amazon S3`,[`backup-storage`],[`Delete`,`Orphan`]),y(`elasticsearch`,`Elasticsearch`,[`secondary-storage`],[`Delete`,`Orphan`])],requests:[b(`orders-access`,[{kind:`HTTPRoute`,name:`orders`,onOrphan:`Deleted`},{kind:`Certificate`,name:`orders-tls`,onOrphan:`Kept`}]),b(`orders-backup-storage`,[{kind:`S3Bucket`,name:`orders-backups-7f3a`,id:`arn:aws:s3:::orders-backups-7f3a`,onOrphan:`Kept`}]),b(`orders-secondary-storage`,[{kind:`ElasticsearchCluster`,name:`orders-es`,onOrphan:`Kept`}])],optimizes:[p(`CamundaOptimize`,{metadata:{namespace:`team-orders`,name:`orders-optimize`,...h},...x(`orders`)})],schedules:[p(`BackupSchedule`,{metadata:{namespace:`team-orders`,name:`orders-backups`,...h},...x(`orders`)}),p(`BackupSchedule`,{metadata:{namespace:`team-orders`,name:`orders-hourly`},...x(`orders`)})],backups:{elasticsearch:[`orders-1`,`orders-2`,`orders-3`].map(e=>p(`LogicalBackupElasticsearch`,{metadata:{namespace:`team-orders`,name:e},...x(`orders`)})),rdbms:[]},presets:[],releases:[],errors:[],...e}),T=w({claim:_(`Orphan`),registrations:[y(`gateway`,`Gateway`,[`access`],[`Delete`]),y(`aws-s3`,`Amazon S3`,[`backup-storage`],[`Delete`,`Orphan`]),y(`elasticsearch`,`Elasticsearch`,[`secondary-storage`],[`Delete`,`Orphan`])]}),E=a(w(),`Orphan`),D=r(w().claim,`Orphan`,`Delete the cluster orders`),O={title:`cluster/DeleteClusterView`,component:s,args:{model:E,mode:`direct`,stage:{kind:`choose`},errors:[],remove:D.remove,policyStep:!1,onPick:u(),preview:u(async()=>({objects:[],errors:[]})),apply:u(async()=>({ok:!0,mode:`direct`})),show:u(),onWritten:u(),onStopWaiting:u()},decorators:[e=>(0,c.jsx)(`div`,{className:`max-w-3xl`,children:(0,c.jsx)(e,{})})]},k=o(E.targets,{CamundaOptimize:[],BackupSchedule:[],CamundaCluster:[{metadata:{name:`orders`,deletionTimestamp:`2026-11-15T10:00:00Z`}}],PluginRequest:[{metadata:{name:`orders-access`}},{metadata:{name:`orders-backup-storage`}},{metadata:{name:`orders-secondary-storage`}}],CamundaClusterClaim:[{metadata:{name:`orders`,deletionTimestamp:`2026-11-15T10:00:00Z`}}]}),A={name:`Keep the resources`},j={name:`Delete the resources, with the bucket warning`,args:{model:a(w(),`Delete`),policyStep:!0}},M={name:`A claim on Keep with an option that cannot keep`,args:{model:a(T,`Delete`)}},N={name:`The requests could not be read`,args:{model:a(w(),`Orphan`,[`PluginRequest`]),errors:[{kind:`PluginRequest`,message:`pluginrequests.plugins.camunda.io is forbidden`}]}},P={name:`Git: the delete was refused after the policy commit`,args:{mode:`git`,stage:{kind:`choose`,errors:[{code:`Stale`,message:`The claim orders changed since you opened it.`}],afterPolicyWrite:{held:`Delete`,unapplied:`Orphan`}}}},F={name:`Keep picked, and an option cannot keep`,args:{model:a(T,`Orphan`),keepRefused:`PluginRegistration "gateway" of section "access" cannot keep the resources. Choose Delete the resources.`}},I={name:`The Ready condition of the cluster reports a problem`,args:{model:a(w({cluster:C(S(`False`,`InvalidReference`))}),`Orphan`)}},L={name:`The Optimize objects could not be read`,args:{model:a(w(),`Orphan`,[`CamundaOptimize`]),errors:[{kind:`CamundaOptimize`,message:`camundaoptimizes.core.camunda.io is forbidden`}]}},R=w({claim:_(void 0,{cluster:void 0}),cluster:p(`CamundaCluster`,{metadata:{namespace:`team-orders`,name:`orders`}})}),z={name:`A cluster that the claim did not make`,args:{model:a(R,`Orphan`)}},B={name:`The claim is deleted, and its cluster stays`,args:{model:a(R,`Orphan`),stage:{kind:`deleted`}}},V={name:`Waiting for the operator to read the deletion policy`,args:{stage:{kind:`waiting`}}},H={name:`The cluster and its requests going`,args:{stage:{kind:`deleting`,progress:k,gitWaiting:!1}}},U={name:`The operator refuses the delete, because an option cannot keep`,args:{stage:{kind:`deleting`,progress:k,gitWaiting:!1,keepRefused:`The delete waits, because PluginRegistration "gateway" of section "access" cannot keep the resources. Set the deletion policy of the claim to Delete, or add Orphan back to the deletion policies of the registration.`}}},W={name:`Git: waiting for the deletion policy`,args:{mode:`git`,stage:{kind:`waiting`}}},G={name:`Git: waiting for GitOps to delete the claim`,args:{mode:`git`,stage:{kind:`deleting`,progress:o(E.targets,{CamundaOptimize:[{metadata:{name:`orders-optimize`}}],BackupSchedule:[{metadata:{name:`orders-backups`}}],CamundaCluster:[{metadata:{name:`orders`}}],PluginRequest:[{metadata:{name:`orders-access`}},{metadata:{name:`orders-backup-storage`}},{metadata:{name:`orders-secondary-storage`}}],CamundaClusterClaim:[{metadata:{name:`orders`}}]}),gitWaiting:!0}}},K={name:`The cluster is deleted`,args:{stage:{kind:`deleted`}}},q={name:`The claim was made again`,args:{stage:{kind:`replaced`}}},J={name:`Changes are off`,args:{mode:void 0}},Y=e=>{let t={apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,namespace:`team-orders`,name:`orders`};return{objects:[],errors:[{code:`Conflict`,message:`conflict with "${e.name}"`,path:`spec.deletionPolicy`,manager:e.name,ref:t}],ownership:[{ref:t,changed:[{path:`spec.deletionPolicy`,steps:[{field:`spec`},{field:`deletionPolicy`}],managers:[e]}],cleared:[]}]}},X={name:`Keep picked, and kubectl set the deletion policy`,args:{model:a(w({claim:_(`Delete`)}),`Orphan`),policyStep:!0,policyOwned:`Another tool set the deletion policy of the claim, so the delete cannot change it without the take-over. Take over the field below, or choose Delete the resources to delete with the policy that the claim has.`,preview:u(async(e,t)=>t?.length?{objects:[],errors:[]}:Y({name:`kubectl-client-side-apply`,operation:`Update`,kind:`other`,known:!0}))},play:async({canvasElement:e})=>{let t=f(e),n=await t.findByRole(`group`,{name:`Fields that another tool set`});await l(n).toHaveTextContent(`spec.deletionPolicy: kubectl-client-side-apply set it. The save changes it.`),await d.type(t.getByLabelText(`Type orders to confirm`),`orders`);let r=t.getByRole(`button`,{name:`Delete the cluster`});await l(r).toBeDisabled(),await d.click(t.getByRole(`checkbox`,{name:`Take over these fields`})),await l(r).toBeEnabled()}},Z={name:`Keep picked, and the operator sets the deletion policy`,args:{model:a(w({claim:_(`Delete`)}),`Orphan`),policyStep:!0,policyOwned:`The operator sets the deletion policy of the claim, so the delete cannot change it. Choose Delete the resources to delete with the policy that the claim has.`,preview:u(async()=>Y({name:`camunda-operator/CamundaClusterClaim/8c1f6c1e`,operation:`Apply`,kind:`operator`,known:!0}))},play:async({canvasElement:e})=>{let t=f(e);await l(await t.findByText(/of the operator sets it\./)).toBeVisible(),await l(t.queryByRole(`checkbox`)).toBeNull(),await d.type(t.getByLabelText(`Type orders to confirm`),`orders`),await l(t.getByRole(`button`,{name:`Delete the cluster`})).toBeDisabled()}},Q=[`KeepTheResources`,`DeleteTheResources`,`KeepRefused`,`RequestsNotRead`,`PolicyWritten`,`KeepPickRefused`,`VolumesNotApplied`,`AttachmentsNotRead`,`ClusterNotMade`,`ClusterNotMadeDeleted`,`DirectPolicyWait`,`DirectProgress`,`DeleteKeepRefused`,`GitPolicyWait`,`GitDeleteWait`,`Deleted`,`Replaced`,`ChangesOff`,`PolicyOfAnotherTool`,`PolicyOfTheOperator`],A.parameters={...A.parameters,docs:{...A.parameters?.docs,source:{originalSource:`{
  name: "Keep the resources"
}`,...A.parameters?.docs?.source}}},j.parameters={...j.parameters,docs:{...j.parameters?.docs,source:{originalSource:`{
  name: "Delete the resources, with the bucket warning",
  args: {
    model: deleteClusterModel(data(), "Delete"),
    policyStep: true
  }
}`,...j.parameters?.docs?.source}}},M.parameters={...M.parameters,docs:{...M.parameters?.docs,source:{originalSource:`{
  name: "A claim on Keep with an option that cannot keep",
  args: {
    model: deleteClusterModel(refused, "Delete")
  }
}`,...M.parameters?.docs?.source}}},N.parameters={...N.parameters,docs:{...N.parameters?.docs,source:{originalSource:`{
  name: "The requests could not be read",
  args: {
    model: deleteClusterModel(data(), "Orphan", ["PluginRequest"]),
    errors: [{
      kind: "PluginRequest",
      message: "pluginrequests.plugins.camunda.io is forbidden"
    }]
  }
}`,...N.parameters?.docs?.source}}},P.parameters={...P.parameters,docs:{...P.parameters?.docs,source:{originalSource:`{
  name: "Git: the delete was refused after the policy commit",
  args: {
    mode: "git",
    stage: {
      kind: "choose",
      errors: [{
        code: "Stale",
        message: "The claim orders changed since you opened it."
      }],
      afterPolicyWrite: {
        held: "Delete",
        unapplied: "Orphan"
      }
    }
  }
}`,...P.parameters?.docs?.source}}},F.parameters={...F.parameters,docs:{...F.parameters?.docs,source:{originalSource:`{
  name: "Keep picked, and an option cannot keep",
  args: {
    model: deleteClusterModel(refused, "Orphan"),
    keepRefused: 'PluginRegistration "gateway" of section "access" cannot keep the resources. Choose Delete the resources.'
  }
}`,...F.parameters?.docs?.source}}},I.parameters={...I.parameters,docs:{...I.parameters?.docs,source:{originalSource:`{
  name: "The Ready condition of the cluster reports a problem",
  args: {
    model: deleteClusterModel(data({
      cluster: retaining(reconciled("False", "InvalidReference"))
    }), "Orphan")
  }
}`,...I.parameters?.docs?.source}}},L.parameters={...L.parameters,docs:{...L.parameters?.docs,source:{originalSource:`{
  name: "The Optimize objects could not be read",
  args: {
    model: deleteClusterModel(data(), "Orphan", ["CamundaOptimize"]),
    errors: [{
      kind: "CamundaOptimize",
      message: "camundaoptimizes.core.camunda.io is forbidden"
    }]
  }
}`,...L.parameters?.docs?.source}}},z.parameters={...z.parameters,docs:{...z.parameters?.docs,source:{originalSource:`{
  name: "A cluster that the claim did not make",
  args: {
    model: deleteClusterModel(notMade, "Orphan")
  }
}`,...z.parameters?.docs?.source}}},B.parameters={...B.parameters,docs:{...B.parameters?.docs,source:{originalSource:`{
  name: "The claim is deleted, and its cluster stays",
  args: {
    model: deleteClusterModel(notMade, "Orphan"),
    stage: {
      kind: "deleted"
    }
  }
}`,...B.parameters?.docs?.source}}},V.parameters={...V.parameters,docs:{...V.parameters?.docs,source:{originalSource:`{
  name: "Waiting for the operator to read the deletion policy",
  args: {
    stage: {
      kind: "waiting"
    }
  }
}`,...V.parameters?.docs?.source}}},H.parameters={...H.parameters,docs:{...H.parameters?.docs,source:{originalSource:`{
  name: "The cluster and its requests going",
  args: {
    stage: {
      kind: "deleting",
      progress,
      gitWaiting: false
    }
  }
}`,...H.parameters?.docs?.source}}},U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
  name: "The operator refuses the delete, because an option cannot keep",
  args: {
    stage: {
      kind: "deleting",
      progress,
      gitWaiting: false,
      keepRefused: 'The delete waits, because PluginRegistration "gateway" of section "access" cannot keep the resources. Set the deletion policy of the claim to Delete, or add Orphan back to the deletion policies of the registration.'
    }
  }
}`,...U.parameters?.docs?.source}}},W.parameters={...W.parameters,docs:{...W.parameters?.docs,source:{originalSource:`{
  name: "Git: waiting for the deletion policy",
  args: {
    mode: "git",
    stage: {
      kind: "waiting"
    }
  }
}`,...W.parameters?.docs?.source}}},G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
  name: "Git: waiting for GitOps to delete the claim",
  args: {
    mode: "git",
    stage: {
      kind: "deleting",
      progress: deleteProgress(keep.targets, {
        CamundaOptimize: [{
          metadata: {
            name: "orders-optimize"
          }
        }],
        BackupSchedule: [{
          metadata: {
            name: "orders-backups"
          }
        }],
        CamundaCluster: [{
          metadata: {
            name: "orders"
          }
        }],
        PluginRequest: [{
          metadata: {
            name: "orders-access"
          }
        }, {
          metadata: {
            name: "orders-backup-storage"
          }
        }, {
          metadata: {
            name: "orders-secondary-storage"
          }
        }],
        CamundaClusterClaim: [{
          metadata: {
            name: "orders"
          }
        }]
      }),
      gitWaiting: true
    }
  }
}`,...G.parameters?.docs?.source}}},K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
  name: "The cluster is deleted",
  args: {
    stage: {
      kind: "deleted"
    }
  }
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  name: "The claim was made again",
  args: {
    stage: {
      kind: "replaced"
    }
  }
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "Changes are off",
  args: {
    mode: undefined
  }
}`,...J.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "Keep picked, and kubectl set the deletion policy",
  args: {
    model: deleteClusterModel(data({
      claim: claim("Delete")
    }), "Orphan"),
    policyStep: true,
    policyOwned: "Another tool set the deletion policy of the claim, so the delete cannot change it without the take-over. Take over the field below, or choose Delete the resources to delete with the policy that the claim has.",
    // With the take-over, the dry run of the policy write has no conflict.
    preview: fn(async (_change: Change, taken?: readonly ObjectOwnership[]) => taken?.length ? {
      objects: [],
      errors: []
    } : ownedPreview({
      name: "kubectl-client-side-apply",
      operation: "Update",
      kind: "other",
      known: true
    }))
  },
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    const group = await canvas.findByRole("group", {
      name: "Fields that another tool set"
    });
    await expect(group).toHaveTextContent("spec.deletionPolicy: kubectl-client-side-apply set it. The save changes it.");
    await userEvent.type(canvas.getByLabelText("Type orders to confirm"), "orders");
    const confirm = canvas.getByRole("button", {
      name: "Delete the cluster"
    });
    await expect(confirm).toBeDisabled();
    await userEvent.click(canvas.getByRole("checkbox", {
      name: "Take over these fields"
    }));
    await expect(confirm).toBeEnabled();
  }
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "Keep picked, and the operator sets the deletion policy",
  args: {
    model: deleteClusterModel(data({
      claim: claim("Delete")
    }), "Orphan"),
    policyStep: true,
    policyOwned: "The operator sets the deletion policy of the claim, so the delete cannot change it. Choose Delete the resources to delete with the policy that the claim has.",
    preview: fn(async () => ownedPreview({
      name: "camunda-operator/CamundaClusterClaim/8c1f6c1e",
      operation: "Apply",
      kind: "operator",
      known: true
    }))
  },
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    await expect(await canvas.findByText(/of the operator sets it\\./)).toBeVisible();
    await expect(canvas.queryByRole("checkbox")).toBeNull();
    await userEvent.type(canvas.getByLabelText("Type orders to confirm"), "orders");
    await expect(canvas.getByRole("button", {
      name: "Delete the cluster"
    })).toBeDisabled();
  }
}`,...Z.parameters?.docs?.source}}}})))()}$();export{L as AttachmentsNotRead,J as ChangesOff,z as ClusterNotMade,B as ClusterNotMadeDeleted,U as DeleteKeepRefused,j as DeleteTheResources,K as Deleted,V as DirectPolicyWait,H as DirectProgress,G as GitDeleteWait,W as GitPolicyWait,F as KeepPickRefused,M as KeepRefused,A as KeepTheResources,X as PolicyOfAnotherTool,Z as PolicyOfTheOperator,P as PolicyWritten,q as Replaced,N as RequestsNotRead,I as VolumesNotApplied,Q as __namedExportsOrder,O as default};