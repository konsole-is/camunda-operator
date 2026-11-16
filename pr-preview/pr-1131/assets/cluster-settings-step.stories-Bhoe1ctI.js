import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{a as r,c as i,d as a,r as o,t as s,u as c}from"./draft-Do2v127u.js";import{d as l}from"./schema-AiosWN3q.js";import{t as u}from"./fields-DpU2ZCPT.js";import{n as d,r as f,t as p}from"./native-select-DYTAn_wB.js";import{a as m,i as h,n as g,s as _}from"./settings-editor-NGul5dmQ.js";import{i as v,r as y}from"./settings-Dhn6zHjJ.js";import{i as b,n as x,t as S}from"./release-options-Dlsrw3dL.js";import{n as C,t as w}from"./wizard-stepper-CtgokRYc.js";import{a as T,n as E,o as D}from"./fixtures-B-6knDVr.js";function O({draft:e,environment:t,releases:n,platformConfigs:r,fields:a,merged:o,secrets:s,pick:c,set:l,reset:u,invalid:f,unread:m=[]}){let h=(0,A.useId)(),y=i(e,t)??``,x=t?.spec.platformConfigRef,S=e.claim.spec.platformConfigRef??x??``,C=S!==``&&S===x,w=t?.spec.displayName||t?.metadata.name,T=e.claim.spec.cluster??{};return(0,k.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-6`,children:[(0,k.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[(0,k.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1`,children:[(0,k.jsx)(`label`,{htmlFor:`${h}-release`,className:`font-heading text-base font-semibold`,children:`Release`}),(0,k.jsxs)(p,{id:`${h}-release`,value:y,onChange:e=>c(`releaseRef`,e.target.value||void 0),children:[!n.some(e=>e.name===y)&&(0,k.jsx)(d,{value:y,children:m.includes(`CamundaRelease`)?`Could not read the releases`:`Pick a release`}),n.map(e=>(0,k.jsx)(d,{value:e.name,children:b(e,e.isDefault)},e.name))]})]}),(0,k.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1`,children:[(0,k.jsx)(`label`,{htmlFor:`${h}-platform`,className:`font-heading text-base font-semibold`,children:`Platform config`}),(0,k.jsxs)(p,{id:`${h}-platform`,value:S,"aria-describedby":C?`${h}-platform-note`:void 0,onChange:e=>c(`platformConfigRef`,e.target.value||void 0),children:[!r.includes(S)&&(0,k.jsx)(d,{value:S,children:m.includes(`CamundaPlatformConfig`)?`Could not read the platform configs`:`Pick a platform config`}),r.map(e=>(0,k.jsx)(d,{value:e,children:e},e))]}),C&&w&&(0,k.jsx)(`p`,{id:`${h}-platform-note`,className:`text-sm text-muted`,children:`Default from the ${w} environment.`})]})]}),v(a.filter(e=>!j.has(e.claimPath??``)),o.spec).map(e=>(0,k.jsx)(g,{group:e,settings:_({group:e,spec:o.spec,live:{},claim:T}),secrets:s,set:l,reset:u,invalid:f},e.component))]})}var k,A,j;function M(){return(M=e((()=>{k=n(),A=t(),m(),x(),h(),y(),f(),r(),j=new Set([`spec.presetRef`,`spec.releaseRef`,`spec.platformConfigRef`]),O.__docgenInfo={description:`ClusterSettingsStep is step 3 of the cluster create flow: the release,
the platform config, then the settings editors over fields, without the
fields at presetRef, releaseRef, and platformConfigRef. fields are
clusterFields of the draft cluster. A field that a step 2 option fills
shows locked with its section. unread are the kinds that could not be
read. It holds no draft: pick sets releaseRef or platformConfigRef, an
empty value unsets it, and set, reset, and invalid take steps from the
root of the claim.`,methods:[],displayName:`ClusterSettingsStep`,props:{draft:{required:!0,tsType:{name:`ClusterDraft`},description:``},environment:{required:!0,tsType:{name:`union`,raw:`Environment | undefined`,elements:[{name:`Environment`},{name:`undefined`}]},description:``},releases:{required:!0,tsType:{name:`unknown`},description:``},platformConfigs:{required:!0,tsType:{name:`unknown`},description:``},fields:{required:!0,tsType:{name:`unknown`},description:``},merged:{required:!0,tsType:{name:`Merged`,elements:[{name:`CamundaClusterSpec`}],raw:`Merged<CamundaClusterSpec>`},description:``},secrets:{required:!0,tsType:{name:`SecretRefs`},description:``},pick:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(
  field: "releaseRef" | "platformConfigRef",
  name: string | undefined,
) => void`,signature:{arguments:[{type:{name:`union`,raw:`"releaseRef" | "platformConfigRef"`,elements:[{name:`literal`,value:`"releaseRef"`},{name:`literal`,value:`"platformConfigRef"`}]},name:`field`},{type:{name:`union`,raw:`string | undefined`,elements:[{name:`string`},{name:`undefined`}]},name:`name`}],return:{name:`void`}}},description:``},set:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[], value: unknown) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`},{type:{name:`unknown`},name:`value`}],return:{name:`void`}}},description:``},reset:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[]) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`}],return:{name:`void`}}},description:``},invalid:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[], message: string | undefined) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`},{type:{name:`union`,raw:`string | undefined`,elements:[{name:`string`},{name:`undefined`}]},name:`message`}],return:{name:`void`}}},description:``},unread:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}}}}})))()}function N({initial:e,secrets:t,pick:n,set:r,reset:i}){let[u,d]=(0,F.useState)(e),f=o(u,T,R,z),p=l({cluster:f.cluster,merged:f.merged,claim:f.claim,sections:[B],registrations:[]});return(0,P.jsx)(w,{steps:[...s],current:`settings`,children:(0,P.jsx)(O,{draft:u,environment:z,releases:S(R,z),platformConfigs:[`basic`,`oidc`],fields:p,merged:f.merged,secrets:t,pick:(e,t)=>{n(e,t),d(n=>t===void 0?a(n,[{field:`spec`},{field:e}]):c(n,[{field:`spec`},{field:e}],t))},set:(e,t)=>{r(e,t),d(n=>c(n,e,t))},reset:e=>{i(e),d(t=>a(t,e))},invalid:()=>{}})})}var P,F,I,L,R,z,B,V,H,U,W,G,K,q,J;function Y(){return(Y=e((()=>{P=n(),F=t(),x(),C(),r(),u(),M(),E(),{fn:I}=__STORYBOOK_MODULE_TEST__,L=(e,t)=>({apiVersion:`core.camunda.io/v1`,kind:`CamundaRelease`,metadata:{name:e},spec:{version:t}}),R=[L(`camunda-8-8`,`8.8.4`),L(`camunda-8-9`,`8.9.9`)],z={...D,spec:{...D.spec,releases:{allowed:[`camunda-8-8`,`camunda-8-9`],default:`camunda-8-9`},platformConfigRef:`basic`}},B={apiVersion:`plugins.camunda.io/v1alpha1`,kind:`PluginSection`,metadata:{name:`access`},spec:{appliesTo:[`CamundaClusterClaim`],label:`Access`,selection:`One`,slot:`externalAccess`}},V=[{name:`orders-db`,keys:[`password`,`username`]},{name:`orders-license`,keys:[`license`]}],H={environment:`prod`,namespace:{kind:`existing`,name:`team-orders`},claim:{metadata:{name:`orders`},spec:{presetRef:`medium`}}},U={title:`create/ClusterSettingsStep`,component:N,args:{initial:H,secrets:V,pick:I(),set:I(),reset:I()},render:e=>(0,P.jsx)(N,{...e},JSON.stringify(e))},W={},G={args:{initial:{...H,claim:{...H.claim,spec:{...H.claim.spec,releaseRef:`camunda-8-8`,platformConfigRef:`oidc`,cluster:{podLabels:{team:`orders`}}}}}}},K={name:`Filled by Access`,args:{initial:{...H,storage:{access:[{section:`access`,registration:`gateway`}]}}}},q={name:`New namespace`,args:{initial:{...H,namespace:{kind:`new`,name:`team-new`,labels:{}}},secrets:[]}},J=[`Defaults`,`Overridden`,`FilledByAccess`,`NewNamespace`],W.parameters={...W.parameters,docs:{...W.parameters?.docs,source:{originalSource:`{}`,...W.parameters?.docs?.source}}},G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
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
}`,...G.parameters?.docs?.source}}},K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
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
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
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
}`,...q.parameters?.docs?.source}}}})))()}Y();export{W as Defaults,K as FilledByAccess,q as NewNamespace,G as Overridden,J as __namedExportsOrder,U as default};