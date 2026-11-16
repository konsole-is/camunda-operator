import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./link-DOLMGIR2.js";import{a as i,c as a,i as o,n as s,o as c,s as l,t as u}from"./table-DMfe51_x.js";import{a as d,i as f,r as p}from"./claim-tab-CNZ4jfeU.js";import{n as m,t as h}from"./unread-note-QLBCoOFq.js";function g({name:e,contract:t,readers:n,unread:r=[]}){let i=r.length===0;return(0,x.jsxs)(`section`,{"aria-label":`Contract`,className:`flex flex-col gap-3 rounded-lg border border-border bg-panel p-4`,children:[(0,x.jsx)(`h2`,{className:`font-heading text-base font-semibold`,children:`Contract`}),(0,x.jsx)(h,{kinds:r}),e===void 0?i&&(0,x.jsx)(`p`,{className:`text-sm text-muted`,children:`This plane has not written its contract yet.`}):t?(0,x.jsx)(_,{name:e,contract:t}):i&&(0,x.jsx)(`p`,{className:`text-sm text-muted`,children:`The contract ${e} does not exist.`}),e!==void 0&&(0,x.jsxs)(x.Fragment,{children:[(0,x.jsx)(`h3`,{className:`font-heading text-sm font-semibold`,children:`Readers`}),n.length===0?i&&(0,x.jsx)(`p`,{className:`text-sm text-muted`,children:`No Optimize reads this contract.`}):(0,x.jsx)(v,{readers:n})]})]})}function _({name:e,contract:t}){let n=t.spec,r=e=>y(n,e),i=y(n,`clientSecretRef`),a=e=>typeof i==`object`&&i?y(i,e):void 0,o=r(`issuerBackendUrl`);return(0,x.jsxs)(f,{children:[(0,x.jsx)(p,{label:`Name`,children:e}),(0,x.jsx)(p,{label:`Base URL`,children:b(r(`baseUrl`))}),(0,x.jsx)(p,{label:`Issuer URL`,children:b(r(`issuerUrl`))}),(0,x.jsx)(p,{label:`Issuer URL inside the cluster`,note:o===void 0||o===``?`Optimize uses the issuer URL.`:void 0,children:b(o)}),(0,x.jsx)(p,{label:`Authorization URL`,children:b(r(`authUrl`))}),(0,x.jsx)(p,{label:`Token URL`,children:b(r(`tokenUrl`))}),(0,x.jsx)(p,{label:`JWKS URL`,children:b(r(`jwksUrl`))}),(0,x.jsx)(p,{label:`Client ID`,children:b(r(`clientId`))}),(0,x.jsx)(p,{label:`Audience`,children:b(r(`audience`))}),(0,x.jsx)(p,{label:`Client secret namespace`,children:b(a(`namespace`))}),(0,x.jsx)(p,{label:`Client secret name`,children:b(a(`name`))}),(0,x.jsx)(p,{label:`Client secret key`,children:b(a(`key`))})]})}function v({readers:e}){return(0,x.jsxs)(u,{label:`Readers table`,"aria-label":`Readers`,children:[(0,x.jsx)(c,{children:(0,x.jsxs)(l,{children:[(0,x.jsx)(i,{children:`Namespace`}),(0,x.jsx)(i,{children:`Name`}),(0,x.jsx)(i,{children:`Cluster`}),(0,x.jsx)(i,{children:`External URL`})]})}),(0,x.jsx)(s,{children:e.map(e=>{let t=e.metadata?.namespace??``,r=e.spec?.clusterRef?.name,i=e.spec?.externalUrl;return(0,x.jsxs)(l,{children:[(0,x.jsx)(o,{className:`text-text-2`,children:t}),(0,x.jsx)(`th`,{scope:`row`,className:`px-2 py-2 text-left font-medium whitespace-nowrap text-text`,children:e.metadata?.name}),(0,x.jsx)(o,{children:typeof r==`string`&&r!==``?(0,x.jsx)(n,{href:`/fleet/clusters/${encodeURIComponent(t)}/${encodeURIComponent(r)}`,className:`relative font-medium text-text hit-area hover:text-info`,children:r}):S}),(0,x.jsx)(o,{className:`min-w-48 break-all whitespace-normal`,children:b(i)})]},`${t}/${e.metadata?.name}`)})})]})}function y(e,t){return typeof e==`object`&&e?e[t]:void 0}function b(e){return e==null||e===``?S:typeof e==`string`?(0,x.jsx)(`code`,{className:`text-xs break-all text-text`,children:e}):(0,x.jsx)(`span`,{className:`text-bad`,children:`Not valid`})}var x,S;function C(){return(C=e((()=>{x=t(),r(),d(),a(),m(),S=(0,x.jsx)(`span`,{className:`text-muted`,children:`Not set`}),g.__docgenInfo={description:`ContractTab is the Contract tab of a plane. name is the contract that the
plane writes, undefined before it wrote one. contract is the
ManagementAuthConfig of that name, undefined when it is not in the list.
The client secret shows as a reference only. readers are the
CamundaOptimize objects that read the contract. unread names the kinds
that the page could not read. While unread is not empty, the tab never
says that the contract or its readers are missing.`,methods:[],displayName:`ContractTab`,props:{name:{required:!0,tsType:{name:`union`,raw:`string | undefined`,elements:[{name:`string`},{name:`undefined`}]},description:``},contract:{required:!0,tsType:{name:`union`,raw:`Resource<"ManagementAuthConfig"> | undefined`,elements:[{name:`Resource`,elements:[{name:`literal`,value:`"ManagementAuthConfig"`}],raw:`Resource<"ManagementAuthConfig">`},{name:`undefined`}]},description:``},readers:{required:!0,tsType:{name:`unknown`},description:``},unread:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}}}}})))()}var w,T,E,D,O,k,A,j,M,N;function P(){return(P=e((()=>{C(),w=`https://login.example.com/realms/camunda`,T=e=>({apiVersion:`core.camunda.io/v1`,kind:`ManagementAuthConfig`,metadata:{name:`production`},spec:{baseUrl:`http://production-identity.platform:80`,issuerUrl:w,...e&&{issuerBackendUrl:e},authUrl:`${w}/protocol/openid-connect/auth`,tokenUrl:`${w}/protocol/openid-connect/token`,jwksUrl:`${w}/protocol/openid-connect/certs`,clientId:`optimize`,audience:`optimize-api`,clientSecretRef:{namespace:`platform`,name:`production-optimize-client`,key:`client-secret`}}}),E=(e,t,n,r)=>({apiVersion:`core.camunda.io/v1`,kind:`CamundaOptimize`,metadata:{namespace:e,name:t},spec:{clusterRef:{name:n},managementAuthRef:`production`,...r&&{externalUrl:r}}}),D={title:`planes/ContractTab`,component:g},O={name:`With issuer inside the cluster`,args:{name:`production`,contract:T(`http://keycloak.platform:8080/auth/realms/camunda`),readers:[E(`team-lab`,`lab-optimize`,`lab`),E(`team-orders`,`orders-optimize`,`orders`,`https://orders.example.com/optimize`)]}},k={name:`Without issuer inside the cluster`,args:{name:`production`,contract:T(),readers:[]}},A={name:`No contract yet`,args:{name:void 0,contract:void 0,readers:[]}},j={name:`Contract missing`,args:{name:`production`,contract:void 0,readers:[]}},M={name:`Contracts not read`,args:{name:`production`,contract:void 0,readers:[],unread:[`ManagementAuthConfig`,`CamundaOptimize`]}},N=[`WithIssuerInsideTheCluster`,`WithoutIssuerInsideTheCluster`,`NoContractYet`,`ContractMissing`,`ContractsNotRead`],O.parameters={...O.parameters,docs:{...O.parameters?.docs,source:{originalSource:`{
  name: "With issuer inside the cluster",
  args: {
    name: "production",
    contract: contractOf("http://keycloak.platform:8080/auth/realms/camunda"),
    readers: [reader("team-lab", "lab-optimize", "lab"), reader("team-orders", "orders-optimize", "orders", "https://orders.example.com/optimize")]
  }
}`,...O.parameters?.docs?.source}}},k.parameters={...k.parameters,docs:{...k.parameters?.docs,source:{originalSource:`{
  name: "Without issuer inside the cluster",
  args: {
    name: "production",
    contract: contractOf(),
    readers: []
  }
}`,...k.parameters?.docs?.source}}},A.parameters={...A.parameters,docs:{...A.parameters?.docs,source:{originalSource:`{
  name: "No contract yet",
  args: {
    name: undefined,
    contract: undefined,
    readers: []
  }
}`,...A.parameters?.docs?.source}}},j.parameters={...j.parameters,docs:{...j.parameters?.docs,source:{originalSource:`{
  name: "Contract missing",
  args: {
    name: "production",
    contract: undefined,
    readers: []
  }
}`,...j.parameters?.docs?.source}}},M.parameters={...M.parameters,docs:{...M.parameters?.docs,source:{originalSource:`{
  name: "Contracts not read",
  args: {
    name: "production",
    contract: undefined,
    readers: [],
    unread: ["ManagementAuthConfig", "CamundaOptimize"]
  }
}`,...M.parameters?.docs?.source}}}})))()}P();export{j as ContractMissing,M as ContractsNotRead,A as NoContractYet,O as WithIssuerInsideTheCluster,k as WithoutIssuerInsideTheCluster,N as __namedExportsOrder,D as default};