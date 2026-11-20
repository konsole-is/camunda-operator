import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{a as r,c as i,d as a,r as o,t as s,u as c}from"./draft-BXud8ANW.js";import{d as l}from"./schema-CkYx6TI9.js";import{t as u}from"./fields-CnMyiCBC.js";import{n as d,r as f,t as p}from"./native-select-DCiAiF5U.js";import{i as m,n as h}from"./settings-editor-Bdippl6j.js";import{a as g,i as _}from"./settings-DiVyIVyU.js";import{a as ee,n as v}from"./editable-settings-CWPNmC2L.js";import{i as te,n as y,t as b}from"./release-options-BeYMc_QH.js";import{n as x,t as S}from"./wizard-stepper-Dnv8fLbT.js";import{a as C,n as w,o as T}from"./fixtures-BIhfPx-a.js";function E({draft:e,environment:t,releases:n,platformConfigs:r,presets:a,fields:o,merged:s,secrets:c,pick:l,set:u,reset:f,invalid:m,errors:_=[],unread:v=[]}){let y=(0,O.useId)(),b=i(e,t)??``,x=t?.spec.platformConfigRef,S=e.claim.spec.platformConfigRef??x??``,C=S!==``&&S===x,w=t?.spec.displayName||t?.metadata.name,T=e.claim.spec.cluster??{},E=e.claim.spec.presetRef,A=E!==void 0&&a.includes(E);return(0,D.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-6`,children:[(0,D.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[(0,D.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1`,children:[(0,D.jsx)(`label`,{htmlFor:`${y}-release`,className:`font-heading text-base font-semibold`,children:`Release`}),(0,D.jsxs)(p,{id:`${y}-release`,value:b,onChange:e=>l(`releaseRef`,e.target.value||void 0),children:[!n.some(e=>e.name===b)&&(0,D.jsx)(d,{value:b,children:v.includes(`CamundaRelease`)?`Could not read the releases`:`Pick a release`}),n.map(e=>(0,D.jsx)(d,{value:e.name,children:te(e,e.isDefault)},e.name))]})]}),(0,D.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1`,children:[(0,D.jsx)(`label`,{htmlFor:`${y}-platform`,className:`font-heading text-base font-semibold`,children:`Platform config`}),(0,D.jsxs)(p,{id:`${y}-platform`,value:S,"aria-describedby":C?`${y}-platform-note`:void 0,onChange:e=>l(`platformConfigRef`,e.target.value||void 0),children:[!r.includes(S)&&(0,D.jsx)(d,{value:S,children:v.includes(`CamundaPlatformConfig`)?`Could not read the platform configs`:`Pick a platform config`}),r.map(e=>(0,D.jsx)(d,{value:e,children:e},e))]}),C&&w&&(0,D.jsx)(`p`,{id:`${y}-platform-note`,className:`text-sm text-muted`,children:`Default from the ${w} environment.`})]})]}),(0,D.jsxs)(`div`,{className:`flex flex-col gap-3`,children:[A&&(0,D.jsxs)(`p`,{className:`max-w-prose text-sm text-muted`,children:[`Your admin made the preset`,` `,(0,D.jsx)(`span`,{className:`font-medium text-text`,children:E}),`. It sets each value that shows “From preset”. A change here applies to this cluster only. It does not change the preset.`]}),g(o.filter(e=>!k.has(e.claimPath??``)),s.spec).map(e=>(0,D.jsx)(h,{collapsed:!0,group:e,settings:ee({group:e,spec:s.spec,live:{},claim:T}),secrets:c,errors:_,set:u,reset:f,invalid:m},e.component))]})]})}var D,O,k;function A(){return(A=e((()=>{D=n(),O=t(),v(),y(),m(),_(),f(),r(),k=new Set([`spec.presetRef`,`spec.releaseRef`,`spec.platformConfigRef`]),E.__docgenInfo={description:`ClusterSettingsStep is step 3 of the cluster create flow: the release,
the platform config, then the settings editors over fields, without the
fields at presetRef, releaseRef, and platformConfigRef. A note names the
preset of the draft when presets, the names of the live presets, hold
it. errors are field errors at paths from the root of the claim, and
each one shows only at a setting whose claim path holds it. fields are
clusterFields of the draft cluster. A field that a step 2 option fills
shows locked with its section. unread are the kinds that could not be
read. It holds no draft: pick sets releaseRef or platformConfigRef, an
empty value unsets it, and set, reset, and invalid take steps from the
root of the claim.`,methods:[],displayName:`ClusterSettingsStep`,props:{draft:{required:!0,tsType:{name:`ClusterDraft`},description:``},environment:{required:!0,tsType:{name:`union`,raw:`Environment | undefined`,elements:[{name:`Environment`},{name:`undefined`}]},description:``},releases:{required:!0,tsType:{name:`unknown`},description:``},platformConfigs:{required:!0,tsType:{name:`unknown`},description:``},presets:{required:!0,tsType:{name:`unknown`},description:``},fields:{required:!0,tsType:{name:`unknown`},description:``},merged:{required:!0,tsType:{name:`Merged`,elements:[{name:`CamundaClusterSpec`}],raw:`Merged<CamundaClusterSpec>`},description:``},secrets:{required:!0,tsType:{name:`SecretRefs`},description:``},pick:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(
  field: "releaseRef" | "platformConfigRef",
  name: string | undefined,
) => void`,signature:{arguments:[{type:{name:`union`,raw:`"releaseRef" | "platformConfigRef"`,elements:[{name:`literal`,value:`"releaseRef"`},{name:`literal`,value:`"platformConfigRef"`}]},name:`field`},{type:{name:`union`,raw:`string | undefined`,elements:[{name:`string`},{name:`undefined`}]},name:`name`}],return:{name:`void`}}},description:``},set:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[], value: unknown) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`},{type:{name:`unknown`},name:`value`}],return:{name:`void`}}},description:``},reset:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[]) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`}],return:{name:`void`}}},description:``},invalid:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[], message: string | undefined) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`},{type:{name:`union`,raw:`string | undefined`,elements:[{name:`string`},{name:`undefined`}]},name:`message`}],return:{name:`void`}}},description:``},errors:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}},unread:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}}}}})))()}function j({initial:e,secrets:t,pick:n,set:r,reset:i,errors:u,brokerResources:d}){let[f,p]=(0,P.useState)(e),m=d===void 0?C:C.map(e=>e.metadata.name===`medium`?{...e,spec:{cluster:{...e.spec.cluster,zeebe:{...e.spec.cluster.zeebe,resources:d}}}}:e),h=o(f,m,B,V),g=l({cluster:h.cluster,merged:h.merged,claim:h.claim,sections:[H],registrations:[]});return(0,N.jsx)(S,{steps:[...s],current:`settings`,children:(0,N.jsx)(E,{draft:f,environment:V,releases:b(B,V),platformConfigs:[`basic`,`oidc`],presets:C.map(e=>e.metadata.name??``),fields:g,merged:h.merged,secrets:t,pick:(e,t)=>{n(e,t),p(n=>t===void 0?a(n,[{field:`spec`},{field:e}]):c(n,[{field:`spec`},{field:e}],t))},set:(e,t)=>{r(e,t),p(n=>c(n,e,t))},reset:e=>{i(e),p(t=>a(t,e))},invalid:()=>{},errors:u})})}function M(e){let t=R(e).getByRole(`region`,{name:`Brokers settings`});return{group:t,row:R(R(t).getByRole(`heading`,{level:3,name:`Brokers`})).getByRole(`button`)}}var N,P,F,I,L,R,z,B,V,H,U,W,G,K,q,J,Y,X,Z,Q,$,ne;function re(){return(re=e((()=>{N=n(),P=t(),y(),x(),r(),u(),A(),w(),{expect:F,fn:I,userEvent:L,within:R}=__STORYBOOK_MODULE_TEST__,z=(e,t)=>({apiVersion:`core.camunda.io/v1`,kind:`CamundaRelease`,metadata:{name:e},spec:{version:t}}),B=[z(`camunda-8-8`,`8.8.4`),z(`camunda-8-9`,`8.9.9`)],V={...T,spec:{...T.spec,releases:{allowed:[`camunda-8-8`,`camunda-8-9`],default:`camunda-8-9`},platformConfigRef:`basic`}},H={apiVersion:`plugins.camunda.io/v1alpha1`,kind:`PluginSection`,metadata:{name:`access`},spec:{appliesTo:[`CamundaClusterClaim`],label:`Access`,selection:`One`,slot:`externalAccess`}},U=[{name:`orders-db`,keys:[`password`,`username`]},{name:`orders-license`,keys:[`license`]}],W={environment:`prod`,namespace:{kind:`existing`,name:`team-orders`},claim:{metadata:{name:`orders`},spec:{presetRef:`medium`}}},G={title:`create/ClusterSettingsStep`,component:j,args:{initial:W,secrets:U,pick:I(),set:I(),reset:I()},render:e=>(0,N.jsx)(j,{...e},JSON.stringify(e))},K={play:async({canvasElement:e})=>{let{row:t}=M(e);await F(t).toHaveAttribute(`aria-expanded`,`false`),await F(t).toHaveAccessibleName(`Brokers 0 changed in the claim`)}},q={args:{initial:{...W,claim:{...W.claim,spec:{...W.claim.spec,releaseRef:`camunda-8-8`,platformConfigRef:`oidc`,cluster:{podLabels:{team:`orders`}}}}}}},J={name:`Filled by Access`,args:{initial:{...W,storage:{access:[{section:`access`,registration:`gateway`}]}}}},Y={name:`New namespace`,args:{initial:{...W,namespace:{kind:`new`,name:`team-new`,labels:{}}},secrets:[]}},X={name:`Changed setting`,args:{initial:{...W,claim:{...W.claim,spec:{...W.claim.spec,cluster:{zeebe:{replicas:5}}}}}},play:async({canvasElement:e})=>{await F(M(e).row).toHaveAccessibleName(`Brokers 1 changed in the claim`)}},Z={name:`Error in a section`,play:async({canvasElement:e})=>{let{group:t,row:n}=M(e);await L.click(n);let r=R(R(t).getAllByRole(`group`,{name:`replicas`})[0]).getByRole(`spinbutton`);await L.clear(r),await L.type(r,`-1`),await L.click(n),await F(n).toHaveAttribute(`aria-expanded`,`false`),await F(n).toHaveAccessibleName(/ 1 error$/),await F(R(n).getByText(`1 error`)).toBeVisible()}},Q={name:`Missing Secret`,args:{initial:{...W,claim:{...W.claim,spec:{...W.claim.spec,cluster:{auth:{clientSecretRef:{name:`orders-oidc`,key:`secret`}}}}}},errors:[{path:`spec.cluster.auth.clientSecretRef`,message:`The namespace team-orders holds no Secret orders-oidc. Pick a Secret of team-orders, or reset the field.`}]}},$={name:`Preset limit below the request`,args:{brokerResources:{requests:{cpu:`2`},limits:{cpu:`1`}}},play:async({canvasElement:e})=>{let{group:t,row:n}=M(e);await F(n).toHaveAttribute(`aria-expanded`,`true`),await F(n).toHaveAccessibleName(/ 1 error$/);let r=R(t).getAllByRole(`group`,{name:`resources`})[0];await F(R(r).getByRole(`alert`)).toHaveTextContent(`The CPU limit of 1 and the CPU request of 2 come from the preset. The limit is below the request.`);let i=R(R(r).getByRole(`group`,{name:`CPU limit`})).getByRole(`textbox`,{name:`Amount`});await L.clear(i),await F(R(r).getByRole(`alert`)).toHaveTextContent(`The CPU limit of 1 and the CPU request of 2 come from the preset. The limit is below the request.`),await F(i).toHaveAccessibleDescription(`The CPU limit of 1 and the CPU request of 2 come from the preset. The limit is below the request.`),await F(n).toHaveAccessibleName(/ 1 error$/)}},ne=[`Defaults`,`Overridden`,`FilledByAccess`,`NewNamespace`,`ChangedSetting`,`ErrorInSection`,`MissingSecret`,`PresetLimitBelowRequest`],K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
  play: async ({
    canvasElement
  }) => {
    const {
      row
    } = brokers(canvasElement);
    await expect(row).toHaveAttribute("aria-expanded", "false");
    await expect(row).toHaveAccessibleName("Brokers 0 changed in the claim");
  }
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
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
    await expect(brokers(canvasElement).row).toHaveAccessibleName("Brokers 1 changed in the claim");
  }
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "Error in a section",
  play: async ({
    canvasElement
  }) => {
    const {
      group,
      row
    } = brokers(canvasElement);
    await userEvent.click(row);
    const replicas = within(within(group).getAllByRole("group", {
      name: "replicas"
    })[0]).getByRole("spinbutton");
    await userEvent.clear(replicas);
    await userEvent.type(replicas, "-1");
    await userEvent.click(row);
    await expect(row).toHaveAttribute("aria-expanded", "false");
    await expect(row).toHaveAccessibleName(/ 1 error$/);
    await expect(within(row).getByText("1 error")).toBeVisible();
  }
}`,...Z.parameters?.docs?.source}}},Q.parameters={...Q.parameters,docs:{...Q.parameters?.docs,source:{originalSource:`{
  name: "Missing Secret",
  args: {
    initial: {
      ...base,
      claim: {
        ...base.claim,
        spec: {
          ...base.claim.spec,
          cluster: {
            auth: {
              clientSecretRef: {
                name: "orders-oidc",
                key: "secret"
              }
            }
          }
        }
      }
    },
    errors: [{
      path: "spec.cluster.auth.clientSecretRef",
      message: "The namespace team-orders holds no Secret orders-oidc. Pick a Secret of team-orders, or reset the field."
    }]
  }
}`,...Q.parameters?.docs?.source}}},$.parameters={...$.parameters,docs:{...$.parameters?.docs,source:{originalSource:`{
  name: "Preset limit below the request",
  args: {
    brokerResources: {
      requests: {
        cpu: "2"
      },
      limits: {
        cpu: "1"
      }
    }
  },
  play: async ({
    canvasElement
  }) => {
    const {
      group,
      row
    } = brokers(canvasElement);
    await expect(row).toHaveAttribute("aria-expanded", "true");
    await expect(row).toHaveAccessibleName(/ 1 error$/);
    const resources = within(group).getAllByRole("group", {
      name: "resources"
    })[0];
    await expect(within(resources).getByRole("alert")).toHaveTextContent("The CPU limit of 1 and the CPU request of 2 come from the preset. The limit is below the request.");
    const limit = within(within(resources).getByRole("group", {
      name: "CPU limit"
    })).getByRole("textbox", {
      name: "Amount"
    });
    // A cleared field keeps the value of the preset.
    await userEvent.clear(limit);
    await expect(within(resources).getByRole("alert")).toHaveTextContent("The CPU limit of 1 and the CPU request of 2 come from the preset. The limit is below the request.");
    await expect(limit).toHaveAccessibleDescription("The CPU limit of 1 and the CPU request of 2 come from the preset. The limit is below the request.");
    await expect(row).toHaveAccessibleName(/ 1 error$/);
  }
}`,...$.parameters?.docs?.source}}}})))()}re();export{X as ChangedSetting,K as Defaults,Z as ErrorInSection,J as FilledByAccess,Q as MissingSecret,Y as NewNamespace,q as Overridden,$ as PresetLimitBelowRequest,ne as __namedExportsOrder,G as default};