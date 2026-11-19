import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,r}from"./deletion-DzQuVW_V.js";import{c as i,o as a,r as o,s,t as c}from"./delete-cluster-view-DCcuted3.js";var l,u,d,f,p,m,h,g,_,v,y,b,x,S,C,w,T,E,D,O,k,A,j,M,N,P,F,I,L,R,z,B,V,H,U,W,G,K,q;function J(){return(J=e((()=>{l=t(),n(),i(),o(),{fn:u}=__STORYBOOK_MODULE_TEST__,d=(e,t)=>t,f=`8c1f6c1e-1d1a-4c5e-9d43-0f3b1c2d4e5f`,p={ownerReferences:[{controller:!0,uid:f}]},m=[{section:`access`,registration:`gateway`},{section:`backup-storage`,registration:`aws-s3`},{section:`secondary-storage`,registration:`elasticsearch`}],h=(e,t={})=>d(`CamundaClusterClaim`,{apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,metadata:{namespace:`team-orders`,name:`orders`,resourceVersion:`41`,uid:f},spec:{releaseRef:`camunda-8-9-1`,plugins:m,deletionPolicy:e},status:{resolvedPolicy:{deletionPolicy:`Orphan`},requests:m.map(e=>({...e,name:`orders-${e.section}`})),cluster:{name:`orders`},...t}}),g=(e,t,n,r)=>d(`PluginSection`,{metadata:{name:e},spec:{label:t,order:n,selection:`One`,appliesTo:[`CamundaClusterClaim`],...r&&{slot:r}}}),_=(e,t,n,r)=>d(`PluginRegistration`,{metadata:{name:e},spec:{label:t,sections:n,deletionPolicies:r}}),v=(e,t)=>d(`PluginRequest`,{metadata:{namespace:`team-orders`,name:e,...p},status:{resources:t}}),y=e=>({spec:{clusterRef:{name:e}}}),b=(e=`True`,t=`Healthy`)=>({observedGeneration:4,conditions:[{type:`Ready`,status:e,reason:t,observedGeneration:4},{type:`ZeebeReady`,status:`True`,reason:`Healthy`,observedGeneration:4}]}),x=e=>d(`CamundaCluster`,{metadata:{namespace:`team-orders`,name:`orders`,generation:4,...p},spec:{zeebe:{persistentVolumeClaimRetentionPolicy:{whenDeleted:`Retain`}}},status:e}),S=(e={})=>({claim:h(),cluster:x(b()),environments:[],sections:[g(`access`,`Access`,10,`externalAccess`),g(`backup-storage`,`Backup storage`,20,`backupStorage`),g(`secondary-storage`,`Secondary storage`,30,`secondaryStorage`)],registrations:[_(`gateway`,`Gateway`,[`access`],[`Delete`,`Orphan`]),_(`aws-s3`,`Amazon S3`,[`backup-storage`],[`Delete`,`Orphan`]),_(`elasticsearch`,`Elasticsearch`,[`secondary-storage`],[`Delete`,`Orphan`])],requests:[v(`orders-access`,[{kind:`HTTPRoute`,name:`orders`,onOrphan:`Deleted`},{kind:`Certificate`,name:`orders-tls`,onOrphan:`Kept`}]),v(`orders-backup-storage`,[{kind:`S3Bucket`,name:`orders-backups-7f3a`,id:`arn:aws:s3:::orders-backups-7f3a`,onOrphan:`Kept`}]),v(`orders-secondary-storage`,[{kind:`ElasticsearchCluster`,name:`orders-es`,onOrphan:`Kept`}])],optimizes:[d(`CamundaOptimize`,{metadata:{namespace:`team-orders`,name:`orders-optimize`,...p},...y(`orders`)})],schedules:[d(`BackupSchedule`,{metadata:{namespace:`team-orders`,name:`orders-backups`,...p},...y(`orders`)}),d(`BackupSchedule`,{metadata:{namespace:`team-orders`,name:`orders-hourly`},...y(`orders`)})],backups:{elasticsearch:[`orders-1`,`orders-2`,`orders-3`].map(e=>d(`LogicalBackupElasticsearch`,{metadata:{namespace:`team-orders`,name:e},...y(`orders`)})),rdbms:[]},presets:[],releases:[],errors:[],...e}),C=S({claim:h(`Orphan`),registrations:[_(`gateway`,`Gateway`,[`access`],[`Delete`]),_(`aws-s3`,`Amazon S3`,[`backup-storage`],[`Delete`,`Orphan`]),_(`elasticsearch`,`Elasticsearch`,[`secondary-storage`],[`Delete`,`Orphan`])]}),w=a(S(),`Orphan`),T=r(S().claim,`Orphan`,`Delete the cluster orders`),E={title:`cluster/DeleteClusterView`,component:c,args:{model:w,mode:`direct`,stage:{kind:`choose`},errors:[],remove:T.remove,policyStep:!1,onPick:u(),preview:u(async()=>({objects:[],errors:[]})),apply:u(async()=>({ok:!0,mode:`direct`})),show:u(),onWritten:u(),onStopWaiting:u()},decorators:[e=>(0,l.jsx)(`div`,{className:`max-w-3xl`,children:(0,l.jsx)(e,{})})]},D=s(w.targets,{CamundaOptimize:[],BackupSchedule:[],CamundaCluster:[{metadata:{name:`orders`,deletionTimestamp:`2026-11-15T10:00:00Z`}}],PluginRequest:[{metadata:{name:`orders-access`}},{metadata:{name:`orders-backup-storage`}},{metadata:{name:`orders-secondary-storage`}}],CamundaClusterClaim:[{metadata:{name:`orders`,deletionTimestamp:`2026-11-15T10:00:00Z`}}]}),O={name:`Keep the resources`},k={name:`Delete the resources, with the bucket warning`,args:{model:a(S(),`Delete`),policyStep:!0}},A={name:`A claim on Keep with an option that cannot keep`,args:{model:a(C,`Delete`)}},j={name:`The requests could not be read`,args:{model:a(S(),`Orphan`,[`PluginRequest`]),errors:[{kind:`PluginRequest`,message:`pluginrequests.plugins.camunda.io is forbidden`}]}},M={name:`Git: the delete was refused after the policy commit`,args:{mode:`git`,stage:{kind:`choose`,errors:[{code:`Stale`,message:`The claim orders changed since you opened it.`}],afterPolicyWrite:{held:`Delete`,unapplied:`Orphan`}}}},N={name:`Keep picked, and an option cannot keep`,args:{model:a(C,`Orphan`),keepRefused:`PluginRegistration "gateway" of section "access" cannot keep the resources. Choose Delete the resources.`}},P={name:`The Ready condition of the cluster reports a problem`,args:{model:a(S({cluster:x(b(`False`,`InvalidReference`))}),`Orphan`)}},F={name:`The Optimize objects could not be read`,args:{model:a(S(),`Orphan`,[`CamundaOptimize`]),errors:[{kind:`CamundaOptimize`,message:`camundaoptimizes.core.camunda.io is forbidden`}]}},I=S({claim:h(void 0,{cluster:void 0}),cluster:d(`CamundaCluster`,{metadata:{namespace:`team-orders`,name:`orders`}})}),L={name:`A cluster that the claim did not make`,args:{model:a(I,`Orphan`)}},R={name:`The claim is deleted, and its cluster stays`,args:{model:a(I,`Orphan`),stage:{kind:`deleted`}}},z={name:`Waiting for the operator to read the deletion policy`,args:{stage:{kind:`waiting`}}},B={name:`The cluster and its requests going`,args:{stage:{kind:`deleting`,progress:D,gitWaiting:!1}}},V={name:`The operator refuses the delete, because an option cannot keep`,args:{stage:{kind:`deleting`,progress:D,gitWaiting:!1,keepRefused:`The delete waits, because PluginRegistration "gateway" of section "access" cannot keep the resources. Set the deletion policy of the claim to Delete, or add Orphan back to the deletion policies of the registration.`}}},H={name:`Git: waiting for the deletion policy`,args:{mode:`git`,stage:{kind:`waiting`}}},U={name:`Git: waiting for GitOps to delete the claim`,args:{mode:`git`,stage:{kind:`deleting`,progress:s(w.targets,{CamundaOptimize:[{metadata:{name:`orders-optimize`}}],BackupSchedule:[{metadata:{name:`orders-backups`}}],CamundaCluster:[{metadata:{name:`orders`}}],PluginRequest:[{metadata:{name:`orders-access`}},{metadata:{name:`orders-backup-storage`}},{metadata:{name:`orders-secondary-storage`}}],CamundaClusterClaim:[{metadata:{name:`orders`}}]}),gitWaiting:!0}}},W={name:`The cluster is deleted`,args:{stage:{kind:`deleted`}}},G={name:`The claim was made again`,args:{stage:{kind:`replaced`}}},K={name:`Changes are off`,args:{mode:void 0}},q=[`KeepTheResources`,`DeleteTheResources`,`KeepRefused`,`RequestsNotRead`,`PolicyWritten`,`KeepPickRefused`,`VolumesNotApplied`,`AttachmentsNotRead`,`ClusterNotMade`,`ClusterNotMadeDeleted`,`DirectPolicyWait`,`DirectProgress`,`DeleteKeepRefused`,`GitPolicyWait`,`GitDeleteWait`,`Deleted`,`Replaced`,`ChangesOff`],O.parameters={...O.parameters,docs:{...O.parameters?.docs,source:{originalSource:`{
  name: "Keep the resources"
}`,...O.parameters?.docs?.source}}},k.parameters={...k.parameters,docs:{...k.parameters?.docs,source:{originalSource:`{
  name: "Delete the resources, with the bucket warning",
  args: {
    model: deleteClusterModel(data(), "Delete"),
    policyStep: true
  }
}`,...k.parameters?.docs?.source}}},A.parameters={...A.parameters,docs:{...A.parameters?.docs,source:{originalSource:`{
  name: "A claim on Keep with an option that cannot keep",
  args: {
    model: deleteClusterModel(refused, "Delete")
  }
}`,...A.parameters?.docs?.source}}},j.parameters={...j.parameters,docs:{...j.parameters?.docs,source:{originalSource:`{
  name: "The requests could not be read",
  args: {
    model: deleteClusterModel(data(), "Orphan", ["PluginRequest"]),
    errors: [{
      kind: "PluginRequest",
      message: "pluginrequests.plugins.camunda.io is forbidden"
    }]
  }
}`,...j.parameters?.docs?.source}}},M.parameters={...M.parameters,docs:{...M.parameters?.docs,source:{originalSource:`{
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
}`,...M.parameters?.docs?.source}}},N.parameters={...N.parameters,docs:{...N.parameters?.docs,source:{originalSource:`{
  name: "Keep picked, and an option cannot keep",
  args: {
    model: deleteClusterModel(refused, "Orphan"),
    keepRefused: 'PluginRegistration "gateway" of section "access" cannot keep the resources. Choose Delete the resources.'
  }
}`,...N.parameters?.docs?.source}}},P.parameters={...P.parameters,docs:{...P.parameters?.docs,source:{originalSource:`{
  name: "The Ready condition of the cluster reports a problem",
  args: {
    model: deleteClusterModel(data({
      cluster: retaining(reconciled("False", "InvalidReference"))
    }), "Orphan")
  }
}`,...P.parameters?.docs?.source}}},F.parameters={...F.parameters,docs:{...F.parameters?.docs,source:{originalSource:`{
  name: "The Optimize objects could not be read",
  args: {
    model: deleteClusterModel(data(), "Orphan", ["CamundaOptimize"]),
    errors: [{
      kind: "CamundaOptimize",
      message: "camundaoptimizes.core.camunda.io is forbidden"
    }]
  }
}`,...F.parameters?.docs?.source}}},L.parameters={...L.parameters,docs:{...L.parameters?.docs,source:{originalSource:`{
  name: "A cluster that the claim did not make",
  args: {
    model: deleteClusterModel(notMade, "Orphan")
  }
}`,...L.parameters?.docs?.source}}},R.parameters={...R.parameters,docs:{...R.parameters?.docs,source:{originalSource:`{
  name: "The claim is deleted, and its cluster stays",
  args: {
    model: deleteClusterModel(notMade, "Orphan"),
    stage: {
      kind: "deleted"
    }
  }
}`,...R.parameters?.docs?.source}}},z.parameters={...z.parameters,docs:{...z.parameters?.docs,source:{originalSource:`{
  name: "Waiting for the operator to read the deletion policy",
  args: {
    stage: {
      kind: "waiting"
    }
  }
}`,...z.parameters?.docs?.source}}},B.parameters={...B.parameters,docs:{...B.parameters?.docs,source:{originalSource:`{
  name: "The cluster and its requests going",
  args: {
    stage: {
      kind: "deleting",
      progress,
      gitWaiting: false
    }
  }
}`,...B.parameters?.docs?.source}}},V.parameters={...V.parameters,docs:{...V.parameters?.docs,source:{originalSource:`{
  name: "The operator refuses the delete, because an option cannot keep",
  args: {
    stage: {
      kind: "deleting",
      progress,
      gitWaiting: false,
      keepRefused: 'The delete waits, because PluginRegistration "gateway" of section "access" cannot keep the resources. Set the deletion policy of the claim to Delete, or add Orphan back to the deletion policies of the registration.'
    }
  }
}`,...V.parameters?.docs?.source}}},H.parameters={...H.parameters,docs:{...H.parameters?.docs,source:{originalSource:`{
  name: "Git: waiting for the deletion policy",
  args: {
    mode: "git",
    stage: {
      kind: "waiting"
    }
  }
}`,...H.parameters?.docs?.source}}},U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
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
}`,...U.parameters?.docs?.source}}},W.parameters={...W.parameters,docs:{...W.parameters?.docs,source:{originalSource:`{
  name: "The cluster is deleted",
  args: {
    stage: {
      kind: "deleted"
    }
  }
}`,...W.parameters?.docs?.source}}},G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
  name: "The claim was made again",
  args: {
    stage: {
      kind: "replaced"
    }
  }
}`,...G.parameters?.docs?.source}}},K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
  name: "Changes are off",
  args: {
    mode: undefined
  }
}`,...K.parameters?.docs?.source}}}})))()}J();export{F as AttachmentsNotRead,K as ChangesOff,L as ClusterNotMade,R as ClusterNotMadeDeleted,V as DeleteKeepRefused,k as DeleteTheResources,W as Deleted,z as DirectPolicyWait,B as DirectProgress,U as GitDeleteWait,H as GitPolicyWait,N as KeepPickRefused,A as KeepRefused,O as KeepTheResources,M as PolicyWritten,G as Replaced,j as RequestsNotRead,P as VolumesNotApplied,q as __namedExportsOrder,E as default};