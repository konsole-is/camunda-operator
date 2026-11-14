import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{h as r}from"./labels-a4ptf10d.js";import{d as i}from"./release-B_s19T_Z.js";import{t as a}from"./policy-UrtrXbln.js";import{i as o,r as s,t as c}from"./draft-W6Wspu-a.js";import{a as l,i as u,n as d,r as f}from"./storage-BNr6uexw.js";import{i as p,n as m}from"./deletion-choice-CnaYvh35.js";import{n as h,t as g}from"./wizard-stepper-C7-Rj3aS.js";import{n as _,o as v}from"./fixtures-BTwPClnx.js";import{n as y,t as b}from"./use-create-flow-C1aXKmPh.js";import{n as x,t as S}from"./section-cards-59Q5l1ek.js";function C({draft:e,views:t,registrations:n,sections:r,context:a,hosts:o,conflicts:c,onSection:l,deletion:u,onDeletion:d}){let f=s(e);return(0,T.jsxs)(`div`,{className:`flex max-w-3xl min-w-0 flex-col gap-8`,children:[t.map(t=>{let s=e.storage&&Object.hasOwn(e.storage,t.name),u=s?e.storage[t.name]:[],d=u.flatMap(e=>{let t=n.find(t=>t.metadata.name===e.registration),o=t&&i(t,f,r,a);return o?[o]:[]});return(0,T.jsx)(w,{view:t,defaultValue:s?u:void 0,notes:d,hosts:t.name===`access`?o:[],conflicts:t.name===`access`?c:[],onChange:(e,n)=>l(t.name,e,n)},t.name)}),(0,T.jsx)(m,{subject:`cluster`,choice:u,onChange:d})]})}function w({view:e,defaultValue:t,notes:n,hosts:r,conflicts:i,onChange:a}){let o=JSON.stringify(e),s=(0,E.useMemo)(()=>JSON.parse(o),[o]),c=s.selection===`Many`||s.required||!s.emptyOptionLabel;return(0,T.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-3`,children:[(0,T.jsx)(S,{section:s,defaultValue:t,onChange:a}),s.options.length===0&&c&&(0,T.jsx)(`p`,{className:`text-sm text-muted`,children:`Your admin allows no option here.`}),n.map(e=>(0,T.jsx)(`p`,{className:`text-sm text-text-2`,children:e},e)),r.length>0&&(0,T.jsxs)(`div`,{className:`text-xs text-muted`,children:[(0,T.jsx)(`p`,{children:`The cluster gets these hosts:`}),(0,T.jsx)(`ul`,{className:`font-mono text-text-2`,children:r.map(e=>(0,T.jsx)(`li`,{children:e.host},e.parameter))})]}),i.length>0&&(0,T.jsx)(`ul`,{role:`alert`,className:`flex flex-col gap-0.5 text-xs text-bad`,children:i.map(e=>(0,T.jsx)(`li`,{children:e},e))})]})}var T,E;function D(){return(D=e((()=>{T=n(),E=t(),x(),o(),r(),a(),p(),C.__docgenInfo={description:`ClusterStorageStep is step 2 of the cluster create flow: one SectionCards
for each view, in order. A section that the step showed before restores
its selections from draft.storage; any other preselects its default.
Under the cards of a section it shows the bucket line of each picked
built-in option, with the presets of context, and under the access cards
the hosts and their conflicts. A section with no option and no empty
option says "Your admin allows no option here." Each view keeps its
object while its content does not change. onDeletion gets each pick.`,methods:[],displayName:`ClusterStorageStep`,props:{draft:{required:!0,tsType:{name:`ClusterDraft`},description:``},views:{required:!0,tsType:{name:`unknown`},description:``},registrations:{required:!0,tsType:{name:`unknown`},description:``},sections:{required:!0,tsType:{name:`unknown`},description:``},context:{required:!0,tsType:{name:`BuiltinContext`},description:``},hosts:{required:!0,tsType:{name:`unknown`},description:``},conflicts:{required:!0,tsType:{name:`unknown`},description:``},onSection:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(
  section: string,
  selections: PluginSelection[],
  errors: ValidationError[],
) => void`,signature:{arguments:[{type:{name:`string`},name:`section`},{type:{name:`Array`,elements:[{name:`PluginSelection`}],raw:`PluginSelection[]`},name:`selections`},{type:{name:`Array`,elements:[{name:`ValidationError`}],raw:`ValidationError[]`},name:`errors`}],return:{name:`void`}}},description:``},deletion:{required:!0,tsType:{name:`DeletionChoice`},description:``},onDeletion:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(policy: DeletionPolicy) => void`,signature:{arguments:[{type:{name:`DeletionPolicy`},name:`policy`}],return:{name:`void`}}},description:``}}}})))()}function O(e,t){return{apiVersion:`plugins.camunda.io/v1alpha1`,kind:`PluginSection`,metadata:{name:e},spec:{appliesTo:[`CamundaClusterClaim`],selection:`One`,...t}}}function k(e,t,n={status:`True`,message:``}){return{apiVersion:`plugins.camunda.io/v1alpha1`,kind:`PluginRegistration`,metadata:{name:e},spec:{sections:[],mode:`Optional`,deletionPolicies:[`Delete`],serviceAccount:{name:e,namespace:`plugins`},...t},status:{conditions:[{type:`Accepted`,status:`True`,reason:`Valid`,message:``,lastTransitionTime:`2026-11-10T00:00:00Z`},{type:`Ready`,status:n.status,reason:n.status===`True`?`Healthy`:`CredentialsExpired`,message:n.message,lastTransitionTime:`2026-11-10T00:00:00Z`}]}}}function A(e,t){return{apiVersion:`core.camunda.io/v1`,kind:`ElasticsearchClusterPreset`,metadata:{name:e},spec:{cluster:t}}}function j({sections:e,registrations:t,environment:n,objects:r={},initial:i,onSection:a,onDeletion:o}){let s={environment:n,sections:e,registrations:t,objects:r},[f,p]=(0,N.useState)({}),m=y({steps:c.filter(e=>e.id===`storage`).map(e=>({...e,check:e=>u(e,s,f,[])})),initial:i});return(0,M.jsx)(g,{steps:c.map(e=>({...e,errors:e.id===`storage`?m.errors.storage:void 0})),current:`storage`,children:(0,M.jsx)(C,{draft:m.draft,views:l(m.draft,s),registrations:t,sections:e,context:{environment:n,objects:r},hosts:[],conflicts:[],onSection:(e,t,n)=>{a(e,t,n),m.set([`storage`,e],t),p(t=>({...t,[e]:n}))},deletion:d(m.draft,s),onDeletion:e=>{o(e),m.set([`claim`,`spec`,`deletionPolicy`],e)}})})}var M,N,P,F,I,L,R,z,B;function V(){return(V=e((()=>{M=n(),N=t(),h(),o(),f(),D(),_(),b(),{fn:P}=__STORYBOOK_MODULE_TEST__,F=[O(`encryption-key`,{label:`Encryption key`,description:`The key that encrypts the volumes and the backups.`,order:20,emptyOptionLabel:`No key of my own`}),O(`volume-encryption`,{label:`Volume encryption`,description:`How the volumes of the cluster are encrypted.`,order:30,emptyOptionLabel:`Default encryption of the cluster`}),O(`backup-storage`,{label:`Backup storage`,description:`Where the backups of the cluster go.`,order:40,emptyOptionLabel:`No backups`}),O(`secondary-storage`,{label:`Secondary storage`,description:`Where Operate, Tasklist, and Optimize read their data.`,order:50}),O(`access`,{label:`Access`,description:`How users and clients reach the cluster or the management plane from outside.`,order:60,emptyOptionLabel:`No access from outside`})],I=[k(`key-service`,{label:`Key from the key service`,description:`A key for each cluster, in the key service of the cloud.`,sections:[`encryption-key`]}),k(`encrypted-volumes`,{label:`Volumes with my key`,description:`A storage class that encrypts each volume with the key.`,sections:[`volume-encryption`],inputs:[{name:`keyRef`,from:`encryption-key/keyRef`,required:!0}]}),k(`bucket`,{label:`Bucket for each cluster`,description:`A bucket of its own, with a retention rule.`,sections:[`backup-storage`],parameters:{type:`object`,properties:{retentionDays:{type:`integer`,title:`Retention in days`,minimum:1,maximum:365}}}}),k(`archive-bucket`,{label:`Archive bucket`,description:`A bucket in the archive tier.`,sections:[`backup-storage`]},{status:`False`,message:`The credentials of the plugin expired.`}),k(`external-search`,{label:`External Elasticsearch`,description:`An Elasticsearch cluster that runs outside Kubernetes.`,sections:[`secondary-storage`]}),k(`elasticsearch`,{label:`New Elasticsearch cluster`,description:`Runs an Elasticsearch cluster for the cluster, from an Elasticsearch cluster preset.`,sections:[`secondary-storage`],controller:{name:`camunda-operator`,version:`0.4.0`},parameters:{type:`object`,properties:{preset:{type:`string`,title:`Preset`},releaseRef:{type:`string`,title:`Release`},overrides:{type:`object`,title:`Overrides`}}},inputs:[{name:`objectStorageConfig`,from:`backup-storage/objectStorageConfig`}]}),k(`gateway`,{label:`Gateway`,description:`A host for each cluster on the shared gateway.`,sections:[`access`],parameterValues:{hostnamePattern:`{name}.{namespace}.example.com`}})],L={title:`create/ClusterStorageStep`,component:j,args:{sections:F,registrations:I,initial:{environment:`prod`,namespace:{kind:`existing`,name:`team-orders`},claim:{metadata:{name:`search`},spec:{presetRef:`medium`}}},onSection:P(),onDeletion:P()},render:e=>(0,M.jsx)(j,{...e},JSON.stringify(e))},R={name:`Options with an unavailable one and a required section`,args:{environment:{...v,spec:{...v.spec,sections:{"encryption-key":{allowed:[`key-service`]},"volume-encryption":{allowed:[`encrypted-volumes`]},"backup-storage":{allowed:[`archive-bucket`,`bucket`],default:`archive-bucket`},"secondary-storage":{allowed:[`external-search`],required:!0},access:{allowed:[`gateway`],default:`gateway`}},parameters:{bucket:{retentionDays:{claimEditable:!0}}}}}}},z={name:`Built-in storage with preset cards`,args:{sections:F.filter(e=>[`backup-storage`,`secondary-storage`].includes(e.metadata.name??``)),environment:{...v,spec:{...v.spec,sections:{"backup-storage":{allowed:[`bucket`]},"secondary-storage":{allowed:[`elasticsearch`],default:`elasticsearch`,required:!0}},presets:{elasticsearchCluster:[`search-small`,`search-ha`]},parameters:{elasticsearch:{preset:{claimEditable:!0}}}}},objects:{ElasticsearchClusterPreset:[A(`search-small`,{replicas:1,resources:{requests:{cpu:`1`,memory:`4Gi`}},storageSize:`50Gi`}),A(`search-ha`,{replicas:3,resources:{requests:{cpu:`2`,memory:`8Gi`}},storageSize:`200Gi`})]}}},B=[`OptionsWithAnUnavailableOneAndARequiredSection`,`BuiltInStorageWithPresetCards`],R.parameters={...R.parameters,docs:{...R.parameters?.docs,source:{originalSource:`{
  name: "Options with an unavailable one and a required section",
  args: {
    environment: {
      ...production,
      spec: {
        ...production.spec,
        sections: {
          "encryption-key": {
            allowed: ["key-service"]
          },
          "volume-encryption": {
            allowed: ["encrypted-volumes"]
          },
          "backup-storage": {
            allowed: ["archive-bucket", "bucket"],
            default: "archive-bucket"
          },
          "secondary-storage": {
            allowed: ["external-search"],
            required: true
          },
          access: {
            allowed: ["gateway"],
            default: "gateway"
          }
        },
        parameters: {
          bucket: {
            retentionDays: {
              claimEditable: true
            }
          }
        }
      }
    }
  }
}`,...R.parameters?.docs?.source}}},z.parameters={...z.parameters,docs:{...z.parameters?.docs,source:{originalSource:`{
  name: "Built-in storage with preset cards",
  args: {
    sections: storageSections.filter(s => ["backup-storage", "secondary-storage"].includes(s.metadata.name ?? "")),
    environment: {
      ...production,
      spec: {
        ...production.spec,
        sections: {
          "backup-storage": {
            allowed: ["bucket"]
          },
          "secondary-storage": {
            allowed: ["elasticsearch"],
            default: "elasticsearch",
            required: true
          }
        },
        presets: {
          elasticsearchCluster: ["search-small", "search-ha"]
        },
        parameters: {
          elasticsearch: {
            preset: {
              claimEditable: true
            }
          }
        }
      }
    },
    objects: {
      ElasticsearchClusterPreset: [searchPreset("search-small", {
        replicas: 1,
        resources: {
          requests: {
            cpu: "1",
            memory: "4Gi"
          }
        },
        storageSize: "50Gi"
      }), searchPreset("search-ha", {
        replicas: 3,
        resources: {
          requests: {
            cpu: "2",
            memory: "8Gi"
          }
        },
        storageSize: "200Gi"
      })]
    }
  }
}`,...z.parameters?.docs?.source}}}})))()}V();export{z as BuiltInStorageWithPresetCards,R as OptionsWithAnUnavailableOneAndARequiredSection,B as __namedExportsOrder,L as default};