import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./lock-DusdijP6.js";import{t as i}from"./fields-gP6ea4EU.js";import{n as a}from"./labels-DYL500th.js";import{s as o,t as s}from"./identity-5GpAw6r-.js";import{a as c,i as l,o as u}from"./claim-tab-BQ_KH2Qj.js";function d({fields:e}){return e.length===0?(0,p.jsx)(`p`,{className:`text-sm text-muted`,children:`No settings.`}):(0,p.jsx)(c,{children:e.map(e=>(0,p.jsxs)(l,{label:e.label,note:e.lock?void 0:e.note,children:[f(e),e.lock&&(0,p.jsxs)(`span`,{className:`flex items-start gap-1.5 text-xs text-muted`,children:[(0,p.jsx)(r,{"aria-hidden":!0,className:`mt-px size-3.5 shrink-0`}),a(e.lock)]})]},e.key))})}function f(e){if(m.includes(e.key))return e.value===!0?`On`:`Off`;if(e.key===`identityProvider`&&typeof e.value==`string`)return s[e.value]??e.value;let{value:t}=e;return t==null||t===``?`-`:Array.isArray(t)?t.join(`, `):typeof t==`object`?(0,p.jsx)(`span`,{className:`block`,children:Object.entries(t).map(([e,t])=>(0,p.jsxs)(`span`,{className:`block break-all`,children:[e,`: `,typeof t==`object`?JSON.stringify(t):String(t)]},e))}):String(t)}var p,m;function h(){return(h=e((()=>{p=t(),n(),u(),i(),o(),m=[`console`,`webModeler`],d.__docgenInfo={description:`PlaneSettings lists fields read-only: the label, the value, and under
the value the lock label, or the note of a field without a lock. A
component switch reads On or Off, and the identity provider its mode.`,methods:[],displayName:`PlaneSettings`,props:{fields:{required:!0,tsType:{name:`unknown`},description:``}}}})))()}var g,_,v,y,b,x;function S(){return(S=e((()=>{h(),g=(e,t,n,r={})=>({key:e,group:`identity`,label:t,value:n,claimPath:e,claimSteps:[],...r}),_={title:`planes/PlaneSettings`,component:d,args:{fields:[]}},v={name:`Identity in OIDC mode`,args:{fields:[g(`identityProvider`,`Identity provider`,`oidc`),g(`admin.claimName`,`Administrator claim name`,`email`,{lock:{reason:`immutableAfterStart`}}),g(`admin.claimValue`,`Administrator claim value`,`admin@example.com`,{lock:{reason:`immutableAfterStart`}}),g(`versions.identity`,`Management Identity version`,`8.9.14`),g(`url.identity`,`Management Identity URL`,`https://management.example.com/identity`)]}},y={name:`Identity in Keycloak mode`,args:{fields:[g(`identityProvider`,`Identity provider`,`keycloak`),g(`admin.username`,`Administrator username`,`admin`,{note:`A new username creates a second user.`}),g(`admin.email`,`Administrator email`,`admin@example.com`),g(`versions.identity`,`Management Identity version`,`8.9.14`)]}},b={name:`Web Modeler with mail`,args:{fields:[g(`webModeler`,`Web Modeler`,!0),g(`webModeler.mail`,`Web Modeler mail`,{fromAddress:`modeler@example.com`,smtpHost:`smtp.example.com`}),g(`versions.webModeler`,`Web Modeler version`,`8.9.15`),g(`url.webModeler`,`Web Modeler URL`,`https://management.example.com/modeler`),g(`url.webModelerWebsockets`,`Web Modeler websockets URL`,`https://management.example.com/modeler-ws`,{lock:{reason:`section`}})]}},x=[`IdentityOidc`,`IdentityKeycloak`,`WebModeler`],v.parameters={...v.parameters,docs:{...v.parameters?.docs,source:{originalSource:`{
  name: "Identity in OIDC mode",
  args: {
    fields: [field("identityProvider", "Identity provider", "oidc"), field("admin.claimName", "Administrator claim name", "email", {
      lock: {
        reason: "immutableAfterStart"
      }
    }), field("admin.claimValue", "Administrator claim value", "admin@example.com", {
      lock: {
        reason: "immutableAfterStart"
      }
    }), field("versions.identity", "Management Identity version", "8.9.14"), field("url.identity", "Management Identity URL", "https://management.example.com/identity")]
  }
}`,...v.parameters?.docs?.source}}},y.parameters={...y.parameters,docs:{...y.parameters?.docs,source:{originalSource:`{
  name: "Identity in Keycloak mode",
  args: {
    fields: [field("identityProvider", "Identity provider", "keycloak"), field("admin.username", "Administrator username", "admin", {
      note: "A new username creates a second user."
    }), field("admin.email", "Administrator email", "admin@example.com"), field("versions.identity", "Management Identity version", "8.9.14")]
  }
}`,...y.parameters?.docs?.source}}},b.parameters={...b.parameters,docs:{...b.parameters?.docs,source:{originalSource:`{
  name: "Web Modeler with mail",
  args: {
    fields: [field("webModeler", "Web Modeler", true), field("webModeler.mail", "Web Modeler mail", {
      fromAddress: "modeler@example.com",
      smtpHost: "smtp.example.com"
    }), field("versions.webModeler", "Web Modeler version", "8.9.15"), field("url.webModeler", "Web Modeler URL", "https://management.example.com/modeler"), field("url.webModelerWebsockets", "Web Modeler websockets URL", "https://management.example.com/modeler-ws", {
      lock: {
        reason: "section"
      }
    })]
  }
}`,...b.parameters?.docs?.source}}}})))()}S();export{y as IdentityKeycloak,v as IdentityOidc,b as WebModeler,x as __namedExportsOrder,_ as default};