import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{b as t,c as n}from"./labels-CoLYcJ9C.js";import{n as r,t as i}from"./rollout-tab-BB8Hci2k.js";function a(e){return{apiVersion:`core.camunda.io/v1`,kind:`CamundaRelease`,metadata:{name:`camunda-${e.replaceAll(`.`,`-`)}`,resourceVersion:`7`},spec:{version:e}}}function o(e,t,n){return{apiVersion:`fleet.camunda.io/v1alpha1`,kind:`Environment`,metadata:{name:e,resourceVersion:`12`},spec:{displayName:t,presets:{camundaCluster:[`medium`]},releases:{allowed:n,default:`camunda-8-9-0`}}}}function s(e,t){return{apiVersion:`v1`,kind:`Namespace`,metadata:{name:e,labels:{[n]:t}}}}function c(e,t,{version:n,releaseRef:r=`camunda-8-9-0`,managed:i=!0,suspend:a,conditions:o}={}){return{apiVersion:`core.camunda.io/v1`,kind:`CamundaCluster`,metadata:{namespace:e,name:t,resourceVersion:`30`,...i&&{labels:{"fleet.camunda.io/claim":t}}},spec:{presetRef:`medium`,releaseRef:r,zeebe:{replicas:3},...a&&{suspend:a}},status:{...n&&{management:{version:n}},...o&&{conditions:o}}}}function l(e,t,n={}){return{apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,metadata:{namespace:e,name:t,resourceVersion:`4`},spec:{presetRef:`medium`,releaseRef:`camunda-8-9-0`,...n}}}function u(e={}){return{releases:[a(`8.9.0`),f,a(`8.8.0`),a(`8.11.0`)],environments:[o(`production`,`Production`,[`camunda-8-9-0`,`camunda-8-9-1`]),o(`staging`,`Staging`,[`camunda-8-9-0`])],namespaces:[s(`team-orders`,`production`),s(`team-pay`,`production`),s(`team-stage`,`staging`),s(`team-legacy`,`production`)],clusters:[c(`team-orders`,`orders`,{version:`8.9.0`}),c(`team-stage`,`stage`,{version:`8.9.0`}),c(`team-pay`,`payments`,{version:`8.9.4`}),c(`team-pay`,`ledger`,{version:`8.7.2`,conditions:[{type:`Ready`,status:`False`,reason:`VersionSkipRefused`,message:d}]}),c(`team-pay`,`billing`,{version:`8.9.0`}),c(`team-legacy`,`legacy`,{version:`8.9.0`,managed:!1}),c(`team-pay`,`paused`,{suspend:!0}),c(`team-pay`,`fresh`),c(`team-pay`,`current`,{version:`8.9.1`,releaseRef:`camunda-8-9-1`}),c(`team-pay`,`orphan`,{version:`8.9.0`})],claims:[l(`team-orders`,`orders`,{plugins:[{section:`secondary-storage`,registration:`elasticsearch`}],attachments:{optimize:{}}}),l(`team-stage`,`stage`),l(`team-pay`,`payments`,{allowVersionDowngrade:`8.9.0`}),l(`team-pay`,`ledger`),l(`team-pay`,`billing`),l(`team-pay`,`paused`),l(`team-pay`,`fresh`),l(`team-pay`,`current`,{releaseRef:`camunda-8-9-1`})],databaseServers:[],clusterPresets:[{apiVersion:`core.camunda.io/v1`,kind:`CamundaClusterPreset`,metadata:{name:`medium`},spec:{cluster:{}}}],sections:[{apiVersion:`plugins.camunda.io/v1alpha1`,kind:`PluginSection`,metadata:{name:`secondary-storage`},spec:{label:`Secondary storage`,appliesTo:[`CamundaClusterClaim`],selection:`One`}}],registrations:[{apiVersion:`plugins.camunda.io/v1alpha1`,kind:`PluginRegistration`,metadata:{name:`elasticsearch`},spec:{label:`Elasticsearch`,section:`secondary-storage`,mode:`Default`,deletionPolicies:[`Delete`,`Orphan`],parameters:{type:`object`,properties:{releaseRef:{type:`string`}}}}}],requests:[],elasticsearchBackups:[{apiVersion:`core.camunda.io/v1`,kind:`LogicalBackupElasticsearch`,metadata:{namespace:`team-pay`,name:`billing-nightly`},spec:{clusterRef:{name:`billing`}},status:{phase:`Pending`}}],rdbmsBackups:[],...e}}var d,f;function p(){return(p=e((()=>{t(),d=`The cluster runs 8.7.2. Move it to an 8.8 release before 8.9.1.`,f=a(`8.9.1`)})))()}function m(){let e=u();return{...e,clusters:e.clusters.map(e=>e.metadata?.name===`fresh`?c(`team-pay`,`fresh`,{version:`8.9.0`}):e)}}var h,g,_,v,y,b,x,S,C,w,T,E,D,O,k,A;function j(){return(j=e((()=>{p(),r(),{expect:h,fn:g,userEvent:_,waitFor:v,within:y}=__STORYBOOK_MODULE_TEST__,b=e=>({ref:{apiVersion:e.apiVersion,kind:e.kind,namespace:e.metadata.namespace,name:e.metadata.name},diff:[{op:`removed`,text:`  releaseRef: camunda-8-9-0`},{op:`added`,text:`  releaseRef: camunda-8-9-1`}]}),x={title:`setup/releases/RolloutTab`,component:i,args:{release:f,data:u(),mode:`direct`,actions:{previewMove:g(async e=>({objects:e.apply.map(b),errors:[]})),applyMove:g(async()=>({ok:!0,mode:`direct`}))}}},S={name:`Every eligibility`},C={name:`Bulk review with impacts`,args:{data:m()},play:async({canvasElement:e})=>{let t=y(e);await _.click(t.getByRole(`button`,{name:`Select all that can move`})),await _.click(t.getByRole(`button`,{name:`Move selected`})),await h(await t.findByText(/team-pay\/fresh: The brokers restart/)).toBeVisible()}},w={name:`Downgrade confirm`,play:async({canvasElement:e})=>{let t=y(e),n=t.getByRole(`link`,{name:`payments`}).closest(`tr`);await _.click(y(n).getByRole(`button`,{name:`Confirm downgrade`})),await h(await t.findByRole(`button`,{name:`Confirm downgrade`})).toBeVisible()}},T={name:`Partial write`,args:{data:m(),actions:{previewMove:g(async e=>({objects:e.apply.map(b),errors:[]})),applyMove:g(async()=>({ok:!1,errors:[{code:`Stale`,message:`The CamundaClusterClaim fresh changed since you opened it.`,ref:{apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,namespace:`team-pay`,name:`fresh`}}]}))}},play:async({canvasElement:e})=>{let t=y(e);await _.click(t.getByRole(`button`,{name:`Select all that can move`})),await _.click(t.getByRole(`button`,{name:`Move selected`}));let n=await t.findByRole(`button`,{name:`Move`});await v(()=>h(n).toBeEnabled()),await _.click(n),await h(await t.findByText(`team-pay/fresh: The CamundaClusterClaim fresh changed since you opened it.`)).toBeVisible()}},E={name:`Move that did not finish`,args:T.args,play:async e=>{await T.play(e);let t=y(e.canvasElement);await _.click(t.getByRole(`button`,{name:`Back`})),await h(t.getByRole(`alert`,{name:`The move did not finish`})).toBeVisible()}},D={name:`Refused on this release`,args:{data:{...u(),clusters:u().clusters.map(e=>e.metadata?.name===`payments`?c(`team-pay`,`payments`,{version:`8.9.4`,releaseRef:`camunda-8-9-1`,conditions:[{type:`Ready`,status:`False`,reason:`VersionDowngradeRefused`,message:`the effective version 8.9.1 is below the running version 8.9.4`}]}):e),claims:u().claims.map(e=>e.metadata.name===`payments`?l(`team-pay`,`payments`,{releaseRef:`camunda-8-9-1`}):e)}}},O={name:`A list that cannot be read`,args:{data:{...u(),errors:[{kind:`LogicalBackupRDBMS`,message:`logicalbackuprdbms.core.camunda.io is forbidden: User "fleet-manager" cannot list resource "logicalbackuprdbms"`}]}}},k={name:`Without settings`,args:{mode:void 0}},A=[`EveryEligibility`,`BulkReviewWithImpacts`,`DowngradeConfirm`,`PartialWrite`,`MoveThatDidNotFinish`,`RefusedOnThisRelease`,`ListThatCannotBeRead`,`WithoutSettings`],S.parameters={...S.parameters,docs:{...S.parameters?.docs,source:{originalSource:`{
  name: "Every eligibility"
}`,...S.parameters?.docs?.source}}},C.parameters={...C.parameters,docs:{...C.parameters?.docs,source:{originalSource:`{
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
}`,...C.parameters?.docs?.source}}},w.parameters={...w.parameters,docs:{...w.parameters?.docs,source:{originalSource:`{
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
}`,...w.parameters?.docs?.source}}},T.parameters={...T.parameters,docs:{...T.parameters?.docs,source:{originalSource:`{
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
}`,...T.parameters?.docs?.source}}},E.parameters={...E.parameters,docs:{...E.parameters?.docs,source:{originalSource:`{
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
}`,...E.parameters?.docs?.source}}},D.parameters={...D.parameters,docs:{...D.parameters?.docs,source:{originalSource:`{
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
}`,...D.parameters?.docs?.source}}},O.parameters={...O.parameters,docs:{...O.parameters?.docs,source:{originalSource:`{
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
}`,...O.parameters?.docs?.source}}},k.parameters={...k.parameters,docs:{...k.parameters?.docs,source:{originalSource:`{
  name: "Without settings",
  args: {
    mode: undefined
  }
}`,...k.parameters?.docs?.source}}}})))()}j();export{C as BulkReviewWithImpacts,w as DowngradeConfirm,S as EveryEligibility,O as ListThatCannotBeRead,E as MoveThatDidNotFinish,T as PartialWrite,D as RefusedOnThisRelease,k as WithoutSettings,A as __namedExportsOrder,x as default};