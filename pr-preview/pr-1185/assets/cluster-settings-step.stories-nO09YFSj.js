import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{a as r,c as i,d as a,r as o,t as s,u as c}from"./draft-DanjNQqF.js";import{d as l}from"./schema-CkYx6TI9.js";import{t as u}from"./fields-um0_h34w.js";import{n as d,r as f,t as p}from"./native-select-CrJ_ExCD.js";import{i as m,n as h}from"./settings-editor-BHgn6GAR.js";import{a as g,i as ee}from"./settings-DiVyIVyU.js";import{a as te,n as _}from"./editable-settings-BykUmLK-.js";import{i as ne,n as v,t as y}from"./release-options-BeYMc_QH.js";import{n as b,t as x}from"./wizard-stepper-DLeOthYR.js";import{a as S,n as C,o as w}from"./fixtures-BIhfPx-a.js";function T({draft:e,environment:t,releases:n,platformConfigs:r,presets:a,fields:o,merged:s,secrets:c,pick:l,set:u,reset:f,invalid:m,errors:ee=[],unread:_=[]}){let v=(0,D.useId)(),y=i(e,t)??``,b=t?.spec.platformConfigRef,x=e.claim.spec.platformConfigRef??b??``,S=x!==``&&x===b,C=t?.spec.displayName||t?.metadata.name,w=e.claim.spec.cluster??{},T=e.claim.spec.presetRef,k=T!==void 0&&a.includes(T);return(0,E.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-6`,children:[(0,E.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[(0,E.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1`,children:[(0,E.jsx)(`label`,{htmlFor:`${v}-release`,className:`font-heading text-base font-semibold`,children:`Release`}),(0,E.jsxs)(p,{id:`${v}-release`,value:y,onChange:e=>l(`releaseRef`,e.target.value||void 0),children:[!n.some(e=>e.name===y)&&(0,E.jsx)(d,{value:y,children:_.includes(`CamundaRelease`)?`Could not read the releases`:`Pick a release`}),n.map(e=>(0,E.jsx)(d,{value:e.name,children:ne(e,e.isDefault)},e.name))]})]}),(0,E.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1`,children:[(0,E.jsx)(`label`,{htmlFor:`${v}-platform`,className:`font-heading text-base font-semibold`,children:`Platform config`}),(0,E.jsxs)(p,{id:`${v}-platform`,value:x,"aria-describedby":S?`${v}-platform-note`:void 0,onChange:e=>l(`platformConfigRef`,e.target.value||void 0),children:[!r.includes(x)&&(0,E.jsx)(d,{value:x,children:_.includes(`CamundaPlatformConfig`)?`Could not read the platform configs`:`Pick a platform config`}),r.map(e=>(0,E.jsx)(d,{value:e,children:e},e))]}),S&&C&&(0,E.jsx)(`p`,{id:`${v}-platform-note`,className:`text-sm text-muted`,children:`Default from the ${C} environment.`})]})]}),(0,E.jsxs)(`div`,{className:`flex flex-col gap-3`,children:[k&&(0,E.jsxs)(`p`,{className:`max-w-prose text-sm text-muted`,children:[`Your admin made the preset`,` `,(0,E.jsx)(`span`,{className:`font-medium text-text`,children:T}),`. It sets each value that shows “From preset”. A change here applies to this cluster only. It does not change the preset.`]}),g(o.filter(e=>!O.has(e.claimPath??``)),s.spec).map(e=>(0,E.jsx)(h,{collapsed:!0,group:e,settings:te({group:e,spec:s.spec,live:{},claim:w}),secrets:c,errors:ee,set:u,reset:f,invalid:m},e.component))]})]})}var E,D,O;function k(){return(k=e((()=>{E=n(),D=t(),_(),v(),m(),ee(),f(),r(),O=new Set([`spec.presetRef`,`spec.releaseRef`,`spec.platformConfigRef`]),T.__docgenInfo={description:`ClusterSettingsStep is step 3 of the cluster create flow: the release,
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
) => void`,signature:{arguments:[{type:{name:`union`,raw:`"releaseRef" | "platformConfigRef"`,elements:[{name:`literal`,value:`"releaseRef"`},{name:`literal`,value:`"platformConfigRef"`}]},name:`field`},{type:{name:`union`,raw:`string | undefined`,elements:[{name:`string`},{name:`undefined`}]},name:`name`}],return:{name:`void`}}},description:``},set:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[], value: unknown) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`},{type:{name:`unknown`},name:`value`}],return:{name:`void`}}},description:``},reset:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[]) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`}],return:{name:`void`}}},description:``},invalid:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[], message: string | undefined) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`},{type:{name:`union`,raw:`string | undefined`,elements:[{name:`string`},{name:`undefined`}]},name:`message`}],return:{name:`void`}}},description:``},errors:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}},unread:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}}}}})))()}function A({initial:e,secrets:t,pick:n,set:r,reset:i,errors:u,presetBrokers:d}){let[f,p]=(0,N.useState)(e),m=d===void 0?S:S.map(e=>e.metadata.name===`medium`?{...e,spec:{cluster:{...e.spec.cluster,zeebe:{...e.spec.cluster.zeebe,...d}}}}:e),h=o(f,m,z,B),g=l({cluster:h.cluster,merged:h.merged,claim:h.claim,sections:[V],registrations:[]});return(0,M.jsx)(x,{steps:[...s],current:`settings`,children:(0,M.jsx)(T,{draft:f,environment:B,releases:y(z,B),platformConfigs:[`basic`,`oidc`],presets:S.map(e=>e.metadata.name??``),fields:g,merged:h.merged,secrets:t,pick:(e,t)=>{n(e,t),p(n=>t===void 0?a(n,[{field:`spec`},{field:e}]):c(n,[{field:`spec`},{field:e}],t))},set:(e,t)=>{r(e,t),p(n=>c(n,e,t))},reset:e=>{i(e),p(t=>a(t,e))},invalid:()=>{},errors:u})})}function j(e){let t=L(e).getByRole(`region`,{name:`Brokers settings`});return{group:t,row:L(L(t).getByRole(`heading`,{level:3,name:`Brokers`})).getByRole(`button`)}}var M,N,P,F,I,L,R,z,B,V,H,U,W,G,K,q,J,Y,X,Z,Q,$,re;function ie(){return(ie=e((()=>{M=n(),N=t(),v(),b(),r(),u(),k(),C(),{expect:P,fn:F,userEvent:I,within:L}=__STORYBOOK_MODULE_TEST__,R=(e,t)=>({apiVersion:`core.camunda.io/v1`,kind:`CamundaRelease`,metadata:{name:e},spec:{version:t}}),z=[R(`camunda-8-8`,`8.8.4`),R(`camunda-8-9`,`8.9.9`)],B={...w,spec:{...w.spec,releases:{allowed:[`camunda-8-8`,`camunda-8-9`],default:`camunda-8-9`},platformConfigRef:`basic`}},V={apiVersion:`plugins.camunda.io/v1alpha1`,kind:`PluginSection`,metadata:{name:`access`},spec:{appliesTo:[`CamundaClusterClaim`],label:`Access`,selection:`One`,slot:`externalAccess`}},H=[{name:`orders-db`,keys:[`password`,`username`]},{name:`orders-license`,keys:[`license`]}],U={environment:`prod`,namespace:{kind:`existing`,name:`team-orders`},claim:{metadata:{name:`orders`},spec:{presetRef:`medium`}}},W={title:`create/ClusterSettingsStep`,component:A,args:{initial:U,secrets:H,pick:F(),set:F(),reset:F()},render:e=>(0,M.jsx)(A,{...e},JSON.stringify(e))},G={play:async({canvasElement:e})=>{let{row:t}=j(e);await P(t).toHaveAttribute(`aria-expanded`,`false`),await P(t).toHaveAccessibleName(`Brokers 0 changed in the claim`)}},K={args:{initial:{...U,claim:{...U.claim,spec:{...U.claim.spec,releaseRef:`camunda-8-8`,platformConfigRef:`oidc`,cluster:{podLabels:{team:`orders`}}}}}}},q={name:`Filled by Access`,args:{initial:{...U,storage:{access:[{section:`access`,registration:`gateway`}]}}}},J={name:`New namespace`,args:{initial:{...U,namespace:{kind:`new`,name:`team-new`,labels:{}}},secrets:[]}},Y={name:`Changed setting`,args:{initial:{...U,claim:{...U.claim,spec:{...U.claim.spec,cluster:{zeebe:{replicas:5}}}}}},play:async({canvasElement:e})=>{await P(j(e).row).toHaveAccessibleName(`Brokers 1 changed in the claim`)}},X={name:`Error in a section`,play:async({canvasElement:e})=>{let{group:t,row:n}=j(e);await I.click(n);let r=L(L(t).getAllByRole(`group`,{name:`replicas`})[0]).getByRole(`spinbutton`);await I.clear(r),await I.type(r,`-1`),await I.click(n),await P(n).toHaveAttribute(`aria-expanded`,`false`),await P(n).toHaveAccessibleName(/ 1 error$/),await P(L(n).getByText(`1 error`)).toBeVisible()}},Z={name:`Missing Secret`,args:{initial:{...U,claim:{...U.claim,spec:{...U.claim.spec,cluster:{auth:{clientSecretRef:{name:`orders-oidc`,key:`secret`}}}}}},errors:[{path:`spec.cluster.auth.clientSecretRef`,message:`The namespace team-orders holds no Secret orders-oidc. Pick a Secret of team-orders, or reset the field.`}]}},Q={name:`Preset limit below the request`,args:{presetBrokers:{resources:{requests:{cpu:`2`},limits:{cpu:`1`}}}},play:async({canvasElement:e})=>{let{group:t,row:n}=j(e);await P(n).toHaveAttribute(`aria-expanded`,`true`),await P(n).toHaveAccessibleName(/ 1 error$/);let r=L(t).getAllByRole(`group`,{name:`resources`})[0];await P(L(r).getByRole(`alert`)).toHaveTextContent(`The CPU limit of 1 and the CPU request of 2 come from the preset. The limit is below the request.`);let i=L(L(r).getByRole(`group`,{name:`CPU limit`})).getByRole(`textbox`,{name:`Amount`});await I.clear(i),await P(L(r).getByRole(`alert`)).toHaveTextContent(`The CPU limit of 1 comes from the preset and is below the CPU request of 2.`),await P(i).toHaveAccessibleDescription(`The CPU limit of 1 comes from the preset and is below the CPU request of 2.`),await P(n).toHaveAccessibleName(/ 1 error$/)}},$={name:`Preset label key that is not valid`,args:{presetBrokers:{podLabels:{"bad key":`x`,team:`payments`}}},play:async({canvasElement:e})=>{let{group:t,row:n}=j(e);await P(n).toHaveAttribute(`aria-expanded`,`true`),await P(n).toHaveAccessibleName(/ 1 error$/);let r=L(t).getAllByRole(`group`,{name:`podLabels`})[0],i=`The label key "bad key" comes from the preset. The claim cannot remove it. Pick another preset, or ask your admin to correct the preset.`;await P(L(r).getByRole(`alert`)).toHaveTextContent(i),await I.click(L(r).getByRole(`button`,{name:`Remove entry 1`})),await P(L(r).getByRole(`alert`)).toHaveTextContent(i),await P(n).toHaveAccessibleName(/ 1 error$/)}},re=[`Defaults`,`Overridden`,`FilledByAccess`,`NewNamespace`,`ChangedSetting`,`ErrorInSection`,`MissingSecret`,`PresetLimitBelowRequest`,`PresetLabelKeyNotValid`],G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
  play: async ({
    canvasElement
  }) => {
    const {
      row
    } = brokers(canvasElement);
    await expect(row).toHaveAttribute("aria-expanded", "false");
    await expect(row).toHaveAccessibleName("Brokers 0 changed in the claim");
  }
}`,...G.parameters?.docs?.source}}},K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
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
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
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
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
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
}`,...J.parameters?.docs?.source}}},Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
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
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
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
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
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
}`,...Z.parameters?.docs?.source}}},Q.parameters={...Q.parameters,docs:{...Q.parameters?.docs,source:{originalSource:`{
  name: "Preset limit below the request",
  args: {
    presetBrokers: {
      resources: {
        requests: {
          cpu: "2"
        },
        limits: {
          cpu: "1"
        }
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
    await userEvent.clear(limit);
    await expect(within(resources).getByRole("alert")).toHaveTextContent("The CPU limit of 1 comes from the preset and is below the CPU request of 2.");
    await expect(limit).toHaveAccessibleDescription("The CPU limit of 1 comes from the preset and is below the CPU request of 2.");
    await expect(row).toHaveAccessibleName(/ 1 error$/);
  }
}`,...Q.parameters?.docs?.source}}},$.parameters={...$.parameters,docs:{...$.parameters?.docs,source:{originalSource:`{
  name: "Preset label key that is not valid",
  args: {
    presetBrokers: {
      podLabels: {
        "bad key": "x",
        team: "payments"
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
    const labels = within(group).getAllByRole("group", {
      name: "podLabels"
    })[0];
    const message = 'The label key "bad key" comes from the preset. The claim cannot remove it. Pick another preset, or ask your admin to correct the preset.';
    await expect(within(labels).getByRole("alert")).toHaveTextContent(message);
    await userEvent.click(within(labels).getByRole("button", {
      name: "Remove entry 1"
    }));
    await expect(within(labels).getByRole("alert")).toHaveTextContent(message);
    await expect(row).toHaveAccessibleName(/ 1 error$/);
  }
}`,...$.parameters?.docs?.source}}}})))()}ie();export{Y as ChangedSetting,G as Defaults,X as ErrorInSection,q as FilledByAccess,Z as MissingSecret,J as NewNamespace,K as Overridden,$ as PresetLabelKeyNotValid,Q as PresetLimitBelowRequest,re as __namedExportsOrder,W as default};