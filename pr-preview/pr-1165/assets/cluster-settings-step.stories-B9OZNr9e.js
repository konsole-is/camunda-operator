import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{a as r,c as i,d as a,r as o,t as s,u as c}from"./draft-DYATAf-Z.js";import{d as l}from"./schema-BD2tGvkb.js";import{t as u}from"./fields-BSC3qrJi.js";import{n as d,r as f,t as p}from"./native-select-cymtpjrG.js";import{a as m,i as h,n as ee,s as te}from"./settings-editor-cJUO-Kr2.js";import{i as g,r as _}from"./settings-B5hZCPCm.js";import{i as v,n as y,t as b}from"./release-options-B3MXEXeq.js";import{n as x,t as S}from"./wizard-stepper-DRcml3Wv.js";import{a as C,n as w,o as T}from"./fixtures-BIhfPx-a.js";function E({draft:e,environment:t,releases:n,platformConfigs:r,fields:a,merged:o,secrets:s,pick:c,set:l,reset:u,invalid:f,unread:m=[]}){let h=(0,O.useId)(),_=i(e,t)??``,y=t?.spec.platformConfigRef,b=e.claim.spec.platformConfigRef??y??``,x=b!==``&&b===y,S=t?.spec.displayName||t?.metadata.name,C=e.claim.spec.cluster??{},w=e.claim.spec.presetRef;return(0,D.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-6`,children:[(0,D.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[(0,D.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1`,children:[(0,D.jsx)(`label`,{htmlFor:`${h}-release`,className:`font-heading text-base font-semibold`,children:`Release`}),(0,D.jsxs)(p,{id:`${h}-release`,value:_,onChange:e=>c(`releaseRef`,e.target.value||void 0),children:[!n.some(e=>e.name===_)&&(0,D.jsx)(d,{value:_,children:m.includes(`CamundaRelease`)?`Could not read the releases`:`Pick a release`}),n.map(e=>(0,D.jsx)(d,{value:e.name,children:v(e,e.isDefault)},e.name))]})]}),(0,D.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1`,children:[(0,D.jsx)(`label`,{htmlFor:`${h}-platform`,className:`font-heading text-base font-semibold`,children:`Platform config`}),(0,D.jsxs)(p,{id:`${h}-platform`,value:b,"aria-describedby":x?`${h}-platform-note`:void 0,onChange:e=>c(`platformConfigRef`,e.target.value||void 0),children:[!r.includes(b)&&(0,D.jsx)(d,{value:b,children:m.includes(`CamundaPlatformConfig`)?`Could not read the platform configs`:`Pick a platform config`}),r.map(e=>(0,D.jsx)(d,{value:e,children:e},e))]}),x&&S&&(0,D.jsx)(`p`,{id:`${h}-platform-note`,className:`text-sm text-muted`,children:`Default from the ${S} environment.`})]})]}),(0,D.jsxs)(`div`,{className:`flex flex-col gap-3`,children:[w&&(0,D.jsxs)(`p`,{className:`max-w-prose text-sm text-muted`,children:[`Your admin made the preset`,` `,(0,D.jsx)(`span`,{className:`font-medium text-text`,children:w}),`. It sets each value that shows “From preset”. A change here applies to this cluster only. It does not change the preset.`]}),g(a.filter(e=>!k.has(e.claimPath??``)),o.spec).map(e=>(0,D.jsx)(ee,{collapsed:!0,group:e,settings:te({group:e,spec:o.spec,live:{},claim:C}),secrets:s,set:l,reset:u,invalid:f},e.component))]})]})}var D,O,k;function A(){return(A=e((()=>{D=n(),O=t(),m(),y(),h(),_(),f(),r(),k=new Set([`spec.presetRef`,`spec.releaseRef`,`spec.platformConfigRef`]),E.__docgenInfo={description:`ClusterSettingsStep is step 3 of the cluster create flow: the release,
the platform config, then the settings editors over fields, without the
fields at presetRef, releaseRef, and platformConfigRef. A note names the
preset of the draft, and each settings group starts closed. fields are
clusterFields of the draft cluster. A field that a step 2 option fills
shows locked with its section. unread are the kinds that could not be
read. It holds no draft: pick sets releaseRef or platformConfigRef, an
empty value unsets it, and set, reset, and invalid take steps from the
root of the claim.`,methods:[],displayName:`ClusterSettingsStep`,props:{draft:{required:!0,tsType:{name:`ClusterDraft`},description:``},environment:{required:!0,tsType:{name:`union`,raw:`Environment | undefined`,elements:[{name:`Environment`},{name:`undefined`}]},description:``},releases:{required:!0,tsType:{name:`unknown`},description:``},platformConfigs:{required:!0,tsType:{name:`unknown`},description:``},fields:{required:!0,tsType:{name:`unknown`},description:``},merged:{required:!0,tsType:{name:`Merged`,elements:[{name:`CamundaClusterSpec`}],raw:`Merged<CamundaClusterSpec>`},description:``},secrets:{required:!0,tsType:{name:`SecretRefs`},description:``},pick:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(
  field: "releaseRef" | "platformConfigRef",
  name: string | undefined,
) => void`,signature:{arguments:[{type:{name:`union`,raw:`"releaseRef" | "platformConfigRef"`,elements:[{name:`literal`,value:`"releaseRef"`},{name:`literal`,value:`"platformConfigRef"`}]},name:`field`},{type:{name:`union`,raw:`string | undefined`,elements:[{name:`string`},{name:`undefined`}]},name:`name`}],return:{name:`void`}}},description:``},set:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[], value: unknown) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`},{type:{name:`unknown`},name:`value`}],return:{name:`void`}}},description:``},reset:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[]) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`}],return:{name:`void`}}},description:``},invalid:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[], message: string | undefined) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`},{type:{name:`union`,raw:`string | undefined`,elements:[{name:`string`},{name:`undefined`}]},name:`message`}],return:{name:`void`}}},description:``},unread:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}}}}})))()}function j({initial:e,secrets:t,pick:n,set:r,reset:i}){let[u,d]=(0,P.useState)(e),f=o(u,C,B,V),p=l({cluster:f.cluster,merged:f.merged,claim:f.claim,sections:[H],registrations:[]});return(0,N.jsx)(S,{steps:[...s],current:`settings`,children:(0,N.jsx)(E,{draft:u,environment:V,releases:b(B,V),platformConfigs:[`basic`,`oidc`],fields:p,merged:f.merged,secrets:t,pick:(e,t)=>{n(e,t),d(n=>t===void 0?a(n,[{field:`spec`},{field:e}]):c(n,[{field:`spec`},{field:e}],t))},set:(e,t)=>{r(e,t),d(n=>c(n,e,t))},reset:e=>{i(e),d(t=>a(t,e))},invalid:()=>{}})})}function M(e){let t=R(e).getByRole(`region`,{name:`Brokers settings`});return{group:t,summary:t.querySelector(`summary`)}}var N,P,F,I,L,R,z,B,V,H,U,W,G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{N=n(),P=t(),y(),x(),r(),u(),A(),w(),{expect:F,fn:I,userEvent:L,within:R}=__STORYBOOK_MODULE_TEST__,z=(e,t)=>({apiVersion:`core.camunda.io/v1`,kind:`CamundaRelease`,metadata:{name:e},spec:{version:t}}),B=[z(`camunda-8-8`,`8.8.4`),z(`camunda-8-9`,`8.9.9`)],V={...T,spec:{...T.spec,releases:{allowed:[`camunda-8-8`,`camunda-8-9`],default:`camunda-8-9`},platformConfigRef:`basic`}},H={apiVersion:`plugins.camunda.io/v1alpha1`,kind:`PluginSection`,metadata:{name:`access`},spec:{appliesTo:[`CamundaClusterClaim`],label:`Access`,selection:`One`,slot:`externalAccess`}},U=[{name:`orders-db`,keys:[`password`,`username`]},{name:`orders-license`,keys:[`license`]}],W={environment:`prod`,namespace:{kind:`existing`,name:`team-orders`},claim:{metadata:{name:`orders`},spec:{presetRef:`medium`}}},G={title:`create/ClusterSettingsStep`,component:j,args:{initial:W,secrets:U,pick:I(),set:I(),reset:I()},render:e=>(0,N.jsx)(j,{...e},JSON.stringify(e))},K={},q={args:{initial:{...W,claim:{...W.claim,spec:{...W.claim.spec,releaseRef:`camunda-8-8`,platformConfigRef:`oidc`,cluster:{podLabels:{team:`orders`}}}}}}},J={name:`Filled by Access`,args:{initial:{...W,storage:{access:[{section:`access`,registration:`gateway`}]}}}},Y={name:`New namespace`,args:{initial:{...W,namespace:{kind:`new`,name:`team-new`,labels:{}}},secrets:[]}},X={name:`Changed setting`,args:{initial:{...W,claim:{...W.claim,spec:{...W.claim.spec,cluster:{zeebe:{replicas:5}}}}}},play:async({canvasElement:e})=>{await F(M(e).summary).toHaveTextContent(`1 changed in the claim`)}},Z={name:`Error in a section`,play:async({canvasElement:e})=>{let{group:t,summary:n}=M(e);await L.click(n);let r=R(R(t).getAllByRole(`group`,{name:`replicas`})[0]).getByRole(`spinbutton`);await L.clear(r),await L.type(r,`-1`),await L.click(n),await F(n).toHaveTextContent(`1 setting has an error`)}},Q=[`Defaults`,`Overridden`,`FilledByAccess`,`NewNamespace`,`ChangedSetting`,`ErrorInSection`],K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
  args: {
    initial: {
      ...base,
      claim: {
        ...base.claim,
        spec: {
          ...base.claim.spec,
          releaseRef: "camunda-8-8",
          platformConfigRef: "oidc",
          cluster: {
            podLabels: {
              team: "orders"
            }
          }
        }
      }
    }
  }
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
  name: "Filled by Access",
  args: {
    initial: {
      ...base,
      storage: {
        access: [{
          section: "access",
          registration: "gateway"
        }]
      }
    }
  }
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "New namespace",
  args: {
    initial: {
      ...base,
      namespace: {
        kind: "new",
        name: "team-new",
        labels: {}
      }
    },
    secrets: []
  }
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "Changed setting",
  args: {
    initial: {
      ...base,
      claim: {
        ...base.claim,
        spec: {
          ...base.claim.spec,
          cluster: {
            zeebe: {
              replicas: 5
            }
          }
        }
      }
    }
  },
  play: async ({
    canvasElement
  }) => {
    await expect(brokers(canvasElement).summary).toHaveTextContent("1 changed in the claim");
  }
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "Error in a section",
  play: async ({
    canvasElement
  }) => {
    const {
      group,
      summary
    } = brokers(canvasElement);
    await userEvent.click(summary);
    const replicas = within(within(group).getAllByRole("group", {
      name: "replicas"
    })[0]).getByRole("spinbutton");
    await userEvent.clear(replicas);
    await userEvent.type(replicas, "-1");
    await userEvent.click(summary);
    await expect(summary).toHaveTextContent("1 setting has an error");
  }
}`,...Z.parameters?.docs?.source}}}})))()}$();export{X as ChangedSetting,K as Defaults,Z as ErrorInSection,J as FilledByAccess,Y as NewNamespace,q as Overridden,Q as __namedExportsOrder,G as default};