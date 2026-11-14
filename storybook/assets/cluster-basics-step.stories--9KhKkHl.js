import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{c as r,l as i}from"./iframe-BqIb-xGg.js";import{n as a,t as o}from"./wizard-stepper-Cfku0Jij.js";import{a as s,c,i as l,l as u,n as d,o as f,t as p,u as m}from"./draft-DNj_MEQf.js";import{o as h,t as g}from"./policy-EofxYXuk.js";import{n as _,t as v}from"./input-DEWa4Ywe.js";import{n as y,t as b}from"./namespace-picker-s5Fbs6rn.js";import{n as x,t as ee}from"./environment-cards-BbQdu1U3.js";import{n as te,t as ne}from"./preset-cards-DnuzPRnL.js";import{a as S,i as C,n as w,o as T,r as E,t as D}from"./fixtures-DQACfKjd.js";import{n as O,t as k}from"./use-create-flow-C1aXKmPh.js";function A(e,t,n){let r={};for(let t of e)r[t.metadata.name??``]={namespaces:0,clusters:0};let i=new Map;for(let n of t){let t=h(n,e).environment?.metadata.name;t!==void 0&&(r[t].namespaces++,i.set(n.metadata?.name??``,t))}for(let e of n){let t=i.get(e.metadata.namespace??``);t!==void 0&&r[t].clusters++}return r}function j(){return(j=e((()=>{g()})))()}function M(){return(M=e((()=>{g()})))()}function N(){return(N=e((()=>{j(),M()})))()}function P({value:e,onChange:t,errors:n,hosts:r,hostsLabel:i}){let a=(0,I.useId)(),[o,s]=(0,I.useState)(!1),c=o?n:[];return(0,F.jsxs)(`div`,{className:`flex max-w-md flex-col gap-1`,children:[(0,F.jsx)(`label`,{htmlFor:`${a}-name`,className:`font-heading text-base font-semibold`,children:`Name`}),(0,F.jsx)(`p`,{id:`${a}-hint`,className:`text-sm text-muted`,children:`Lowercase letters, digits, and '-', at most 40 characters.`}),(0,F.jsx)(v,{id:`${a}-name`,value:e,autoComplete:`off`,spellCheck:!1,"aria-invalid":c.length>0||void 0,"aria-describedby":c.length>0?`${a}-hint ${a}-errors`:`${a}-hint`,onChange:e=>{s(!0),t?.(e.target.value)}}),c.length>0&&(0,F.jsx)(`ul`,{id:`${a}-errors`,className:`flex flex-col gap-0.5 text-xs text-bad`,children:c.map(e=>(0,F.jsx)(`li`,{children:e},e))}),r.length>0&&(0,F.jsxs)(`div`,{className:`mt-1 text-xs text-muted`,children:[(0,F.jsx)(`p`,{children:i}),(0,F.jsx)(`ul`,{className:`font-mono text-text-2`,children:r.map(e=>(0,F.jsx)(`li`,{children:e.host},e.parameter))})]})]})}var F,I;function L(){return(L=e((()=>{F=n(),I=t(),_(),m(),P.__docgenInfo={description:`ClaimNameField is the name of a new claim, with hostsLabel and the hosts
that the name gives under it. errors show only after the first edit, so an
empty form does not open with an error.`,methods:[],displayName:`ClaimNameField`,props:{value:{required:!0,tsType:{name:`string`},description:``},onChange:{required:!1,tsType:{name:`signature`,type:`function`,raw:`(name: string) => void`,signature:{arguments:[{type:{name:`string`},name:`name`}],return:{name:`void`}}},description:``},errors:{required:!0,tsType:{name:`unknown`},description:``},hosts:{required:!0,tsType:{name:`unknown`},description:``},hostsLabel:{required:!0,tsType:{name:`string`},description:``}}}})))()}function R({draft:e,set:t,environments:n,counts:r,namespaces:i,presets:a,usage:o,nameErrors:s,hosts:l}){let u=n.find(t=>t.metadata.name===e.environment),d=n.length>0&&!u,p=e.claim.spec.presetRef,m=c(e.namespace,u);return(0,H.useEffect)(()=>{m!==e.namespace&&t([`namespace`],m)}),(0,V.jsxs)(`div`,{className:`flex max-w-3xl min-w-0 flex-col gap-8`,children:[n.length>0&&(0,V.jsx)(ee,{environments:n,counts:r,value:e.environment,onChange:r=>{if(r===e.environment)return;t([`environment`],r),t([`namespace`],void 0),t([`storage`],void 0),t([`claim`,`spec`,`deletionPolicy`],void 0);let i=n.find(e=>e.metadata.name===r);p&&!(i?.spec.presets?.camundaCluster??[]).includes(p)&&t([`claim`,`spec`,`presetRef`],void 0)}}),(0,V.jsx)(P,{value:e.claim.metadata.name,errors:s,hosts:l,hostsLabel:`The cluster gets these hosts:`,onChange:e=>t([`claim`,`metadata`,`name`],e)}),d?(0,V.jsx)(z,{title:`Namespace`,children:(0,V.jsx)(B,{})}):(0,V.jsx)(b,{namespaces:i.map(e=>({name:e.metadata?.name??``,environment:h(e,n).environment?.metadata.name})),environment:u&&f(u),defaultValue:e.namespace,onChange:(e,n)=>t([`namespace`],n.length?void 0:e)},e.environment??``),(0,V.jsx)(z,{title:`Preset`,children:d?(0,V.jsx)(B,{}):u&&a.length===0?(0,V.jsx)(`p`,{className:`text-sm text-muted`,children:`The environment ${u.spec.displayName||u.metadata.name} allows no cluster preset. Ask your admin to add one.`}):(0,V.jsx)(ne,{presets:a,usage:o,value:p,onChange:e=>t([`claim`,`spec`,`presetRef`],e)})})]})}function z({title:e,children:t}){return(0,V.jsxs)(`section`,{className:`flex min-w-0 flex-col gap-3`,children:[(0,V.jsx)(`h2`,{className:`font-heading text-base font-semibold`,children:e}),t]})}function B(){return(0,V.jsx)(`p`,{className:`text-sm text-muted`,children:`Choose an environment first.`})}var V,H;function U(){return(U=e((()=>{V=n(),H=t(),y(),l(),g(),L(),x(),te(),R.__docgenInfo={description:`ClusterBasicsStep is step 1 of the cluster create flow: the environment
(only while an Environment exists), the name, the namespace, and the
preset. presets are the presets that the picked environment offers.
nameErrors are the errors of the name and of its hosts. A new environment
clears the namespace, the step 2 picks, and the preset when the new
environment does not offer it. Each part restores its value from draft
when the step mounts again.`,methods:[],displayName:`ClusterBasicsStep`,props:{draft:{required:!0,tsType:{name:`ClusterDraft`},description:``},set:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(path: DraftPath, value: unknown) => void`,signature:{arguments:[{type:{name:`unknown`},name:`path`},{type:{name:`unknown`},name:`value`}],return:{name:`void`}}},description:``},environments:{required:!0,tsType:{name:`unknown`},description:``},counts:{required:!0,tsType:{name:`Readonly`,elements:[{name:`Record`,elements:[{name:`string`},{name:`EnvironmentCount`}],raw:`Record<string, EnvironmentCount>`}],raw:`Readonly<Record<string, EnvironmentCount>>`},description:``},namespaces:{required:!0,tsType:{name:`unknown`},description:``},presets:{required:!0,tsType:{name:`unknown`},description:``},usage:{required:!0,tsType:{name:`ReadonlyMap`,elements:[{name:`string`},{name:`Array`,elements:[{name:`string`}],raw:`string[]`}],raw:`ReadonlyMap<string, string[]>`},description:``},nameErrors:{required:!0,tsType:{name:`unknown`},description:``},hosts:{required:!0,tsType:{name:`unknown`},description:``}}}})))()}function W({environments:e,namespaces:t,presets:n,usage:r={},initial:i,hosts:a=[],conflicts:c=[],set:l}){let f=t=>s(n,e.find(e=>e.metadata.name===t.environment)),m=O({steps:p.map(n=>n.id===`basics`?{...n,check:n=>d(n,{environments:e,namespaces:t.map(t=>({name:t.metadata?.name??``,environment:h(t,e).environment?.metadata.name})),takenNames:[],conflicts:c,presets:f(n).map(e=>e.metadata.name??``)})}:n),initial:i});return(0,G.jsx)(o,{steps:p.map(e=>({...e,errors:m.errors[e.id]})),current:m.current,children:(0,G.jsx)(R,{draft:m.draft,set:(e,t)=>{l(e,t),m.set(e,t)},environments:e,counts:A(e,t,[]),namespaces:t,presets:f(m.draft),usage:new Map(Object.entries(r)),nameErrors:[...u(m.draft.claim.metadata.name,[]),...c],hosts:a})})}var G,K,q,J,Y,X,Z,Q;function $(){return($=e((()=>{G=n(),a(),m(),l(),i(),g(),N(),U(),w(),k(),{fn:K}=__STORYBOOK_MODULE_TEST__,q=[{metadata:{name:`team-payments`,labels:{[r]:`prod`}}},{metadata:{name:`team-orders`,labels:{[r]:`prod`}}},{metadata:{name:`sandbox`,labels:{[r]:`dev`}}}],J={title:`create/ClusterBasicsStep`,component:W,args:{environments:[T,D,E],namespaces:q,presets:S,usage:C,initial:{environment:`prod`,namespace:{kind:`existing`,name:`team-orders`},claim:{metadata:{name:`search`},spec:{presetRef:`medium`}}},set:K()},render:e=>(0,G.jsx)(W,{...e},JSON.stringify(e))},Y={name:`Production picked`,args:{hosts:[{parameter:`hostnamePattern`,host:`search.team-orders.example.com`},{parameter:`grpcHostnamePattern`,host:`grpc-search.team-orders.example.com`}]}},X={name:`Taken host`,args:{environments:[T,D],initial:{environment:`prod`,namespace:{kind:`existing`,name:`team-orders`},claim:{metadata:{name:`billing`},spec:{presetRef:`small`}}},hosts:[{parameter:`hostnamePattern`,host:`billing.team-orders.example.com`}],conflicts:[`The host billing.team-orders.example.com is taken by team-payments/billing. Choose another name.`]}},Z={name:`No environment`,args:{environments:[],initial:{claim:{metadata:{name:``},spec:{}}}}},Q=[`ProductionPicked`,`TakenHost`,`NoEnvironment`],Y.parameters={...Y.parameters,docs:{...Y.parameters?.docs,source:{originalSource:`{
  name: "Production picked",
  args: {
    hosts: [{
      parameter: "hostnamePattern",
      host: "search.team-orders.example.com"
    }, {
      parameter: "grpcHostnamePattern",
      host: "grpc-search.team-orders.example.com"
    }]
  }
}`,...Y.parameters?.docs?.source}}},X.parameters={...X.parameters,docs:{...X.parameters?.docs,source:{originalSource:`{
  name: "Taken host",
  args: {
    environments: [production, development],
    initial: {
      environment: "prod",
      namespace: {
        kind: "existing",
        name: "team-orders"
      },
      claim: {
        metadata: {
          name: "billing"
        },
        spec: {
          presetRef: "small"
        }
      }
    },
    hosts: [{
      parameter: "hostnamePattern",
      host: "billing.team-orders.example.com"
    }],
    conflicts: ["The host billing.team-orders.example.com is taken by team-payments/billing. Choose another name."]
  }
}`,...X.parameters?.docs?.source}}},Z.parameters={...Z.parameters,docs:{...Z.parameters?.docs,source:{originalSource:`{
  name: "No environment",
  args: {
    environments: [],
    initial: {
      claim: {
        metadata: {
          name: ""
        },
        spec: {}
      }
    }
  }
}`,...Z.parameters?.docs?.source}}}})))()}$();export{Z as NoEnvironment,Y as ProductionPicked,X as TakenHost,Q as __namedExportsOrder,J as default};