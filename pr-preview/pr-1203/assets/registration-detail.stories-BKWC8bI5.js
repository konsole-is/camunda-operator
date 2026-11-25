import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{t as r}from"./hooks-DwXdO6lu.js";import{x as i}from"./labels-Be1Q9KU6.js";import{a,i as o,n as s,r as c,t as l}from"./tabs-1hHOXEQP.js";import"./events-C2nhwmNt.js";import{o as u}from"./unread-Do34-4tt.js";import{n as d}from"./tabs-CxIoKCAa.js";import{n as f,r as p}from"./grants-tab-CQ-xXofA.js";import{t as m}from"./use-clock-DsKrgeKH.js";import{n as h,t as g}from"./about-tab-Ceu3VEqb.js";import{i as _,n as v}from"./fixtures-kgATjJaf.js";import{i as y,n as b,r as x,t as S}from"./contract-tab-CiyenwI0.js";import{n as C,t as w}from"./health-tab-B8esibLF.js";import{i as T,n as E,r as D,t as O}from"./parameters-tab-BQkcXKDX.js";import{t as k}from"./sources-pgcXN-2p.js";import{i as A,n as j,r as M,t as N}from"./used-by-tab-BkzYsSVR.js";function P(){return(P=e((()=>{r(),i(),k()})))()}function F(e){return d(I,e,`about`)}var I;function L(){return(L=e((()=>{I=[{value:`about`,label:`About`},{value:`contract`,label:`Contract`},{value:`parameters`,label:`Parameters`},{value:`grants`,label:`Grants`},{value:`health`,label:`Health`},{value:`used-by`,label:`Used by`}]})))()}function R({data:e,errors:t,stale:n,tab:r,now:i}){let[a,d]=(0,B.useState)(r),p=e=>{let t=F(e);d(t),window.history.replaceState(null,``,`?tab=${t}`)},m=u({errors:t,stale:n}),{sources:h}=e,_=m.includes(`PluginRegistration`),v=h.registrations.find(t=>t.metadata.name===e.name),[y,b]=(0,B.useState)(!1);!_&&y!==!v&&b(!v);let C=v??(_&&!y?e.registration:void 0),E=_&&(0,z.jsx)(`p`,{className:`mb-4 text-sm text-muted`,children:`This is the registration as the page last read it. It can be old.`});return(0,z.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,z.jsxs)(`header`,{className:`flex min-w-0 flex-col gap-1`,children:[(0,z.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em] break-words`,children:C?.spec.label??e.name}),(0,z.jsx)(`p`,{className:`font-mono text-sm break-all text-muted`,children:e.name})]}),t.map(e=>(0,z.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind)),m.length>0&&(0,z.jsx)(`p`,{className:`text-sm text-muted`,children:`Not read: ${m.join(`, `)}. What needs them shows as not known${_&&C?`, and the registration as the page last read it`:``}.`}),C?(0,z.jsxs)(l,{value:a,onValueChange:p,children:[(0,z.jsx)(c,{className:`h-auto max-w-full flex-wrap`,children:I.map(e=>(0,z.jsx)(o,{value:e.value,children:e.label},e.value))}),(0,z.jsxs)(s,{value:`about`,children:[E,(0,z.jsx)(g,{registration:C,registrations:h.registrations,unread:m})]}),(0,z.jsxs)(s,{value:`contract`,children:[E,(0,z.jsx)(S,{contract:x(C,h.sections,m)})]}),(0,z.jsxs)(s,{value:`parameters`,children:[E,(0,z.jsx)(O,{rows:T(C,h.environments,m),unread:m})]}),(0,z.jsxs)(s,{value:`grants`,children:[E,(0,z.jsx)(f,{registration:C,grants:e.grants,unread:m,now:i})]}),(0,z.jsx)(s,{value:`health`,children:(0,z.jsx)(w,{registration:C,events:e.events,unread:m,now:i})}),(0,z.jsx)(s,{value:`used-by`,children:(0,z.jsx)(N,{rows:A(e.name,{...h,errors:t,stale:n},e.grants),unread:m})})]}):(0,z.jsx)(`p`,{className:`text-sm`,children:`This registration no longer exists.${_?` It was gone when the page last read it.`:``}`})]})}var z,B;function V(){return(V=e((()=>{z=n(),B=t(),m(),a(),h(),y(),b(),P(),p(),C(),D(),E(),L(),M(),j(),R.__docgenInfo={description:`RegistrationDetail is the page of the registration data.name, opened on
tab. A tab change sets the tab query parameter without a navigation.
errors holds the error of each failed watch, and stale the kind of each
list whose watch is not fresh (freshKey). Once a read
of PluginRegistration has no registration of the name, the page says
that it no longer exists. Before that, while the kind is not read, it
shows the registration of its list, else data.registration, as possibly
old. now is the time that the ages count to.`,methods:[],displayName:`RegistrationDetail`,props:{data:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
  name: string;
  /** registration is the registration as the server read it for the page. */
  registration: PluginRegistration;
  sources: RegistrationSnapshot;
  /** grants are the Grants with REGISTRATION_LABEL=name, in every namespace. */
  grants: Grant[];
  /** events are the events that regard the registration. */
  events: Resource<"Event">[];
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`registration`,value:{name:`PluginRegistration`,required:!0},description:`registration is the registration as the server read it for the page.`},{key:`sources`,value:{name:`intersection`,raw:`RegistrationSources & {
  readErrors?: ReadError[];
}`,elements:[{name:`signature`,type:`object`,raw:`{
  sections: PluginSection[];
  registrations: PluginRegistration[];
  requests: PluginRequest[];
  namespaces: V1Namespace[];
  environments: Environment[];
}`,signature:{properties:[{key:`sections`,value:{name:`Array`,elements:[{name:`PluginSection`}],raw:`PluginSection[]`,required:!0}},{key:`registrations`,value:{name:`Array`,elements:[{name:`PluginRegistration`}],raw:`PluginRegistration[]`,required:!0}},{key:`requests`,value:{name:`Array`,elements:[{name:`PluginRequest`}],raw:`PluginRequest[]`,required:!0}},{key:`namespaces`,value:{name:`Array`,elements:[{name:`V1Namespace`}],raw:`V1Namespace[]`,required:!0}},{key:`environments`,value:{name:`Array`,elements:[{name:`Environment`}],raw:`Environment[]`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{
  readErrors?: ReadError[];
}`,signature:{properties:[{key:`readErrors`,value:{name:`Array`,elements:[{name:`ReadError`}],raw:`ReadError[]`,required:!1}}]}}],required:!0}},{key:`grants`,value:{name:`Array`,elements:[{name:`Grant`}],raw:`Grant[]`,required:!0},description:`grants are the Grants with REGISTRATION_LABEL=name, in every namespace.`},{key:`events`,value:{name:`Array`,elements:[{name:`Resource`,elements:[{name:`literal`,value:`"Event"`}],raw:`Resource<"Event">`}],raw:`Resource<"Event">[]`,required:!0},description:`events are the events that regard the registration.`}]}},description:``},errors:{required:!0,tsType:{name:`Array`,elements:[{name:`ReadError`}],raw:`ReadError[]`},description:``},stale:{required:!0,tsType:{name:`unknown`},description:``},tab:{required:!0,tsType:{name:`union`,raw:`"about" | "contract" | "parameters" | "grants" | "health" | "used-by"`,elements:[{name:`literal`,value:`"about"`},{name:`literal`,value:`"contract"`},{name:`literal`,value:`"parameters"`},{name:`literal`,value:`"grants"`},{name:`literal`,value:`"health"`},{name:`literal`,value:`"used-by"`}]},description:``},now:{required:!0,tsType:{name:`string`},description:``}}}})))()}var H,U,W,G,K;function q(){return(q=e((()=>{_(),V(),H=v(`elasticsearch`),U={title:`setup/plugins/RegistrationDetail`,component:R,args:{data:H,errors:[],stale:[],tab:`about`,now:`2026-11-13T09:00:00Z`}},W={name:`About tab of a built-in registration`},G={name:`Registration that no longer exists`,args:{data:{...H,sources:{...H.sources,registrations:[]}}}},K=[`AboutBuiltIn`,`Gone`],W.parameters={...W.parameters,docs:{...W.parameters?.docs,source:{originalSource:`{
  name: "About tab of a built-in registration"
}`,...W.parameters?.docs?.source}}},G.parameters={...G.parameters,docs:{...G.parameters?.docs,source:{originalSource:`{
  name: "Registration that no longer exists",
  args: {
    data: {
      ...data,
      sources: {
        ...data.sources,
        registrations: []
      }
    }
  }
}`,...G.parameters?.docs?.source}}}})))()}q();export{W as AboutBuiltIn,G as Gone,K as __namedExportsOrder,U as default};