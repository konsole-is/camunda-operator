import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{t as n}from"./hooks-Bys_naRE.js";import{n as r,t as i}from"./data-table-DjtcvAuE.js";import{n as a,t as o}from"./link-DOLMGIR2.js";import{r as s,t as c}from"./button-Ex6AYQb6.js";import{n as l,r as u,t as d}from"./not-read-RuqQXk-B.js";import{o as f,s as p}from"./status-B-qMFQRy.js";import{n as m,t as h}from"./status-chip-Dhkg3JlN.js";import{o as g}from"./unread-Do34-4tt.js";import{n as _,t as v}from"./plane-Cu84L1lC.js";import{a as y,i as b,n as x,o as S,s as C}from"./planes-table-D6EvfbhX.js";import{n as w,t as T}from"./not-attached-chip-C0Ux39hP.js";import{t as E}from"./answered-CQp4Rcoz.js";import{r as D,t as O}from"./fixtures-BSgMhAb3.js";function k({planes:e,errors:t=[],stale:n=[]}){let r=g({errors:t,stale:n}).filter(e=>M.includes(e));return(0,A.jsxs)(`div`,{className:`flex flex-col gap-6`,children:[(0,A.jsxs)(`div`,{className:`flex flex-wrap items-center justify-between gap-3`,children:[(0,A.jsx)(`h1`,{className:`font-heading text-3xl font-bold tracking-[-0.02em]`,children:`Management planes`}),(0,A.jsx)(c,{asChild:!0,children:(0,A.jsx)(a,{href:`/fleet/planes/new`,children:`New management plane`})})]}),t.map(e=>(0,A.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read `,e.kind,`: `,e.message]},e.kind)),e.length>0&&(0,A.jsx)(d,{kinds:n.filter(e=>!t.some(t=>t.kind===e))}),(0,A.jsx)(i,{label:`Management planes`,emptyText:r.length>0?u(r):e.length>0?void 0:`No management plane yet.`,columns:[{key:`plane`,label:`Management plane`,sortable:!0},{key:`identity`,label:`Identity`},{key:`components`,label:`Components`},{key:`serves`,label:`Serves`},{key:`clusters`,label:`Clusters`,sortable:!0},{key:`optimize`,label:`Optimize`},{key:`contract`,label:`Contract`},{key:`status`,label:`Status`,sortable:!0},{key:`warnings`,label:`Warnings`}],filters:[{key:`mode`,label:`Identity`,options:j.map(e=>({value:e,label:e}))},{key:`status`,label:`Status`,options:N.map(e=>({value:e,label:e}))}],rows:e.map(e=>{let t=`${e.namespace}/${e.name}`,n=f(e.ready).label;return{id:e.id,label:e.name,href:`/fleet/planes/${t}`,search:[e.name,e.namespace,`environments`in e.serves?e.serves.environments.join(` `):e.serves.selector].join(` `),sort:{plane:t,clusters:e.attached,status:n},filters:{mode:e.mode??``,status:n},cells:{plane:(0,A.jsx)(y,{plane:e}),identity:e.mode??`—`,components:e.components.join(`, `),serves:(0,A.jsx)(S,{plane:e}),clusters:(0,A.jsx)(b,{plane:e}),optimize:_(e),contract:e.contract??`—`,status:(0,A.jsx)(h,{condition:e.ready,readyReason:!0}),warnings:(0,A.jsx)(T,{clusters:e.notAttached})}}})})]})}var A,j,M,N;function P(){return(P=e((()=>{A=t(),o(),r(),l(),p(),m(),s(),E(),n(),C(),w(),v(),x(),j=[`Keycloak`,`External Keycloak`,`OIDC`],M=[`CamundaManagementCluster`,`CamundaManagementClusterClaim`],N=[`Ready`,`Not ready`,`Unknown`,`No status`],k.__docgenInfo={description:`PlaneList is the page of every management plane, with search, filters,
sort, and pages. Each of errors shows above the table, and the rows stay.
stale holds the kinds whose list is not current.`,methods:[],displayName:`PlaneList`,props:{planes:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
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
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]}}],raw:`NotAttached[]`,required:!0}},{key:`optimize`,value:{name:`number`,required:!0}},{key:`contract`,value:{name:`string`,required:!1}},{key:`ready`,value:{name:`ConditionLike`,required:!1}}]}}],raw:`Plane[]`},description:``},errors:{required:!1,tsType:{name:`Array`,elements:[{name:`ReadError`}],raw:`ReadError[]`},description:``,defaultValue:{value:`[]`,computed:!1}},stale:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}}}}})))()}var F,I,L,R;function z(){return(z=e((()=>{O(),P(),F={title:`planes/PlaneList`,component:k},I={name:`Mixed planes`,args:{planes:D}},L={name:`No planes`,args:{planes:[]}},R=[`MixedPlanes`,`NoPlanes`],I.parameters={...I.parameters,docs:{...I.parameters?.docs,source:{originalSource:`{
  name: "Mixed planes",
  args: {
    planes
  }
}`,...I.parameters?.docs?.source}}},L.parameters={...L.parameters,docs:{...L.parameters?.docs,source:{originalSource:`{
  name: "No planes",
  args: {
    planes: []
  }
}`,...L.parameters?.docs?.source}}}})))()}z();export{I as MixedPlanes,L as NoPlanes,R as __namedExportsOrder,F as default};