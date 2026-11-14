import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./data-table-DceIS0m9.js";import{n as i,t as a}from"./link-DOLMGIR2.js";import{r as o,t as s}from"./button-3gXZFVT7.js";import{a as c,o as l}from"./status-CBDUVTbC.js";import{n as u,t as d}from"./status-chip-DikExibM.js";import{n as f,t as p}from"./plane-CFdfSrrO.js";import{a as m,i as h,n as g,o as _,s as v}from"./planes-table-CvkKb73s.js";import{n as y,t as b}from"./not-attached-chip-BYFMP5TL.js";import{r as x,t as S}from"./fixtures-ac7SM6IH.js";function C({planes:e,errors:t=[]}){return(0,w.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,w.jsxs)(`div`,{className:`flex flex-wrap items-center justify-between gap-3`,children:[(0,w.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em]`,children:`Management planes`}),(0,w.jsx)(s,{asChild:!0,children:(0,w.jsx)(i,{href:`/fleet/planes/new`,children:`New management plane`})})]}),t.map(e=>(0,w.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind)),(0,w.jsx)(r,{label:`Management planes`,emptyText:e.length===0?`No management plane yet.`:void 0,columns:[{key:`plane`,label:`Management plane`,sortable:!0},{key:`identity`,label:`Identity`},{key:`components`,label:`Components`},{key:`serves`,label:`Serves`},{key:`clusters`,label:`Clusters`,sortable:!0},{key:`optimize`,label:`Optimize`},{key:`contract`,label:`Contract`},{key:`status`,label:`Status`,sortable:!0},{key:`warnings`,label:`Warnings`}],filters:[{key:`mode`,label:`Identity`,options:T.map(e=>({value:e,label:e}))},{key:`status`,label:`Status`,options:E.map(e=>({value:e,label:e}))}],rows:e.map(e=>{let t=`${e.namespace}/${e.name}`,n=c(e.ready).label;return{id:e.id,label:e.name,href:`/fleet/planes/${t}`,search:[e.name,e.namespace,`environments`in e.serves?e.serves.environments.join(` `):e.serves.selector].join(` `),sort:{plane:t,clusters:e.attached,status:n},filters:{mode:e.mode??``,status:n},cells:{plane:(0,w.jsx)(m,{plane:e}),identity:e.mode??`—`,components:e.components.join(`, `),serves:(0,w.jsx)(_,{plane:e}),clusters:(0,w.jsx)(h,{plane:e}),optimize:f(e),contract:e.contract??`—`,status:(0,w.jsx)(d,{condition:e.ready,readyReason:!0}),warnings:(0,w.jsx)(b,{clusters:e.notAttached})}}})})]})}var w,T,E;function D(){return(D=e((()=>{w=t(),a(),n(),l(),u(),o(),v(),y(),p(),g(),T=[`Keycloak`,`External Keycloak`,`OIDC`],E=[`Ready`,`Not ready`,`Unknown`,`No status`],C.__docgenInfo={description:`PlaneList is the page of every management plane, with search, filters,
sort, and pages. Each of errors shows above the table, and the rows stay.`,methods:[],displayName:`PlaneList`,props:{planes:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  id: string;
  namespace: string;
  name: string;
  managed: boolean;
  mode?: IdentityMode;
  components: string[];
  serves:
    { environments: string[] } | { selector: string; noClusterSelector?: true };
  servedEnvironments: string[];
  servesNoEnvironment?: boolean;
  attached: number;
  notAttached: NotAttached[];
  optimize: number;
  contract?: string;
  ready?: ConditionLike;
}`,signature:{properties:[{key:`id`,value:{name:`string`,required:!0}},{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`managed`,value:{name:`boolean`,required:!0}},{key:`mode`,value:{name:`union`,raw:`"Keycloak" | "External Keycloak" | "OIDC"`,elements:[{name:`literal`,value:`"Keycloak"`},{name:`literal`,value:`"External Keycloak"`},{name:`literal`,value:`"OIDC"`}],required:!1}},{key:`components`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}},{key:`serves`,value:{name:`union`,raw:`{ environments: string[] } | { selector: string; noClusterSelector?: true }`,elements:[{name:`signature`,type:`object`,raw:`{ environments: string[] }`,signature:{properties:[{key:`environments`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}}]}},{name:`signature`,type:`object`,raw:`{ selector: string; noClusterSelector?: true }`,signature:{properties:[{key:`selector`,value:{name:`string`,required:!0}},{key:`noClusterSelector`,value:{name:`literal`,value:`true`,required:!1}}]}}],required:!0}},{key:`servedEnvironments`,value:{name:`Array`,elements:[{name:`string`}],raw:`string[]`,required:!0}},{key:`servesNoEnvironment`,value:{name:`boolean`,required:!1}},{key:`attached`,value:{name:`number`,required:!0}},{key:`notAttached`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  namespace: string;
  name: string;
  reason?: string;
  message?: string;
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]}}],raw:`NotAttached[]`,required:!0}},{key:`optimize`,value:{name:`number`,required:!0}},{key:`contract`,value:{name:`string`,required:!1}},{key:`ready`,value:{name:`ConditionLike`,required:!1}}]}}],raw:`Plane[]`},description:``},errors:{required:!1,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{ kind: string; message: string }`,signature:{properties:[{key:`kind`,value:{name:`string`,required:!0}},{key:`message`,value:{name:`string`,required:!0}}]}}],raw:`ReadError[]`},description:``,defaultValue:{value:`[]`,computed:!1}}}}})))()}var O,k,A,j;function M(){return(M=e((()=>{S(),D(),O={title:`planes/PlaneList`,component:C},k={name:`Mixed planes`,args:{planes:x}},A={name:`No planes`,args:{planes:[]}},j=[`MixedPlanes`,`NoPlanes`],k.parameters={...k.parameters,docs:{...k.parameters?.docs,source:{originalSource:`{
  name: "Mixed planes",
  args: {
    planes
  }
}`,...k.parameters?.docs?.source}}},A.parameters={...A.parameters,docs:{...A.parameters?.docs,source:{originalSource:`{
  name: "No planes",
  args: {
    planes: []
  }
}`,...A.parameters?.docs?.source}}}})))()}M();export{k as MixedPlanes,A as NoPlanes,j as __namedExportsOrder,O as default};