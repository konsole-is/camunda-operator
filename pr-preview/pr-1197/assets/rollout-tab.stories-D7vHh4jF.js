import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{c as r,x as i}from"./labels-Be1Q9KU6.js";import{n as a,t as o}from"./rollout-tab-BTtF3IV7.js";function s(e){return{apiVersion:`core.camunda.io/v1`,kind:`CamundaRelease`,metadata:{name:`camunda-${e.replaceAll(`.`,`-`)}`,resourceVersion:`7`},spec:{version:e}}}function c(e,t,n){return{apiVersion:`fleet.camunda.io/v1alpha1`,kind:`Environment`,metadata:{name:e,resourceVersion:`12`},spec:{displayName:t,presets:{camundaCluster:[`medium`]},releases:{allowed:n,default:`camunda-8-9-0`}}}}function l(e,t){return{apiVersion:`v1`,kind:`Namespace`,metadata:{name:e,labels:{[r]:t}}}}function u(e,t,{version:n,releaseRef:r=`camunda-8-9-0`,managed:i=!0,suspend:a,conditions:o}={}){return{apiVersion:`core.camunda.io/v1`,kind:`CamundaCluster`,metadata:{namespace:e,name:t,resourceVersion:`30`,...i&&{labels:{"fleet.camunda.io/claim":t}}},spec:{presetRef:`medium`,releaseRef:r,zeebe:{replicas:3},...a&&{suspend:a}},status:{...n&&{management:{version:n}},...o&&{conditions:o}}}}function d(e,t,n={}){return{apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,metadata:{namespace:e,name:t,resourceVersion:`4`},spec:{presetRef:`medium`,releaseRef:`camunda-8-9-0`,...n}}}function f(e={}){return{releases:[s(`8.9.0`),m,s(`8.8.0`),s(`8.11.0`)],environments:[c(`production`,`Production`,[`camunda-8-9-0`,`camunda-8-9-1`]),c(`staging`,`Staging`,[`camunda-8-9-0`])],namespaces:[l(`team-orders`,`production`),l(`team-pay`,`production`),l(`team-stage`,`staging`),l(`team-legacy`,`production`)],clusters:[u(`team-orders`,`orders`,{version:`8.9.0`}),u(`team-stage`,`stage`,{version:`8.9.0`}),u(`team-pay`,`payments`,{version:`8.9.4`}),u(`team-pay`,`ledger`,{version:`8.7.2`,conditions:[{type:`Ready`,status:`False`,reason:`VersionSkipRefused`,message:p}]}),u(`team-pay`,`billing`,{version:`8.9.0`}),u(`team-legacy`,`legacy`,{version:`8.9.0`,managed:!1}),u(`team-pay`,`paused`,{suspend:!0}),u(`team-pay`,`fresh`),u(`team-pay`,`current`,{version:`8.9.1`,releaseRef:`camunda-8-9-1`}),u(`team-pay`,`orphan`,{version:`8.9.0`})],claims:[d(`team-orders`,`orders`,{plugins:[{section:`secondary-storage`,registration:`elasticsearch`}],attachments:{optimize:{}}}),d(`team-stage`,`stage`),d(`team-pay`,`payments`,{allowVersionDowngrade:`8.9.0`}),d(`team-pay`,`ledger`),d(`team-pay`,`billing`),d(`team-pay`,`paused`),d(`team-pay`,`fresh`),d(`team-pay`,`current`,{releaseRef:`camunda-8-9-1`})],databaseServers:[],clusterPresets:[{apiVersion:`core.camunda.io/v1`,kind:`CamundaClusterPreset`,metadata:{name:`medium`},spec:{cluster:{}}}],sections:[{apiVersion:`plugins.camunda.io/v1alpha1`,kind:`PluginSection`,metadata:{name:`secondary-storage`},spec:{label:`Secondary storage`,appliesTo:[`CamundaClusterClaim`],selection:`One`}}],registrations:[{apiVersion:`plugins.camunda.io/v1alpha1`,kind:`PluginRegistration`,metadata:{name:`elasticsearch`},spec:{label:`Elasticsearch`,section:`secondary-storage`,mode:`Default`,deletionPolicies:[`Delete`,`Orphan`],parameters:{type:`object`,properties:{releaseRef:{type:`string`}}}}}],requests:[],elasticsearchBackups:[{apiVersion:`core.camunda.io/v1`,kind:`LogicalBackupElasticsearch`,metadata:{namespace:`team-pay`,name:`billing-nightly`},spec:{clusterRef:{name:`billing`}},status:{phase:`Pending`}}],rdbmsBackups:[],...e}}var p,m;function h(){return(h=e((()=>{i(),p=`The cluster runs 8.7.2. Move it to an 8.8 release before 8.9.1.`,m=s(`8.9.1`)})))()}function g(){let e=f();return{...e,clusters:e.clusters.map(e=>e.metadata?.name===`fresh`?u(`team-pay`,`fresh`,{version:`8.9.0`}):e)}}var _,v,y,b,x,S,C,w,T,E,D,O,k,A,j,M,N,P,F,I;function L(){return(L=e((()=>{_=n(),v=t(),h(),a(),{expect:y,fn:b,userEvent:x,waitFor:S,within:C}=__STORYBOOK_MODULE_TEST__,w=e=>({ref:{apiVersion:e.apiVersion,kind:e.kind,namespace:e.metadata.namespace,name:e.metadata.name},diff:[{op:`removed`,text:`  releaseRef: camunda-8-9-0`},{op:`added`,text:`  releaseRef: camunda-8-9-1`}]}),T={title:`setup/releases/RolloutTab`,component:o,args:{release:m,data:f(),mode:`direct`,actions:{previewMove:b(async e=>({objects:e.apply.map(w),errors:[]})),applyMove:b(async()=>({ok:!0,mode:`direct`}))},onFailed:b()},render:function(e){let[t,n]=(0,v.useState)();return(0,_.jsx)(o,{...e,failed:t,onFailed:t=>{e.onFailed(t),n(t)}})}},E={name:`Every eligibility`},D={name:`Bulk review with impacts`,args:{data:g()},play:async({canvasElement:e})=>{let t=C(e);await x.click(t.getByRole(`button`,{name:`Select all that can move`})),await x.click(t.getByRole(`button`,{name:`Move selected`})),await y(await t.findByText(/team-pay\/fresh: The brokers restart/)).toBeVisible()}},O=[[`team-orders`,`orders`],[`team-pay`,`fresh`]].map(([e,t])=>({apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,namespace:e,name:t})),k={name:`A move that takes over a field`,args:{data:{...g(),elasticsearchBackups:[]},actions:{previewMove:b(async e=>({objects:e.apply.map(w),errors:e.takeOver?[]:O.map(e=>({code:`Conflict`,message:`conflict with "kubectl-client-side-apply"`,path:`spec.releaseRef`,manager:`kubectl-client-side-apply`,ref:e})),...!e.takeOver&&{ownership:O.map(e=>({ref:e,changed:[{path:`spec.releaseRef`,steps:[{field:`spec`},{field:`releaseRef`}],managers:[{name:`kubectl-client-side-apply`,operation:`Update`,kind:`other`,known:!0}]}],cleared:[]}))}})),applyMove:b(async()=>({ok:!0,mode:`direct`}))}},play:async({canvasElement:e})=>{let t=C(e);await x.click(t.getByRole(`button`,{name:`Select all that can move`})),await x.click(t.getByRole(`button`,{name:`Move selected`}));let n=await t.findByRole(`group`,{name:`Fields that another tool set`});await y(C(n).getByRole(`list`,{name:`CamundaClusterClaim fresh in team-pay`})).toBeVisible();let r=t.getByRole(`region`,{name:`Review the move`});await y(C(r).getByRole(`button`,{name:`Move`})).toBeDisabled()}},A={name:`Downgrade confirm`,play:async({canvasElement:e})=>{let t=C(e),n=t.getByRole(`link`,{name:`payments`}).closest(`tr`);await x.click(C(n).getByRole(`button`,{name:`Confirm downgrade`})),await y(await t.findByRole(`button`,{name:`Confirm downgrade`})).toBeVisible()}},j={name:`Partial write`,args:{data:g(),actions:{previewMove:b(async e=>({objects:e.apply.map(w),errors:[]})),applyMove:b(async()=>({ok:!1,errors:[{code:`Stale`,message:`The CamundaClusterClaim fresh changed since you opened it.`,ref:{apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,namespace:`team-pay`,name:`fresh`}}]}))}},play:async({canvasElement:e})=>{let t=C(e);await x.click(t.getByRole(`button`,{name:`Select all that can move`})),await x.click(t.getByRole(`button`,{name:`Move selected`}));let n=await t.findByRole(`button`,{name:`Move`});await S(()=>y(n).toBeEnabled()),await x.click(n),await y(await t.findByText(`team-pay/fresh: The CamundaClusterClaim fresh changed since you opened it.`)).toBeVisible()}},M={name:`Move that did not finish`,args:j.args,play:async e=>{await j.play(e);let t=C(e.canvasElement);await x.click(t.getByRole(`button`,{name:`Back`})),await y(t.getByRole(`alert`,{name:`The move did not finish`})).toBeVisible()}},N={name:`Refused on this release`,args:{data:{...f(),clusters:f().clusters.map(e=>e.metadata?.name===`payments`?u(`team-pay`,`payments`,{version:`8.9.4`,releaseRef:`camunda-8-9-1`,conditions:[{type:`Ready`,status:`False`,reason:`VersionDowngradeRefused`,message:`the effective version 8.9.1 is below the running version 8.9.4`}]}):e),claims:f().claims.map(e=>e.metadata.name===`payments`?d(`team-pay`,`payments`,{releaseRef:`camunda-8-9-1`}):e)}}},P={name:`A list that cannot be read`,args:{data:{...f(),errors:[{kind:`LogicalBackupRDBMS`,message:`logicalbackuprdbms.core.camunda.io is forbidden: User "fleet-manager" cannot list resource "logicalbackuprdbms"`}]}}},F={name:`Without settings`,args:{mode:void 0}},I=[`EveryEligibility`,`BulkReviewWithImpacts`,`MoveThatTakesOverAField`,`DowngradeConfirm`,`PartialWrite`,`MoveThatDidNotFinish`,`RefusedOnThisRelease`,`ListThatCannotBeRead`,`WithoutSettings`],E.parameters={...E.parameters,docs:{...E.parameters?.docs,source:{originalSource:`{
  name: "Every eligibility"
}`,...E.parameters?.docs?.source}}},D.parameters={...D.parameters,docs:{...D.parameters?.docs,source:{originalSource:`{
  name: "Bulk review with impacts",
  args: {
    data: twoCanMove()
  },
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", {
      name: "Select all that can move"
    }));
    await userEvent.click(canvas.getByRole("button", {
      name: "Move selected"
    }));
    await expect(await canvas.findByText(/team-pay\\/fresh: The brokers restart/)).toBeVisible();
  }
}`,...D.parameters?.docs?.source}}},k.parameters={...k.parameters,docs:{...k.parameters?.docs,source:{originalSource:`{
  name: "A move that takes over a field",
  args: {
    data: {
      ...twoCanMove(),
      elasticsearchBackups: []
    },
    actions: {
      previewMove: fn(async (c: Change) => ({
        objects: c.apply.map(changed),
        errors: c.takeOver ? [] : byKubectl.map(ref => ({
          code: "Conflict" as const,
          message: 'conflict with "kubectl-client-side-apply"',
          path: "spec.releaseRef",
          manager: "kubectl-client-side-apply",
          ref
        })),
        ...(!c.takeOver && {
          ownership: byKubectl.map(ref => ({
            ref,
            changed: [{
              path: "spec.releaseRef",
              steps: [{
                field: "spec"
              }, {
                field: "releaseRef"
              }],
              managers: [{
                name: "kubectl-client-side-apply",
                operation: "Update",
                kind: "other" as const,
                known: true
              }]
            }],
            cleared: []
          }))
        })
      })),
      applyMove: fn(async () => ({
        ok: true as const,
        mode: "direct" as const
      }))
    }
  },
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", {
      name: "Select all that can move"
    }));
    await userEvent.click(canvas.getByRole("button", {
      name: "Move selected"
    }));
    const group = await canvas.findByRole("group", {
      name: "Fields that another tool set"
    });
    await expect(within(group).getByRole("list", {
      name: "CamundaClusterClaim fresh in team-pay"
    })).toBeVisible();
    const review = canvas.getByRole("region", {
      name: "Review the move"
    });
    await expect(within(review).getByRole("button", {
      name: "Move"
    })).toBeDisabled();
  }
}`,...k.parameters?.docs?.source}}},A.parameters={...A.parameters,docs:{...A.parameters?.docs,source:{originalSource:`{
  name: "Downgrade confirm",
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    const row = canvas.getByRole("link", {
      name: "payments"
    }).closest("tr")!;
    await userEvent.click(within(row).getByRole("button", {
      name: "Confirm downgrade"
    }));
    await expect(await canvas.findByRole("button", {
      name: "Confirm downgrade"
    })).toBeVisible();
  }
}`,...A.parameters?.docs?.source}}},j.parameters={...j.parameters,docs:{...j.parameters?.docs,source:{originalSource:`{
  name: "Partial write",
  args: {
    data: twoCanMove(),
    actions: {
      previewMove: fn(async (c: Change) => ({
        objects: c.apply.map(changed),
        errors: []
      })),
      applyMove: fn(async () => ({
        ok: false as const,
        errors: [{
          code: "Stale" as const,
          message: "The CamundaClusterClaim fresh changed since you opened it.",
          ref: {
            apiVersion: "fleet.camunda.io/v1alpha1",
            kind: "CamundaClusterClaim",
            namespace: "team-pay",
            name: "fresh"
          }
        }]
      }))
    }
  },
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", {
      name: "Select all that can move"
    }));
    await userEvent.click(canvas.getByRole("button", {
      name: "Move selected"
    }));
    const confirm = await canvas.findByRole("button", {
      name: "Move"
    });
    await waitFor(() => expect(confirm).toBeEnabled());
    await userEvent.click(confirm);
    await expect(await canvas.findByText("team-pay/fresh: The CamundaClusterClaim fresh changed since you opened it.")).toBeVisible();
  }
}`,...j.parameters?.docs?.source}}},M.parameters={...M.parameters,docs:{...M.parameters?.docs,source:{originalSource:`{
  name: "Move that did not finish",
  args: PartialWrite.args,
  play: async context => {
    await PartialWrite.play!(context);
    const canvas = within(context.canvasElement);
    await userEvent.click(canvas.getByRole("button", {
      name: "Back"
    }));
    await expect(canvas.getByRole("alert", {
      name: "The move did not finish"
    })).toBeVisible();
  }
}`,...M.parameters?.docs?.source}}},N.parameters={...N.parameters,docs:{...N.parameters?.docs,source:{originalSource:`{
  name: "Refused on this release",
  args: {
    data: {
      ...rolloutData(),
      clusters: rolloutData().clusters.map(c => c.metadata?.name === "payments" ? cluster("team-pay", "payments", {
        version: "8.9.4",
        releaseRef: "camunda-8-9-1",
        conditions: [{
          type: "Ready",
          status: "False",
          reason: "VersionDowngradeRefused",
          message: "the effective version 8.9.1 is below the running version 8.9.4"
        }]
      }) : c),
      claims: rolloutData().claims.map(c => c.metadata.name === "payments" ? clusterClaim("team-pay", "payments", {
        releaseRef: "camunda-8-9-1"
      }) : c)
    }
  }
}`,...N.parameters?.docs?.source}}},P.parameters={...P.parameters,docs:{...P.parameters?.docs,source:{originalSource:`{
  name: "A list that cannot be read",
  args: {
    data: {
      ...rolloutData(),
      errors: [{
        kind: "LogicalBackupRDBMS",
        message: 'logicalbackuprdbms.core.camunda.io is forbidden: User "fleet-manager" cannot list resource "logicalbackuprdbms"'
      }]
    }
  }
}`,...P.parameters?.docs?.source}}},F.parameters={...F.parameters,docs:{...F.parameters?.docs,source:{originalSource:`{
  name: "Without settings",
  args: {
    mode: undefined
  }
}`,...F.parameters?.docs?.source}}}})))()}L();export{D as BulkReviewWithImpacts,A as DowngradeConfirm,E as EveryEligibility,P as ListThatCannotBeRead,M as MoveThatDidNotFinish,k as MoveThatTakesOverAField,j as PartialWrite,N as RefusedOnThisRelease,F as WithoutSettings,I as __namedExportsOrder,T as default};