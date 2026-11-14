import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,f as r}from"./labels-lxF_8UjP.js";import{a as i,i as a,n as o,t as s}from"./field-help-DAQjHzC5.js";import{c,t as l}from"./merge-B877RDyT.js";import{a as u,i as d,n as f,o as p,r as m,s as h}from"./table-DGxvrdAu.js";import{g,t as _}from"./fields-Ds_W30BQ.js";import{n as v,r as y,t as b}from"./field-state-TPIe2yan.js";import{n as x,r as S,t as C}from"./settings-D_JwDfaT.js";function w({groups:e,docs:t,operatorVersion:n,cluster:r}){let i=e.every(e=>e.fields.length===0);return(0,E.jsx)(`section`,{"aria-label":`Settings`,className:`flex min-w-0 flex-col gap-6`,children:i?(0,E.jsx)(`p`,{className:`text-sm text-muted`,children:`No settings.`}):e.map(e=>(0,E.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-2`,children:[(0,E.jsx)(`h3`,{className:`font-heading text-sm font-semibold`,children:e.embedded?`${e.label} (embedded)`:e.label}),e.fields.length===0?(0,E.jsx)(`p`,{className:`text-sm text-muted`,children:`No settings.`}):(0,E.jsxs)(`table`,{"aria-label":`${e.label} settings`,className:`w-full text-sm`,children:[(0,E.jsx)(u,{children:(0,E.jsxs)(p,{children:[(0,E.jsx)(d,{children:`Field`}),(0,E.jsx)(d,{children:`Value`}),(0,E.jsx)(d,{children:`State`})]})}),(0,E.jsx)(f,{children:e.fields.map(e=>(0,E.jsxs)(p,{children:[(0,E.jsx)(`th`,{scope:`row`,className:`px-2 py-2 text-left align-top font-normal`,children:(0,E.jsxs)(`span`,{className:`inline-flex items-center gap-1`,children:[(0,E.jsx)(`code`,{className:`text-text break-words`,children:e.label}),(0,E.jsx)(s,{label:e.label,doc:t[e.key],operatorVersion:n})]})}),(0,E.jsx)(m,{className:`max-w-80 align-top break-words whitespace-normal`,children:(0,E.jsx)(v,{value:e.value})}),(0,E.jsx)(m,{className:`align-top whitespace-normal`,children:(0,E.jsx)(b,{field:e,claimHref:T(e,r)})})]},e.key))})]})]},e.label))})}function T(e,t){return e.claimPath?C(t.namespace,t.name,e.claimPath):void 0}var E;function D(){return(D=e((()=>{E=t(),o(),y(),h(),S(),w.__docgenInfo={description:`ComponentSettings shows the fields of one component read only: each
group as a table of the field with its help, the value, and where the
value comes from, with the lock and a link to the field in the Claim tab.
docs holds the help of a field by its key.`,methods:[],displayName:`ComponentSettings`,props:{groups:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  label: string;
  component: SchemaGroup;
  embedded: boolean;
  fields: ClusterField[];
}`,signature:{properties:[{key:`label`,value:{name:`string`,required:!0}},{key:`component`,value:{name:`SchemaGroup`,required:!0}},{key:`embedded`,value:{name:`boolean`,required:!0}},{key:`fields`,value:{name:`Array`,elements:[{name:`ClusterField`}],raw:`ClusterField[]`,required:!0}}]}}],raw:`SettingsGroup[]`},description:``},docs:{required:!0,tsType:{name:`Record`,elements:[{name:`string`},{name:`FieldDoc`}],raw:`Record<string, FieldDoc>`},description:``},operatorVersion:{required:!0,tsType:{name:`string`},description:``},cluster:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{ namespace: string; name: string }`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}}]}},description:``}}}})))()}function O(e,t,r,i=!0){let o=c({cluster:j},A,t),s=g({cluster:{kind:`CamundaCluster`,metadata:{name:`orders`,namespace:`team-a`,labels:i?{[n]:`orders`}:{},managedFields:r},spec:t},merged:o,claim:{kind:`CamundaClusterClaim`,metadata:{name:`orders`,namespace:`team-a`},spec:{presetRef:`medium`,cluster:{zeebe:{replicas:5}},plugins:[{section:`volume-encryption`,registration:`aws-ebs-kms`}]}},sections:[{kind:`PluginSection`,metadata:{name:`volume-encryption`},spec:{label:`Volume encryption`,slot:`volumeEncryption`,appliesTo:[`CamundaClusterClaim`],selection:`One`}}],registrations:[]});return{groups:x(s,e,o.spec),docs:a(s),operatorVersion:`0.2.0`,cluster:{namespace:`team-a`,name:`orders`}}}var k,A,j,M,N,P,F,I,L;function R(){return(R=e((()=>{i(),_(),r(),l(),D(),S(),k=`CamundaClusterClaim/cluster/0b5c7a2e`,A={version:`8.9.23`},j={zeebe:{replicas:3,partitions:3,storageSize:`32Gi`},gateway:{replicas:2}},M=(e,t,n)=>({manager:e,time:n,fieldsV1:{"f:spec":t}}),N={title:`cluster/ComponentSettings`,component:w},P={name:`Brokers with a filled and a resized field`,args:O(`zeebe`,{zeebe:{replicas:5,storageClassName:`encrypted`,storageSize:`64Gi`},podLabels:{team:`orders`}},[M(k,{"f:zeebe":{"f:replicas":{},"f:storageClassName":{}}},`2026-10-01T00:00:00Z`),M(`pvc-autoresizer`,{"f:zeebe":{"f:storageSize":{}}},`2026-10-02T00:00:00Z`)])},F={name:`Gateway with embedded apps`,args:O(`gateway`,{gateway:{mode:`Standalone`},operate:{replicas:1,podLabels:{team:`orders`}},tasklist:{replicas:1}},[M(k,{"f:gateway":{"f:mode":{}},"f:operate":{"f:replicas":{},"f:podLabels":{"f:team":{}}},"f:tasklist":{"f:replicas":{}}},`2026-10-01T00:00:00Z`)])},I={name:`Not managed`,args:O(`zeebe`,{zeebe:{replicas:3,storageSize:`64Gi`}},[M(`kubectl-client-side-apply`,{"f:zeebe":{"f:replicas":{},"f:storageSize":{}}},`2026-10-01T00:00:00Z`)],!1)},L=[`BrokersWithAFilledAndAResizedField`,`GatewayWithEmbeddedApps`,`NotManaged`],P.parameters={...P.parameters,docs:{...P.parameters?.docs,source:{originalSource:`{
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
}`,...P.parameters?.docs?.source}}},F.parameters={...F.parameters,docs:{...F.parameters?.docs,source:{originalSource:`{
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
}`,...F.parameters?.docs?.source}}},I.parameters={...I.parameters,docs:{...I.parameters?.docs,source:{originalSource:`{
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
}`,...I.parameters?.docs?.source}}}})))()}R();export{P as BrokersWithAFilledAndAResizedField,F as GatewayWithEmbeddedApps,I as NotManaged,L as __namedExportsOrder,N as default};