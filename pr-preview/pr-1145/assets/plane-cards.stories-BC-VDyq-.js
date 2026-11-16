import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{h as t,o as n,x as r}from"./labels-Be1Q9KU6.js";import{a as i,c as a,t as o}from"./identity-DWXU5AdA.js";import{n as s,t as c}from"./camundamanagementcluster-B7akbDjz.js";import{t as l}from"./placement-BPmEgwCN.js";import{i as u,n as d,o as f}from"./cards-5qPsMtrX.js";import{n as p,t as m}from"./component-card-C9VSQ0DG.js";function h(e){let{plane:t,unread:r=[]}=e,i=g(t),a=[...i===`keycloak`?[`keycloak`]:[],...i===`externalKeycloak`||i===`oidc`?[`identityProvider`]:[],`identity`,...t.spec.console?[`console`]:[],...t.spec.webModeler?[`webModelerRestapi`,`webModelerWebsockets`]:[]],s=[...e.pods].sort((e,t)=>(e.metadata?.name??``).localeCompare(t.metadata?.name??``)),u=r.includes(`Pod`),p=r.includes(`CamundaManagementCluster`);return a.map(a=>{if(a===`identityProvider`)return{id:a,label:b[a],used:!0,embedded:[],pods:[],numbers:[{label:`Mode`,value:i?o[i]:`-`},_(t,i,e.config,r)]};let m=s.filter(e=>e.metadata?.labels?.[n]===x[a]).map(e=>e.metadata?.name??``),h=s.filter(e=>m.includes(e.metadata?.name??``)),g=l(c,t.status?.conditions,y[a])[0];return{id:a,label:b[a],used:!0,...g&&{condition:g},...p&&{unknown:!0},embedded:[],pods:m,numbers:u?[{label:`Pods ready`,value:`Could not read Pod`,severity:`Warning`},{label:`Highest use`,value:`Not known`}]:[f(h),d(e.usage,m)]}})}function g(e){let t=e.spec.identityProvider??{};if(t.keycloak)return`keycloak`;if(t.externalKeycloak)return`externalKeycloak`;if(t.oidc)return`oidc`}function _(e,t,n,r){if(t===`oidc`&&r.includes(`CamundaPlatformConfig`))return{label:`Issuer`,value:`Could not read CamundaPlatformConfig`,severity:`Warning`};let i=e.spec.identityProvider?.externalKeycloak,o=a(t,{url:i?.url??``,realm:i?.realm??``,adminSecret:``},n);return{label:`Issuer`,value:o.kind===`known`?o.url:`Unknown`}}function v(e){if(e!==`identityProvider`)return e===null?`?tab=overview`:`?${new URLSearchParams({tab:`overview`,component:e})}`}var y,b,x;function S(){return(S=e((()=>{u(),s(),r(),i(),y={keycloak:`keycloak`,identityProvider:void 0,identity:`identity`,console:`console`,webModelerRestapi:`webModeler`,webModelerWebsockets:`webModeler`},b={keycloak:`Keycloak`,identityProvider:`Identity provider`,identity:`Management Identity`,console:`Console`,webModelerRestapi:`Web Modeler REST API`,webModelerWebsockets:`Web Modeler websockets`},x=t})))()}function C(e,t,n=!0){return{apiVersion:`core.camunda.io/v1`,kind:`CamundaManagementCluster`,metadata:{namespace:`platform`,name:`plane-a`},spec:{platformConfigRef:`oidc-platform`,identityProvider:e,identity:{},...n&&{console:{},webModeler:{}}},status:{conditions:t}}}function w(e,t,r=!0){return{kind:`Pod`,metadata:{name:e,labels:{[n]:t}},status:{phase:`Running`,conditions:[{type:`Ready`,status:r?`True`:`False`}],containerStatuses:[{name:`app`,ready:r,restartCount:0}]}}}var T,E,D,O,k,A,j,M,N,P,F;function I(){return(I=e((()=>{p(),r(),S(),{fn:T}=__STORYBOOK_MODULE_TEST__,E=(e,t=`True`,n=`Healthy`)=>({type:e,status:t,reason:n,message:t===`True`?``:`${e} reports ${n}`,lastTransitionTime:`2026-11-15T12:00:00Z`}),D=(e,t)=>({pod:e,cpu:{usedCores:t*.5,limitCores:1},memory:{usedBytes:t*1024**3,limitBytes:1024**3}}),O={metadata:{name:`oidc-platform`},spec:{auth:{method:`oidc`,oidc:{issuerUrl:`https://login.example.com/realms/camunda`}}}},k=[w(`plane-a-management-identity-0`,`management-identity`),w(`plane-a-console-0`,`console`),w(`plane-a-web-modeler-restapi-0`,`web-modeler-restapi`),w(`plane-a-web-modeler-websockets-0`,`web-modeler-websockets`)],A={title:`planes/PlaneCards`,component:m,args:{hrefOf:v,onOpen:T(),cards:[]}},j={name:`OIDC ready`,args:{cards:h({plane:C({oidc:{}},[E(`IdentityReady`),E(`ConsoleReady`),E(`WebModelerReady`)]),pods:k,config:O,usage:k.map((e,t)=>D(e.metadata?.name??``,.2+t*.1))})}},M={name:`Keycloak creating`,args:{cards:h({plane:C({keycloak:{}},[E(`KeycloakReady`,`False`,`Creating`),E(`IdentityReady`,`False`,`PrerequisiteNotMet`)],!1),pods:[w(`plane-a-keycloak-0`,`keycloak`,!1),w(`plane-a-management-identity-0`,`management-identity`,!1)],usage:[]})}},N={name:`Identity down`,args:{cards:h({plane:C({oidc:{}},[E(`IdentityReady`,`False`,`Down`),E(`ConsoleReady`),E(`WebModelerReady`)]),pods:[w(`plane-a-management-identity-0`,`management-identity`,!1),...k.slice(1)],config:O,usage:[]})}},P={name:`Plane and pods not read`,args:{cards:h({plane:C({oidc:{}},[E(`IdentityReady`)]),pods:k,config:O,usage:[],unread:[`CamundaManagementCluster`,`Pod`,`CamundaPlatformConfig`]})}},F=[`OidcReady`,`KeycloakCreating`,`IdentityDown`,`NotRead`],j.parameters={...j.parameters,docs:{...j.parameters?.docs,source:{originalSource:`{
  name: "OIDC ready",
  args: {
    cards: planeCards({
      plane: plane({
        oidc: {}
      }, [condition("IdentityReady"), condition("ConsoleReady"), condition("WebModelerReady")]),
      pods,
      config,
      usage: pods.map((p, i) => usage(p.metadata?.name ?? "", 0.2 + i * 0.1))
    })
  }
}`,...j.parameters?.docs?.source}}},M.parameters={...M.parameters,docs:{...M.parameters?.docs,source:{originalSource:`{
  name: "Keycloak creating",
  args: {
    cards: planeCards({
      plane: plane({
        keycloak: {}
      }, [condition("KeycloakReady", "False", "Creating"), condition("IdentityReady", "False", "PrerequisiteNotMet")], false),
      pods: [pod("plane-a-keycloak-0", "keycloak", false), pod("plane-a-management-identity-0", "management-identity", false)],
      usage: []
    })
  }
}`,...M.parameters?.docs?.source}}},N.parameters={...N.parameters,docs:{...N.parameters?.docs,source:{originalSource:`{
  name: "Identity down",
  args: {
    cards: planeCards({
      plane: plane({
        oidc: {}
      }, [condition("IdentityReady", "False", "Down"), condition("ConsoleReady"), condition("WebModelerReady")]),
      pods: [pod("plane-a-management-identity-0", "management-identity", false), ...pods.slice(1)],
      config,
      usage: []
    })
  }
}`,...N.parameters?.docs?.source}}},P.parameters={...P.parameters,docs:{...P.parameters?.docs,source:{originalSource:`{
  name: "Plane and pods not read",
  args: {
    cards: planeCards({
      plane: plane({
        oidc: {}
      }, [condition("IdentityReady")]),
      pods,
      config,
      usage: [],
      unread: ["CamundaManagementCluster", "Pod", "CamundaPlatformConfig"]
    })
  }
}`,...P.parameters?.docs?.source}}}})))()}I();export{N as IdentityDown,M as KeycloakCreating,P as NotRead,j as OidcReady,F as __namedExportsOrder,A as default};