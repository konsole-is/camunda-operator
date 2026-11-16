import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{C as n,E as r,_ as i,b as a,f as o,g as s,h as c,m as l}from"./release-DN3G17Sp.js";import{a as u,o as d}from"./fields-CFhEjS2t.js";import{a as f,i as p,n as m,r as h}from"./claim-tab-DyENXb9O.js";import{d as g,i as _,n as v,u as y}from"./values-9f-8xfft.js";function b(e,t,n){let s=e.spec.parameters;if(s!==void 0&&!_(s))return{error:`The parameter schema is not an object.`};let l=s?.properties;if(l!==void 0&&!_(l))return{error:`The properties of the parameter schema are not an object.`};let u=i(e,void 0);if(!(`value`in u))return u;let d=[];if(!n.includes(`Environment`))for(let n of t){let t=i(e,n);if(!(`value`in t))return t;d.push({environment:n.spec.displayName||(n.metadata.name??``),policies:t.value})}let f=o(e)?.hidden??[];return{value:[...new Set([...Object.keys(l??{}),...Object.keys(u.value)])].filter(e=>!f.includes(e)).sort(a).map(e=>{let t=r(u.value,e);return{name:e,...x(l?r(l,e):void 0),...t&&c(t)&&{fallback:t.value},claimEditable:t?.claimEditable===!0,...!n.includes(`Environment`)&&{environments:d.map(({environment:t,policies:n})=>{let i=r(n,e);return{environment:t,value:i&&c(i)?i.value:void 0,claimEditable:i?.claimEditable===!0}})}}})}}function x(e){if(e===void 0)return{};let t=u(e);return!t||!_(e)?{error:`The schema of this parameter is not an object.`}:{...typeof e.description==`string`&&{description:e.description},...typeof e.type==`string`&&{type:e.type},...t.minimum!==void 0&&{minimum:t.minimum},...t.maximum!==void 0&&{maximum:t.maximum},...t.enum&&{values:t.enum}}}function S(){return(S=e((()=>{d(),l(),n(),s(),v()})))()}function C({rows:e,unread:t}){return`value`in e?e.value.length===0?(0,E.jsx)(`p`,{className:`text-sm text-muted`,children:`This option takes no parameters.`}):(0,E.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[t.includes(`Environment`)&&(0,E.jsx)(`p`,{className:`text-sm text-muted`,children:`Values of the environments: Not known: Environment is not read.`}),(0,E.jsx)(`div`,{className:`grid grid-cols-1 gap-4 lg:grid-cols-2`,children:e.value.map(e=>(0,E.jsx)(m,{label:e.name,children:e.error?(0,E.jsx)(y,{text:e.error}):(0,E.jsxs)(E.Fragment,{children:[e.description&&(0,E.jsx)(`p`,{className:`text-sm text-text-2`,children:e.description}),(0,E.jsxs)(p,{children:[(0,E.jsx)(h,{label:`Type`,children:e.type??`Any`}),w(e)&&(0,E.jsx)(h,{label:`Range`,children:w(e)}),e.values&&(0,E.jsx)(h,{label:`Values`,children:e.values.join(`, `)}),(0,E.jsx)(h,{label:`Default`,children:(0,E.jsx)(T,{value:e.fallback,none:`None`})}),(0,E.jsx)(h,{label:`A claim can change it`,children:e.claimEditable?`Yes`:`No`}),e.environments?.map(e=>(0,E.jsx)(h,{label:e.environment,note:e.claimEditable?`A claim can change it`:void 0,children:(0,E.jsx)(T,{value:e.value,none:`No value`})},e.environment))]})]})},e.name))})]}):(0,E.jsx)(y,{text:`error`in e?e.error:e.refusal.message})}function w({minimum:e,maximum:t}){if(e!==void 0&&t!==void 0)return`${e} to ${t}`;if(e!==void 0)return`At least ${e}`;if(t!==void 0)return`At most ${t}`}function T({value:e,none:t}){return e===void 0?(0,E.jsx)(`span`,{className:`text-muted`,children:t}):(0,E.jsx)(`span`,{className:`font-mono break-all`,children:JSON.stringify(e)})}var E;function D(){return(D=e((()=>{E=t(),f(),g(),C.__docgenInfo={description:`ParametersTab shows each parameter of rows in its own panel: its type,
its range or values, its default, whether a claim can change it, and its
value in each Environment. While Environment is in unread, it says that
the values of the environments are not known.`,methods:[],displayName:`ParametersTab`,props:{rows:{required:!0,tsType:{name:`PolicyResult`,elements:[{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  name: string;
  error?: string;
  description?: string;
  /** type is the type of the property schema, as written there. */
  type?: string;
  minimum?: number;
  maximum?: number;
  values?: readonly string[];
  /** fallback is the value without an Environment: parameterValues of the registration. */
  fallback?: unknown;
  claimEditable: boolean;
  /** environments is unset while Environment is not read. */
  environments?: {
    environment: string;
    value?: unknown;
    claimEditable: boolean;
  }[];
}`,signature:{properties:[{key:`name`,value:{name:`string`,required:!0}},{key:`error`,value:{name:`string`,required:!1}},{key:`description`,value:{name:`string`,required:!1}},{key:`type`,value:{name:`string`,required:!1},description:`type is the type of the property schema, as written there.`},{key:`minimum`,value:{name:`number`,required:!1}},{key:`maximum`,value:{name:`number`,required:!1}},{key:`values`,value:{name:`unknown`,required:!1}},{key:`fallback`,value:{name:`unknown`,required:!1},description:`fallback is the value without an Environment: parameterValues of the registration.`},{key:`claimEditable`,value:{name:`boolean`,required:!0}},{key:`environments`,value:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  environment: string;
  value?: unknown;
  claimEditable: boolean;
}`,signature:{properties:[{key:`environment`,value:{name:`string`,required:!0}},{key:`value`,value:{name:`unknown`,required:!1}},{key:`claimEditable`,value:{name:`boolean`,required:!0}}]}}],raw:`{
  environment: string;
  value?: unknown;
  claimEditable: boolean;
}[]`,required:!1},description:`environments is unset while Environment is not read.`}]}}],raw:`ParameterRow[]`}],raw:`PolicyResult<ParameterRow[]>`},description:``},unread:{required:!0,tsType:{name:`unknown`},description:``}}}})))()}export{b as i,D as n,S as r,C as t};