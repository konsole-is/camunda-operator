import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./data-table-CuRj6hK8.js";import{n as i}from"./json-Ch039IR8.js";import{t as a}from"./policy-DfdqtECx.js";import{n as o,r as s,t as c}from"./not-read-RuqQXk-B.js";function l({rows:e,unread:t=[]}){let n=e=>e.environmentUnknown?f:e.environment??d,a=[...[...new Set(e.flatMap(e=>e.environment?[e.environment]:[]))].sort(i),...e.some(e=>!e.environmentUnknown&&!e.environment)?[d]:[],...e.some(e=>e.environmentUnknown)?[f]:[]];return(0,u.jsxs)(`section`,{"aria-label":`Clusters served`,className:`flex flex-col gap-3 rounded-lg border border-border bg-panel p-4`,children:[(0,u.jsx)(`h2`,{className:`font-heading text-base font-semibold`,children:`Clusters served`}),e.length>0&&(0,u.jsx)(c,{kinds:t,tab:!0}),(0,u.jsx)(r,{label:`Clusters served table`,emptyText:e.length>0?void 0:t.length>0?s(t,!0):`This plane lists no clusters yet.`,columns:[{key:`cluster`,label:`Cluster`,sortable:!0},{key:`environment`,label:`Environment`,sortable:!0},{key:`namespace`,label:`Namespace`},{key:`attached`,label:`Attached`},{key:`reason`,label:`Reason`},{key:`message`,label:`Message`}],filters:[{key:`environment`,label:`Environment`,options:a.map(e=>({value:e,label:e}))},{key:`attached`,label:`Attached`,options:[{value:`Attached`,label:`Attached`},{value:`Not attached`,label:`Not attached`}]}],rows:e.map(e=>({id:`${e.namespace}/${e.name}`,label:e.name,href:e.href,search:[e.name,e.namespace,e.reason,e.message].filter(Boolean).join(` `),filters:{environment:n(e),attached:e.attached?`Attached`:`Not attached`},sort:{cluster:e.name,environment:n(e)},cells:{cluster:e.name,environment:e.environment??(0,u.jsx)(`span`,{className:`text-muted`,children:n(e)}),namespace:e.namespace,attached:e.attached?`Yes`:`No`,reason:e.reason??p,message:e.message?(0,u.jsx)(`span`,{className:`block min-w-48 whitespace-normal`,children:e.message}):p}}))})]})}var u,d,f,p;function m(){return(m=e((()=>{u=t(),n(),o(),a(),d=`No environment`,f=`Not known`,p=(0,u.jsx)(`span`,{className:`text-muted`,children:`-`}),l.__docgenInfo={description:`ServedClustersTab is the Clusters served tab of a plane: one row for each
cluster that the plane lists, with filters for the environment and for
attached, and a search. unread names the kinds that the page could not
read. While unread is not empty, the tab never says that the plane lists
no clusters.`,methods:[],displayName:`ServedClustersTab`,props:{rows:{required:!0,tsType:{name:`Array`,elements:[{name:`signature`,type:`object`,raw:`{
  namespace: string;
  name: string;
  environment?: string;
  environmentUnknown?: true;
  attached: boolean;
  reason?: string;
  message?: string;
  href: string;
}`,signature:{properties:[{key:`namespace`,value:{name:`string`,required:!0}},{key:`name`,value:{name:`string`,required:!0}},{key:`environment`,value:{name:`string`,required:!1}},{key:`environmentUnknown`,value:{name:`literal`,value:`true`,required:!1}},{key:`attached`,value:{name:`boolean`,required:!0}},{key:`reason`,value:{name:`string`,required:!1}},{key:`message`,value:{name:`string`,required:!1}},{key:`href`,value:{name:`string`,required:!0}}]}}],raw:`ServedCluster[]`},description:``},unread:{required:!1,tsType:{name:`unknown`},description:``,defaultValue:{value:`[]`,computed:!1}}}}})))()}var h,g,_,v,y,b;function x(){return(x=e((()=>{m(),h=(e,t,n={})=>({namespace:`team-${e}`,name:e,environment:`production`,attached:t,href:`/fleet/clusters/team-${e}/${e}`,...n}),g={title:`planes/ServedClustersTab`,component:l},_={name:`Each reason`,args:{rows:[h(`orders`,!0),h(`modeler`,!0,{reason:`BasicAuthUserFailed`,message:`The management plane serves the cluster, but the Web Modeler user on it is missing`}),h(`billing`,!1,{reason:`ClaimedElsewhere`,message:`The management plane platform/sandbox already serves this cluster`}),h(`search`,!1,{reason:`NotReady`,message:`The cluster publishes no gateway endpoints yet`}),h(`ledger`,!1,{environment:`staging`,reason:`InvalidReference`,message:`The OIDC issuer of the cluster differs from the issuer of the management plane`}),h(`reports`,!1,{reason:`WriteFailed`,message:`The API server refused the write of the Console ping settings`}),h(`lab`,!0,{environment:void 0})]}},v={name:`No clusters`,args:{rows:[]}},y={name:`Plane not read`,args:{rows:[],unread:[`CamundaManagementCluster`]}},b=[`EachReason`,`NoClusters`,`PlaneNotRead`],_.parameters={..._.parameters,docs:{..._.parameters?.docs,source:{originalSource:`{
  name: "Each reason",
  args: {
    rows: [row("orders", true), row("modeler", true, {
      reason: "BasicAuthUserFailed",
      message: "The management plane serves the cluster, but the Web Modeler user on it is missing"
    }), row("billing", false, {
      reason: "ClaimedElsewhere",
      message: "The management plane platform/sandbox already serves this cluster"
    }), row("search", false, {
      reason: "NotReady",
      message: "The cluster publishes no gateway endpoints yet"
    }), row("ledger", false, {
      environment: "staging",
      reason: "InvalidReference",
      message: "The OIDC issuer of the cluster differs from the issuer of the management plane"
    }), row("reports", false, {
      reason: "WriteFailed",
      message: "The API server refused the write of the Console ping settings"
    }), row("lab", true, {
      environment: undefined
    })]
  }
}`,..._.parameters?.docs?.source}}},v.parameters={...v.parameters,docs:{...v.parameters?.docs,source:{originalSource:`{
  name: "No clusters",
  args: {
    rows: []
  }
}`,...v.parameters?.docs?.source}}},y.parameters={...y.parameters,docs:{...y.parameters?.docs,source:{originalSource:`{
  name: "Plane not read",
  args: {
    rows: [],
    unread: ["CamundaManagementCluster"]
  }
}`,...y.parameters?.docs?.source}}}})))()}x();export{_ as EachReason,v as NoClusters,y as PlaneNotRead,b as __namedExportsOrder,g as default};