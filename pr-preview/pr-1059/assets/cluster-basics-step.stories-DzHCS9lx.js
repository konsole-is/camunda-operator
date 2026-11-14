import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{c as r,h as i}from"./labels-a4ptf10d.js";import{t as a,u as o}from"./policy-UrtrXbln.js";import{a as s,c,i as l,l as u,n as d,o as f,t as p,u as m}from"./draft-W6Wspu-a.js";import{n as h,t as g}from"./wizard-stepper-msPSPme8.js";import{i as _,n as v,r as y,t as b}from"./claim-name-field-C3-_1rdw.js";import{n as x,t as S}from"./namespace-picker-CkpnYYVv.js";import{n as C,t as w}from"./environment-cards-CpHEaXVU.js";import{n as T,t as E}from"./preset-cards-DB9LYto6.js";import{a as D,i as O,n as k,o as A,r as j,t as M}from"./fixtures-BTwPClnx.js";import{n as N,t as P}from"./use-create-flow-C1aXKmPh.js";function F({draft:e,set:t,environments:n,counts:r,namespaces:i,presets:a,usage:s,nameErrors:l,hosts:u}){let d=n.find(t=>t.metadata.name===e.environment),p=n.length>0&&!d,m=e.claim.spec.presetRef,h=c(e.namespace,d);return(0,z.useEffect)(()=>{h!==e.namespace&&t([`namespace`],h)}),(0,R.jsxs)(`div`,{className:`flex max-w-3xl min-w-0 flex-col gap-8`,children:[n.length>0&&(0,R.jsx)(w,{environments:n,counts:r,value:e.environment,onChange:r=>{if(r===e.environment)return;t([`environment`],r),t([`namespace`],void 0),t([`storage`],void 0),t([`claim`,`spec`,`deletionPolicy`],void 0);let i=n.find(e=>e.metadata.name===r);m&&!(i?.spec.presets?.camundaCluster??[]).includes(m)&&t([`claim`,`spec`,`presetRef`],void 0)}}),(0,R.jsx)(b,{value:e.claim.metadata.name,errors:l,hosts:u,hostsLabel:`The cluster gets these hosts:`,onChange:e=>t([`claim`,`metadata`,`name`],e)}),p?(0,R.jsx)(I,{title:`Namespace`,children:(0,R.jsx)(L,{})}):(0,R.jsx)(S,{namespaces:i.map(e=>({name:e.metadata?.name??``,environment:o(e,n).environment?.metadata.name})),environment:d&&f(d),defaultValue:e.namespace,onChange:(e,n)=>t([`namespace`],n.length?void 0:e)},e.environment??``),(0,R.jsx)(I,{title:`Preset`,children:p?(0,R.jsx)(L,{}):d&&a.length===0?(0,R.jsx)(`p`,{className:`text-sm text-muted`,children:`The environment ${d.spec.displayName||d.metadata.name} allows no cluster preset. Ask your admin to add one.`}):(0,R.jsx)(E,{presets:a,usage:s,value:m,onChange:e=>t([`claim`,`spec`,`presetRef`],e)})})]})}function I({title:e,children:t}){return(0,R.jsxs)(`section`,{className:`flex min-w-0 flex-col gap-3`,children:[(0,R.jsx)(`h2`,{className:`font-heading text-base font-semibold`,children:e}),t]})}function L(){return(0,R.jsx)(`p`,{className:`text-sm text-muted`,children:`Choose an environment first.`})}var R,z;function B(){return(B=e((()=>{R=n(),z=t(),x(),l(),a(),v(),C(),T(),F.__docgenInfo={description:`ClusterBasicsStep is step 1 of the cluster create flow: the environment
(only while an Environment exists), the name, the namespace, and the
preset. presets are the presets that the picked environment offers.
nameErrors are the errors of the name and of its hosts. A new environment
clears the namespace, the step 2 picks, and the preset when the new
environment does not offer it. Each part restores its value from draft
when the step mounts again.`,methods:[],displayName:`ClusterBasicsStep`,props:{draft:{required:!0,tsType:{name:`ClusterDraft`},description:``},set:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(path: DraftPath, value: unknown) => void`,signature:{arguments:[{type:{name:`unknown`},name:`path`},{type:{name:`unknown`},name:`value`}],return:{name:`void`}}},description:``},environments:{required:!0,tsType:{name:`unknown`},description:``},counts:{required:!0,tsType:{name:`Readonly`,elements:[{name:`Record`,elements:[{name:`string`},{name:`EnvironmentCount`}],raw:`Record<string, EnvironmentCount>`}],raw:`Readonly<Record<string, EnvironmentCount>>`},description:``},namespaces:{required:!0,tsType:{name:`unknown`},description:``},presets:{required:!0,tsType:{name:`unknown`},description:``},usage:{required:!0,tsType:{name:`ReadonlyMap`,elements:[{name:`string`},{name:`Array`,elements:[{name:`string`}],raw:`string[]`}],raw:`ReadonlyMap<string, string[]>`},description:``},nameErrors:{required:!0,tsType:{name:`unknown`},description:``},hosts:{required:!0,tsType:{name:`unknown`},description:``}}}})))()}function V({environments:e,namespaces:t,presets:n,usage:r={},initial:i,hosts:a=[],conflicts:c=[],set:l}){let f=t=>s(n,e.find(e=>e.metadata.name===t.environment)),m=N({steps:p.map(n=>n.id===`basics`?{...n,check:n=>d(n,{environments:e,namespaces:t.map(t=>({name:t.metadata?.name??``,environment:o(t,e).environment?.metadata.name})),takenNames:[],conflicts:c,presets:f(n).map(e=>e.metadata.name??``)})}:n),initial:i});return(0,H.jsx)(g,{steps:p.map(e=>({...e,errors:m.errors[e.id]})),current:m.current,children:(0,H.jsx)(F,{draft:m.draft,set:(e,t)=>{l(e,t),m.set(e,t)},environments:e,counts:_(e,t,[]),namespaces:t,presets:f(m.draft),usage:new Map(Object.entries(r)),nameErrors:[...u(m.draft.claim.metadata.name,[]),...c],hosts:a})})}var H,U,W,G,K,q,J,Y;function X(){return(X=e((()=>{H=n(),h(),m(),l(),i(),a(),y(),B(),k(),P(),{fn:U}=__STORYBOOK_MODULE_TEST__,W=[{metadata:{name:`team-payments`,labels:{[r]:`prod`}}},{metadata:{name:`team-orders`,labels:{[r]:`prod`}}},{metadata:{name:`sandbox`,labels:{[r]:`dev`}}}],G={title:`create/ClusterBasicsStep`,component:V,args:{environments:[A,M,j],namespaces:W,presets:D,usage:O,initial:{environment:`prod`,namespace:{kind:`existing`,name:`team-orders`},claim:{metadata:{name:`search`},spec:{presetRef:`medium`}}},set:U()},render:e=>(0,H.jsx)(V,{...e},JSON.stringify(e))},K={name:`Production picked`,args:{hosts:[{parameter:`hostnamePattern`,host:`search.team-orders.example.com`},{parameter:`grpcHostnamePattern`,host:`grpc-search.team-orders.example.com`}]}},q={name:`Taken host`,args:{environments:[A,M],initial:{environment:`prod`,namespace:{kind:`existing`,name:`team-orders`},claim:{metadata:{name:`billing`},spec:{presetRef:`small`}}},hosts:[{parameter:`hostnamePattern`,host:`billing.team-orders.example.com`}],conflicts:[`The host billing.team-orders.example.com is taken by team-payments/billing. Choose another name.`]}},J={name:`No environment`,args:{environments:[],initial:{claim:{metadata:{name:``},spec:{}}}}},Y=[`ProductionPicked`,`TakenHost`,`NoEnvironment`],K.parameters={...K.parameters,docs:{...K.parameters?.docs,source:{originalSource:`{
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
}`,...K.parameters?.docs?.source}}},q.parameters={...q.parameters,docs:{...q.parameters?.docs,source:{originalSource:`{
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
}`,...q.parameters?.docs?.source}}},J.parameters={...J.parameters,docs:{...J.parameters?.docs,source:{originalSource:`{
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
}`,...J.parameters?.docs?.source}}}})))()}X();export{J as NoEnvironment,K as ProductionPicked,q as TakenHost,Y as __namedExportsOrder,G as default};