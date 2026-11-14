import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{a as r,f as i}from"./labels-lxF_8UjP.js";import{c as a}from"./dialog-Ud3V7eMw.js";import{r as o}from"./button-lmY3Hw21.js";import{t as s}from"./fields-Ds_W30BQ.js";import{n as c}from"./labels-LUZ4_yzX.js";import{h as l,p as u,t as d,v as f}from"./session-Dcps0m7c.js";import{a as p,i as m,r as h}from"./backup-caution-DTKzsTIy.js";import{a as g,n as _,r as v,t as y}from"./topology-PI2JSrmt.js";import{n as b,t as x}from"./locked-field-Bczcj0Gm.js";import{n as S}from"./cluster-edit-DdC4kMfQ.js";import{r as C,t as w}from"./topology-view-r1D7YDdI.js";function T(e){return[{field:`spec`},{field:`cluster`},{field:e},{field:`mode`}]}function E(e){let{before:t,after:n}=p(e),r=g(t.spec,[]),i=g(n.spec,[]),a={before:{},after:{}};for(let e of r)i.some(t=>t.process===e.process)||(a.before[e.process]=`removed`);for(let e of i)r.some(t=>t.process===e.process)||(a.after[e.process]=`new`);let o=v(n.spec,`gateway`);return{before:r,after:i,marks:a,modes:D.map(r=>{let i=u(e.session,T(r));return{process:r,label:y[r],before:v(t.spec,r),after:v(n.spec,r),host:r===`gateway`||o===`Embedded`?`zeebe`:`gateway`,...i&&{lock:i}}})}}var D;function O(){return(O=e((()=>{s(),d(),m(),_(),D=[`gateway`,`operate`,`tasklist`,`admin`]})))()}function k({change:e,impacts:t,others:n,refused:r,onPick:i}){return(0,N.jsxs)(`div`,{className:`flex flex-col gap-5`,children:[(0,N.jsx)(`div`,{className:`grid gap-4 sm:grid-cols-2`,children:e.modes.map(e=>(0,N.jsx)(j,{row:e,onPick:i},e.process))}),r&&(0,N.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[(t=>e.modes.find(e=>e.process===t)?.label??t)(r.process),` keeps its mode. `,c(r.lock),`.`]}),(0,N.jsxs)(`div`,{className:`grid gap-4 sm:grid-cols-2`,children:[(0,N.jsx)(A,{title:`Now`,children:(0,N.jsx)(w,{boxes:e.before,marks:e.marks.before,pods:!1,label:`Now`})}),(0,N.jsx)(A,{title:`After the change`,children:(0,N.jsx)(w,{boxes:e.after,marks:e.marks.after,pods:!1,label:`After the change`})})]}),(0,N.jsxs)(`section`,{"aria-label":`What happens`,className:`flex flex-col gap-2`,children:[(0,N.jsx)(`h3`,{className:`font-heading text-sm font-semibold text-text`,children:`What happens`}),t.length>0?(0,N.jsx)(`ul`,{className:`flex list-disc flex-col gap-1 pl-5 text-sm text-text-2`,children:t.map(e=>(0,N.jsx)(`li`,{children:e},e))}):(0,N.jsx)(`p`,{className:`text-sm text-muted`,children:`Nothing changes yet.`}),n&&(0,N.jsx)(`p`,{className:`text-xs text-muted`,children:`These lines include the other pending changes of this cluster.`})]})]})}function A({title:e,children:t}){return(0,N.jsxs)(`div`,{className:`flex min-w-0 flex-col gap-2`,children:[(0,N.jsx)(`h3`,{className:`font-heading text-sm font-semibold text-text`,children:e}),t]})}function j({row:e,onPick:t}){let n=(0,P.useId)();return e.lock?(0,N.jsx)(x,{label:e.label,value:e.after,reason:c(e.lock)}):(0,N.jsxs)(`fieldset`,{className:`flex min-w-0 flex-col gap-1.5`,children:[(0,N.jsx)(`legend`,{className:`mb-1.5 text-sm font-medium text-text`,children:e.label}),(0,N.jsx)(`div`,{className:`grid grid-cols-2 gap-0.5 rounded-md border border-border bg-panel-2 p-0.5`,children:F.map(r=>(0,N.jsxs)(`label`,{className:`flex cursor-pointer items-center justify-center rounded px-3 py-1.5 text-sm text-text-2 has-checked:bg-panel has-checked:font-medium has-checked:text-text has-checked:ring-1 has-checked:ring-accent has-focus-visible:ring-3 has-focus-visible:ring-ring/50`,children:[(0,N.jsx)(`input`,{type:`radio`,name:n,value:r,checked:e.after===r,onChange:()=>t(e.process,r),className:`sr-only`}),r]},r))}),(0,N.jsx)(`p`,{className:`text-xs text-muted`,children:M(e)})]})}function M(e){return e.after===`Standalone`?`Own Deployment and Service.`:e.host===`zeebe`?`Runs in the brokers.`:`Runs in the gateway.`}var N,P,F;function I(){return(I=e((()=>{N=n(),P=t(),b(),o(),a(),s(),S(),m(),O(),C(),F=[`Standalone`,`Embedded`],k.__docgenInfo={description:`TopologyChangePanel shows a mode choice for each row of change, the
topology now and after the change, and impacts. others says that the
change holds pending changes that are not a mode. refused is the last
pick that a lock refused.`,methods:[],displayName:`TopologyChangePanel`,props:{change:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  before: TopologyBox[];
  after: TopologyBox[];
  marks: BoxMarks;
  modes: ModeRow[];
}`,signature:{properties:[{key:`before`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  process: Process;
  label: string;
  workload: "StatefulSet" | "Deployment";
  embedded: { process: Process; label: string }[];
  pods: PodDot[];
}`,signature:{properties:[{key:`process`,value:{name:`CoreComponent`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`workload`,value:{name:`union`,raw:`"StatefulSet" | "Deployment"`,elements:[{name:`literal`,value:`"StatefulSet"`},{name:`literal`,value:`"Deployment"`}],required:!0}},{key:`embedded`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ process: Process; label: string }`,signature:{properties:[{key:`process`,value:{name:`CoreComponent`,required:!0}},{key:`label`,value:{name:`string`,required:!0}}]}}],raw:`{ process: Process; label: string }[]`,required:!0}},{key:`pods`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ name: string; tone: StatusTone; state: string }`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`tone`,value:{name:`StatusTone`,required:!0}},{key:`state`,value:{name:`string`,required:!0}}]}}],raw:`PodDot[]`,required:!0}}]}}],raw:`TopologyBox[]`,required:!0}},{key:`after`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  process: Process;
  label: string;
  workload: "StatefulSet" | "Deployment";
  embedded: { process: Process; label: string }[];
  pods: PodDot[];
}`,signature:{properties:[{key:`process`,value:{name:`CoreComponent`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`workload`,value:{name:`union`,raw:`"StatefulSet" | "Deployment"`,elements:[{name:`literal`,value:`"StatefulSet"`},{name:`literal`,value:`"Deployment"`}],required:!0}},{key:`embedded`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ process: Process; label: string }`,signature:{properties:[{key:`process`,value:{name:`CoreComponent`,required:!0}},{key:`label`,value:{name:`string`,required:!0}}]}}],raw:`{ process: Process; label: string }[]`,required:!0}},{key:`pods`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ name: string; tone: StatusTone; state: string }`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`tone`,value:{name:`StatusTone`,required:!0}},{key:`state`,value:{name:`string`,required:!0}}]}}],raw:`PodDot[]`,required:!0}}]}}],raw:`TopologyBox[]`,required:!0}},{key:`marks`,value:{name:`signature`,type:`object`,raw:`{
  before: Partial<Record<Process, "removed">>;
  after: Partial<Record<Process, "new">>;
}`,signature:{properties:[{key:`before`,value:{name:`Partial`,elements:[{name:`Record`,elements:[{name:`CoreComponent`,required:!0},{name:`literal`,value:`"removed"`}],raw:`Record<Process, "removed">`}],raw:`Partial<Record<Process, "removed">>`,required:!0}},{key:`after`,value:{name:`Partial`,elements:[{name:`Record`,elements:[{name:`CoreComponent`,required:!0},{name:`literal`,value:`"new"`}],raw:`Record<Process, "new">`}],raw:`Partial<Record<Process, "new">>`,required:!0}}]},required:!0}},{key:`modes`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  process: ModeProcess;
  label: string;
  before: ComponentMode;
  after: ComponentMode;
  host: Process;
  lock?: FieldLock;
}`,signature:{properties:[{key:`process`,value:{name:`unknown[number]`,raw:`(typeof MODE_PROCESSES)[number]`,required:!0}},{key:`label`,value:{name:`string`,required:!0}},{key:`before`,value:{name:`NonNullable`,elements:[{name:`NonNullable["mode"]`,raw:`NonNullable<CamundaClusterSpec["operate"]>["mode"]`}],raw:`NonNullable<
  NonNullable<CamundaClusterSpec["operate"]>["mode"]
>`,required:!0}},{key:`after`,value:{name:`NonNullable`,elements:[{name:`NonNullable["mode"]`,raw:`NonNullable<CamundaClusterSpec["operate"]>["mode"]`}],raw:`NonNullable<
  NonNullable<CamundaClusterSpec["operate"]>["mode"]
>`,required:!0}},{key:`host`,value:{name:`CoreComponent`,required:!0}},{key:`lock`,value:{name:`FieldLock`,required:!1}}]}}],raw:`ModeRow[]`,required:!0}}]}},description:``},impacts:{required:!0,tsType:{name:`Array`,elements:[{name:`string`}],raw:`string[]`},description:``},others:{required:!0,tsType:{name:`boolean`},description:``},refused:{required:!1,tsType:{name:`signature`,type:`object`,raw:`{ process: ModeProcess; lock: FieldLock }`,signature:{properties:[{key:`process`,value:{name:`unknown[number]`,raw:`(typeof MODE_PROCESSES)[number]`,required:!0}},{key:`lock`,value:{name:`FieldLock`,required:!0}}]}},description:``},onPick:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(process: ModeProcess, mode: ComponentMode) => void`,signature:{arguments:[{type:{name:`unknown[number]`,raw:`(typeof MODE_PROCESSES)[number]`},name:`process`},{type:{name:`NonNullable`,elements:[{name:`NonNullable["mode"]`,raw:`NonNullable<CamundaClusterSpec["operate"]>["mode"]`}],raw:`NonNullable<
  NonNullable<CamundaClusterSpec["operate"]>["mode"]
