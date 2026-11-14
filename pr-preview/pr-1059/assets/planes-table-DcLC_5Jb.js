import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{a as r,t as i}from"./hooks-Cuq9NGnJ.js";import{n as a,t as o}from"./data-table-BosTQOYf.js";import{n as s,t as c}from"./status-chip-CwiFx0YW.js";import{i as l,r as u,t as d}from"./plane-CFdfSrrO.js";import{n as f,t as p}from"./not-attached-chip-DJOdfp-U.js";function m({plane:e}){return(0,_.jsxs)(`span`,{className:`flex flex-col`,children:[(0,_.jsx)(`span`,{className:`font-medium`,children:e.name}),(0,_.jsx)(`span`,{className:`text-xs text-muted`,children:e.namespace}),!e.managed&&(0,_.jsx)(`span`,{className:`text-xs text-text-2`,children:`Not managed by the fleet manager`})]})}function h({plane:e}){return`noClusterSelector`in e.serves?(0,_.jsx)(`span`,{children:`No cluster, because it has no cluster selector`}):`environments`in e.serves?(0,_.jsx)(`span`,{children:e.serves.environments.join(`, `)}):(0,_.jsx)(`span`,{className:`font-mono text-xs break-words`,children:e.serves.selector})}function g({plane:e}){let t=e.notAttached.length;return(0,_.jsx)(`span`,{className:`whitespace-nowrap`,children:t>0?`${e.attached} attached · ${t} not attached`:`${e.attached} attached`})}var _;function v(){return(v=e((()=>{_=n(),m.__docgenInfo={description:`PlaneName is the name of plane over its namespace, and says when the fleet manager does not manage it.`,methods:[],displayName:`PlaneName`,props:{plane:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
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
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]}}],raw:`NotAttached[]`,required:!0}},{key:`optimize`,value:{name:`number`,required:!0}},{key:`contract`,value:{name:`string`,required:!1}},{key:`ready`,value:{name:`ConditionLike`,required:!1}}]}},description:``}}},h.__docgenInfo={description:`PlaneServes is the environments of plane, or its raw namespace selector, or
says that it serves no cluster without a cluster selector.`,methods:[],displayName:`PlaneServes`,props:{plane:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
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
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]}}],raw:`NotAttached[]`,required:!0}},{key:`optimize`,value:{name:`number`,required:!0}},{key:`contract`,value:{name:`string`,required:!1}},{key:`ready`,value:{name:`ConditionLike`,required:!1}}]}},description:``}}},g.__docgenInfo={description:`PlaneClusters is the count of the clusters that plane serves, and of those it does not.`,methods:[],displayName:`PlaneClusters`,props:{plane:{required:!0,tsType:{name:`signature`,type:`object`,raw:`{
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
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]}}],raw:`NotAttached[]`,required:!0}},{key:`optimize`,value:{name:`number`,required:!0}},{key:`contract`,value:{name:`string`,required:!1}},{key:`ready`,value:{name:`ConditionLike`,required:!1}}]}},description:``}}}})))()}function y(e){let t=r(`CamundaManagementCluster`,{},e.clusters),n=r(`CamundaManagementClusterClaim`,{},e.claims),i=r(`Namespace`,{},e.namespaces),a=r(`Environment`,{},e.environments);return{planes:(0,S.useMemo)(()=>l({clusters:t.items,claims:n.items,namespaces:i.items,environments:a.items}),[t.items,n.items,i.items,a.items]),errors:[[`CamundaManagementCluster`,t],[`CamundaManagementClusterClaim`,n],[`Namespace`,i],[`Environment`,a]].flatMap(([e,t])=>t.error?[{kind:e,message:t.error}]:[])}}function b({planes:e,environment:t,toolbar:n=!0}){let r=u(e,t);return r.length===0?null:(0,x.jsx)(o,{label:`Management planes`,toolbar:n,columns:[{key:`plane`,label:`Management plane`},{key:`status`,label:`Status`},{key:`identity`,label:`Identity`},{key:`serves`,label:`Serves`},{key:`clusters`,label:`Clusters`},{key:`warnings`,label:`Warnings`}],rows:r.map(e=>({id:e.id,label:e.name,href:`/fleet/planes/${e.namespace}/${e.name}`,search:`${e.name} ${e.namespace}`,cells:{plane:(0,x.jsx)(m,{plane:e}),status:(0,x.jsx)(c,{condition:e.ready,readyReason:!0}),identity:e.mode??`—`,serves:(0,x.jsx)(h,{plane:e}),clusters:(0,x.jsx)(g,{plane:e}),warnings:(0,x.jsx)(p,{clusters:e.notAttached})}}))})}var x,S;function C(){return(C=e((()=>{x=n(),S=t(),a(),s(),i(),v(),f(),d(),b.__docgenInfo={description:`PlanesTable shows the planes of planesIn(planes, environment), or nothing
when there are none. With toolbar false it has no search box.`,methods:[],displayName:`PlanesTable`,props:{planes:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
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
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}}]}}],raw:`NotAttached[]`,required:!0}},{key:`optimize`,value:{name:`number`,required:!0}},{key:`contract`,value:{name:`string`,required:!1}},{key:`ready`,value:{name:`ConditionLike`,required:!1}}]}}],raw:`Plane[]`},description:``},environment:{required:!1,tsType:{name:`union`,raw:`string | null`,elements:[{name:`string`},{name:`null`}]},description:``},toolbar:{required:!1,tsType:{name:`boolean`},description:``,defaultValue:{value:`true`,computed:!1}}}}})))()}export{m as a,g as i,C as n,h as o,y as r,v as s,b as t};