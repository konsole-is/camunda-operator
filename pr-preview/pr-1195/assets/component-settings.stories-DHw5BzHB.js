import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,x as r}from"./labels-Be1Q9KU6.js";import{a as i,i as a,n as o,t as s}from"./field-help-BTwfe7AB.js";import{a as c,o as l}from"./iframe-WoYcP6TC.js";import{E as u,t as d}from"./merge-BODiO-dm.js";import{p as f}from"./schema-CP8P9yVZ.js";import{t as p}from"./fields-DCbk8kBj.js";import{i as m,n as h,r as g,t as _}from"./field-state-Cwvr8TgX.js";import{i as v,r as y,t as b}from"./settings-DiVyIVyU.js";import{a as x,c as S,i as C,n as w,o as T,s as E}from"./table-CjGQ_c5K.js";function D({groups:e,docs:t,operatorVersion:n,cluster:r}){let i=e.every(e=>e.fields.length===0);return(0,k.jsx)(`section`,{"aria-label":`Settings`,className:`flex min-w-0 flex-col gap-6`,children:i?(0,k.jsx)(`p`,{className:`text-sm text-muted`,children:`No settings.`}):e.map(e=>(0,k.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-2`,children:[(0,k.jsx)(`h3`,{className:`font-heading text-sm font-semibold`,children:e.embedded?`${e.label} (embedded)`:e.label}),e.fields.length===0?(0,k.jsx)(`p`,{className:`text-sm text-muted`,children:`No settings.`}):(0,k.jsxs)(`table`,{"aria-label":`${e.label} settings`,className:`w-full text-sm`,children:[(0,k.jsx)(T,{children:(0,k.jsxs)(E,{className:_.row,children:[(0,k.jsx)(x,{children:`Field`}),(0,k.jsx)(x,{children:`Value`}),(0,k.jsx)(x,{className:_.stateHead,children:`State`})]})}),(0,k.jsx)(w,{children:e.fields.map(e=>(0,k.jsxs)(E,{className:_.row,children:[(0,k.jsx)(`th`,{scope:`row`,className:c(`px-2 py-2 text-left align-top font-normal`,_.field),children:(0,k.jsxs)(`span`,{className:`inline-flex items-center gap-1`,children:[(0,k.jsx)(`code`,{className:`text-text break-words`,children:e.label}),(0,k.jsx)(s,{label:e.label,doc:t[e.key],operatorVersion:n})]})}),(0,k.jsx)(C,{className:c(`max-w-80 align-top break-words whitespace-normal`,_.value),children:(0,k.jsx)(g,{value:e.value})}),(0,k.jsx)(C,{className:c(`align-top whitespace-normal`,_.state),children:(0,k.jsx)(h,{field:e,claimHref:O(e,r)})})]},e.key))})]})]},e.label))})}function O(e,t){return e.claimPath?b(t.namespace,t.name,e.claimPath):void 0}var k;function A(){return(A=e((()=>{k=t(),o(),m(),S(),l(),v(),D.__docgenInfo={description:`ComponentSettings shows the fields of one component read only: each
group as a table of the field with its help, the value, and where the
value comes from, with the lock and a link to the field in the Claim tab.
docs holds the help of a field by its key.`,methods:[],displayName:`ComponentSettings`,props:{groups:{required:!0,tsType:{name:`unknown`},description:``},docs:{required:!0,tsType:{name:`Record`,elements:[{name:`string`},{name:`FieldDoc`}],raw:`Record<string, FieldDoc>`},description:``},operatorVersion:{required:!0,tsType:{name:`string`},description:``},cluster:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{ namespace: string; name: string }`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}}]}},description:``}}}})))()}function j(e,t,r,i=!0){let o=u({cluster:P},N,t),s=f({cluster:{kind:`CamundaCluster`,metadata:{name:`orders`,namespace:`team-a`,labels:i?{[n]:`orders`}:{},managedFields:r},spec:t},merged:o,claim:{kind:`CamundaClusterClaim`,metadata:{name:`orders`,namespace:`team-a`},spec:{presetRef:`medium`,cluster:{zeebe:{replicas:5}},plugins:[{section:`volume-encryption`,registration:`aws-ebs-kms`}]}},sections:[{kind:`PluginSection`,metadata:{name:`volume-encryption`},spec:{label:`Volume encryption`,slot:`volumeEncryption`,appliesTo:[`CamundaClusterClaim`],selection:`One`}}],registrations:[]});return{groups:y(s,e,o.spec),docs:a(s),operatorVersion:`0.2.0`,cluster:{namespace:`team-a`,name:`orders`}}}var M,N,P,F,I,L,R,z,B;function V(){return(V=e((()=>{i(),p(),r(),d(),A(),v(),M=`CamundaClusterClaim/cluster/0b5c7a2e`,N={version:`8.9.23`},P={zeebe:{replicas:3,partitions:3,storageSize:`32Gi`},gateway:{replicas:2}},F=(e,t,n)=>({manager:e,time:n,fieldsV1:{"f:spec":t}}),I={title:`cluster/ComponentSettings`,component:D},L={name:`Brokers with a filled and a resized field`,args:j(`zeebe`,{zeebe:{replicas:5,storageClassName:`encrypted`,storageSize:`64Gi`},podLabels:{team:`orders`}},[F(M,{"f:zeebe":{"f:replicas":{},"f:storageClassName":{}}},`2026-10-01T00:00:00Z`),F(`pvc-autoresizer`,{"f:zeebe":{"f:storageSize":{}}},`2026-10-02T00:00:00Z`)])},R={name:`Gateway with embedded apps`,args:j(`gateway`,{gateway:{mode:`Standalone`},operate:{replicas:1,podLabels:{team:`orders`}},tasklist:{replicas:1}},[F(M,{"f:gateway":{"f:mode":{}},"f:operate":{"f:replicas":{},"f:podLabels":{"f:team":{}}},"f:tasklist":{"f:replicas":{}}},`2026-10-01T00:00:00Z`)])},z={name:`Not managed`,args:j(`zeebe`,{zeebe:{replicas:3,storageSize:`64Gi`}},[F(`kubectl-client-side-apply`,{"f:zeebe":{"f:replicas":{},"f:storageSize":{}}},`2026-10-01T00:00:00Z`)],!1)},B=[`BrokersWithAFilledAndAResizedField`,`GatewayWithEmbeddedApps`,`NotManaged`],L.parameters={...L.parameters,docs:{...L.parameters?.docs,source:{originalSource:`{
  name: "Brokers with a filled and a resized field",
  args: argsOf("zeebe", {
    zeebe: {
      replicas: 5,
      storageClassName: "encrypted",
      storageSize: "64Gi"
    },
    podLabels: {
      team: "orders"
    }
  }, [owned(CLAIM, {
    "f:zeebe": {
      "f:replicas": {},
      "f:storageClassName": {}
    }
  }, "2026-10-01T00:00:00Z"), owned("pvc-autoresizer", {
    "f:zeebe": {
      "f:storageSize": {}
    }
  }, "2026-10-02T00:00:00Z")])
}`,...L.parameters?.docs?.source}}},R.parameters={...R.parameters,docs:{...R.parameters?.docs,source:{originalSource:`{
  name: "Gateway with embedded apps",
  args: argsOf("gateway", {
    gateway: {
      mode: "Standalone"
    },
    operate: {
      replicas: 1,
      podLabels: {
        team: "orders"
      }
    },
    tasklist: {
      replicas: 1
    }
  }, [owned(CLAIM, {
    "f:gateway": {
      "f:mode": {}
    },
    "f:operate": {
      "f:replicas": {},
      "f:podLabels": {
        "f:team": {}
      }
    },
    "f:tasklist": {
      "f:replicas": {}
    }
  }, "2026-10-01T00:00:00Z")])
}`,...R.parameters?.docs?.source}}},z.parameters={...z.parameters,docs:{...z.parameters?.docs,source:{originalSource:`{
  name: "Not managed",
  args: argsOf("zeebe", {
    zeebe: {
      replicas: 3,
      storageSize: "64Gi"
    }
  }, [owned("kubectl-client-side-apply", {
    "f:zeebe": {
      "f:replicas": {},
      "f:storageSize": {}
    }
  }, "2026-10-01T00:00:00Z")], false)
}`,...z.parameters?.docs?.source}}}})))()}V();export{L as BrokersWithAFilledAndAResizedField,R as GatewayWithEmbeddedApps,z as NotManaged,B as __namedExportsOrder,I as default};