>`},name:`mode`}],return:{name:`void`}}},description:``}}}})))()}function L(e,t,n=()=>void 0){let i={name:`orders`,namespace:`team-payments`},a={apiVersion:`core.camunda.io/v1`,kind:`CamundaCluster`,metadata:{...i,labels:{[r]:`orders`}},spec:e},o=l({claim:{apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,metadata:{...i,resourceVersion:`41`},spec:{cluster:e}},resource:a,locks:n});for(let[e,n]of t){let t=f(o,T(e),n);`session`in t&&(o=t.session)}let s={session:o,live:a,presets:[],releases:[],sections:[]};return{change:E(s),impacts:h(s),others:!1}}function R({spec:e,picks:t,locks:n,onPick:r}){let[i,a]=(0,B.useState)(t);return(0,z.jsx)(k,{...L(e,i,n),onPick:(e,t)=>{r(e,t),a(n=>[...n.filter(([t])=>t!==e),[e,t]])}})}var z,B,V,H,U,W,G,K;function q(){return(q=e((()=>{z=n(),B=t(),i(),d(),m(),O(),I(),{fn:V}=__STORYBOOK_MODULE_TEST__,H={title:`cluster/TopologyChangePanel`,component:R,args:{onPick:V()},render:e=>(0,z.jsx)(R,{...e},JSON.stringify([e.spec,e.picks]))},U={name:`Gateway becomes embedded`,args:{spec:{},picks:[[`gateway`,`Embedded`]]}},W={name:`Operate becomes standalone with prepared settings`,args:{spec:{operate:{replicas:2,resources:{requests:{cpu:`1`}}}},picks:[[`operate`,`Standalone`]]}},G={name:`Locked mode`,args:{spec:{connectors:{enabled:!0}},picks:[[`tasklist`,`Standalone`]],locks:e=>e===`spec.cluster.gateway.mode`?{reason:`admin`}:void 0}},K=[`GatewayBecomesEmbedded`,`OperateBecomesStandaloneWithPreparedSettings`,`LockedMode`],U.parameters={...U.parameters,docs:{...U.parameters?.docs,source:{originalSource:`{
  name: "Gateway becomes embedded",
  args: {
    spec: {},
    picks: [["gateway", "Embedded"]]
  }
}`,...U.parameters?.docs?.source}}},W.parameters={...W.parameters,docs:{...W.parameters?.docs,source:{originalSource:`{
  name: "Operate becomes standalone with prepared settings",
  args: {
    spec: {
      operate: {
        replicas: 2,
        resources: {
          requests: {
            cpu: "1"
          }
        }
      }
    },
    picks: [["operate", "Standalone"]]
  }
}`,...W.parameters?.docs?.source}}},G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
  name: "Locked mode",
  args: {
    spec: {
      connectors: {
        enabled: true
      }
    },
    picks: [["tasklist", "Standalone"]],
    locks: path => path === "spec.cluster.gateway.mode" ? {
      reason: "admin"
    } : undefined
  }
}`,...G.parameters?.docs?.source}}}})))()}q();export{U as GatewayBecomesEmbedded,G as LockedMode,W as OperateBecomesStandaloneWithPreparedSettings,K as __namedExportsOrder,H as default};