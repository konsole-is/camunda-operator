import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,r,t as i}from"./not-read-RuqQXk-B.js";import{a,c as o,i as s,n as c,o as l,s as u,t as d}from"./table-gxD6zRl0.js";import{n as f,t as p}from"./status-chip-BroebdH3.js";function m({rows:e,callbacks:t,oidc:n,unread:o=[]}){return(0,g.jsxs)(`section`,{"aria-label":`Optimize`,className:`flex flex-col gap-3 rounded-lg border border-border bg-panel p-4`,children:[(0,g.jsx)(`h2`,{className:`font-heading text-base font-semibold`,children:`Optimize`}),(0,g.jsxs)(`div`,{className:`flex flex-col gap-1 text-sm`,children:[(0,g.jsxs)(`div`,{className:`flex flex-wrap items-center gap-2`,children:[(0,g.jsx)(`span`,{className:`text-muted`,children:`Callbacks`}),(0,g.jsx)(p,{condition:t,readyReason:!0})]}),t?.message&&(0,g.jsx)(`p`,{className:`text-text-2`,children:t.message})]}),n&&(0,g.jsx)(`p`,{className:`text-sm text-text`,children:`Add the callbacks to your identity provider.`}),e.length===0?(0,g.jsx)(`p`,{className:`text-sm text-muted`,children:o.length>0?r(o,!0):`No Optimize with an external URL reads the contract of this plane.`}):(0,g.jsxs)(g.Fragment,{children:[(0,g.jsx)(i,{kinds:o,tab:!0}),(0,g.jsxs)(d,{label:`Optimize table`,"aria-label":`Optimize`,children:[(0,g.jsx)(l,{children:(0,g.jsxs)(u,{children:[(0,g.jsx)(a,{children:`Namespace`}),(0,g.jsx)(a,{children:`Name`}),(0,g.jsx)(a,{children:`External URL`}),(0,g.jsx)(a,{children:`Callback URL`})]})}),(0,g.jsx)(c,{children:e.map(e=>(0,g.jsxs)(u,{children:[(0,g.jsx)(s,{className:`text-text-2`,children:e.namespace}),(0,g.jsx)(`th`,{scope:`row`,className:`px-2 py-2 text-left font-medium whitespace-nowrap text-text`,children:e.name}),(0,g.jsx)(s,{className:`min-w-48 break-all whitespace-normal`,children:(0,g.jsx)(h,{url:e.externalUrl})}),(0,g.jsx)(s,{className:`min-w-48 break-all whitespace-normal`,children:e.callbackUrl===void 0?(0,g.jsx)(`span`,{className:`text-bad`,children:`Not a valid URL`}):(0,g.jsx)(`code`,{className:`text-xs text-text`,children:e.callbackUrl})})]},`${e.namespace}/${e.name}`))})]})]})]})}function h({url:e}){return e===void 0?(0,g.jsx)(`span`,{className:`text-bad`,children:`Not a valid URL`}):/^https?:\/\//.test(e)?(0,g.jsx)(`a`,{href:e,target:`_blank`,rel:`noreferrer`,className:`relative inline-block text-info underline-offset-4 hit-area hover:underline`,children:e}):(0,g.jsx)(`code`,{className:`text-xs text-text`,children:e})}var g;function _(){return(_=e((()=>{g=t(),n(),f(),o(),m.__docgenInfo={description:`OptimizeTab is the Optimize tab of a plane: the state of callbacks once,
then one row for each Optimize whose login callback the plane needs. In
the oidc mode it tells the reader to add the callbacks to the identity
provider. unread names the kinds that the page could not read. While
unread is not empty, the tab never says that there is no Optimize.`,methods:[],displayName:`OptimizeTab`,props:{rows:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  namespace: string;
  name: string;
  externalUrl?: string;
  callbackUrl?: string;
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`externalUrl`,value:{name:`string`,required:!1}},{key:`callbackUrl`,value:{name:`string`,required:!1}}]}}],raw:`OptimizeRow[]`},description:``},callbacks:{required:!0,tsType:{name:`union`,raw:`ConditionLike | undefined`,elements:[{name:`ConditionLike`},{name:`undefined`}]},description:``},oidc:{required:!0,tsType:{name:`boolean`},description:``},unread:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}}}}})))()}var v,y,b,x,S,C,w,T;function E(){return(E=e((()=>{_(),v=(e,t,n)=>({namespace:e,name:t,externalUrl:`https://${n}/optimize`,callbackUrl:`https://${n}/optimize/api/authentication/callback`}),y=(e,t,n=``)=>({type:`OptimizeCallbacksReady`,status:e,reason:t,message:n}),b={title:`planes/OptimizeTab`,component:m},x={name:`Keycloak mode, registered`,args:{rows:[v(`team-orders`,`orders-optimize`,`orders.example.com`),v(`team-billing`,`billing-optimize`,`billing.example.com`)],callbacks:y(`True`,`Healthy`),oidc:!1}},S={name:`Keycloak mode, failed`,args:{rows:[v(`team-orders`,`orders-optimize`,`orders.example.com`)],callbacks:y(`False`,`ConnectionFailed`,`The operator could not reach Keycloak at http://production-keycloak.platform:80/auth to register the login callbacks of Optimize`),oidc:!1}},C={name:`OIDC mode`,args:{rows:[v(`team-orders`,`orders-optimize`,`orders.example.com`)],callbacks:y(`True`,`Disabled`,`The identity provider of the platform config holds the callback URLs of Optimize`),oidc:!0}},w={name:`No Optimize`,args:{rows:[],callbacks:y(`True`,`NoCallbacks`),oidc:!1}},T=[`KeycloakModeRegistered`,`KeycloakModeFailed`,`OidcMode`,`NoOptimize`],x.parameters={...x.parameters,docs:{...x.parameters?.docs,source:{originalSource:`{
  name: "Keycloak mode, registered",
  args: {
    rows: [row("team-orders", "orders-optimize", "orders.example.com"), row("team-billing", "billing-optimize", "billing.example.com")],
    callbacks: callbacks("True", "Healthy"),
    oidc: false
  }
}`,...x.parameters?.docs?.source}}},S.parameters={...S.parameters,docs:{...S.parameters?.docs,source:{originalSource:`{
  name: "Keycloak mode, failed",
  args: {
    rows: [row("team-orders", "orders-optimize", "orders.example.com")],
    callbacks: callbacks("False", "ConnectionFailed", "The operator could not reach Keycloak at http://production-keycloak.platform:80/auth to register the login callbacks of Optimize"),
    oidc: false
  }
}`,...S.parameters?.docs?.source}}},C.parameters={...C.parameters,docs:{...C.parameters?.docs,source:{originalSource:`{
  name: "OIDC mode",
  args: {
    rows: [row("team-orders", "orders-optimize", "orders.example.com")],
    callbacks: callbacks("True", "Disabled", "The identity provider of the platform config holds the callback URLs of Optimize"),
    oidc: true
  }
}`,...C.parameters?.docs?.source}}},w.parameters={...w.parameters,docs:{...w.parameters?.docs,source:{originalSource:`{
  name: "No Optimize",
  args: {
    rows: [],
    callbacks: callbacks("True", "NoCallbacks"),
    oidc: false
  }
}`,...w.parameters?.docs?.source}}}})))()}E();export{S as KeycloakModeFailed,x as KeycloakModeRegistered,w as NoOptimize,C as OidcMode,T as __namedExportsOrder,b as default};