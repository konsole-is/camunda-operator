import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,i as r,n as i,o as a,r as o,s,t as c}from"./table-DE4FSmwW.js";function l({permissions:e}){return`error`in e?(0,u.jsxs)(`p`,{role:`alert`,className:`text-sm text-bad`,children:[`Could not read the permissions: `,e.error]}):(0,u.jsxs)(`div`,{className:`space-y-2`,children:[(0,u.jsxs)(c,{label:`Permissions of the fleet manager`,children:[(0,u.jsx)(n,{children:(0,u.jsxs)(a,{children:[(0,u.jsx)(r,{children:`API group`}),(0,u.jsx)(r,{children:`Resources`}),(0,u.jsx)(r,{children:`Names`}),(0,u.jsx)(r,{children:`Verbs`})]})}),(0,u.jsx)(i,{children:e.rules.map(e=>(0,u.jsxs)(a,{children:[(0,u.jsx)(o,{children:e.apiGroups.map(e=>e||`core`).join(`, `)}),(0,u.jsx)(o,{children:e.resources.join(`, `)}),(0,u.jsx)(o,{className:e.resourceNames.length?``:`text-muted`,children:e.resourceNames.length?e.resourceNames.join(`, `):`any`}),(0,u.jsx)(o,{children:e.verbs.join(`, `)})]},`${e.apiGroups.join(`,`)}|${e.resources.join(`,`)}|${e.resourceNames.join(`,`)}|${e.verbs.join(`,`)}`))})]}),e.incomplete&&(0,u.jsx)(`p`,{className:`text-sm text-warn`,children:`The API server could not list every rule.`})]})}var u;function d(){return(d=e((()=>{u=t(),s(),l.__docgenInfo={description:`PermissionsTable shows the rules of the app, read only. The chart sets
them. An error shows as an alert in place of the table.`,methods:[],displayName:`PermissionsTable`,props:{permissions:{required:!0,tsType:{name:`union`,raw:`AppPermissions | { error: string }`,elements:[{name:`AppPermissions`},{name:`signature`,type:`object`,raw:`{ error: string }`,signature:{properties:[{key:`error`,value:{name:`string`,required:!0}}]}}]},description:``}}}})))()}var f,p,m,h,g,_;function v(){return(v=e((()=>{d(),f=[{apiGroups:[``],resources:[`configmaps`],resourceNames:[`fleet-manager-settings`],verbs:[`get`,`update`,`patch`]},{apiGroups:[``],resources:[`secrets`],resourceNames:[],verbs:[`get`,`list`,`watch`]},{apiGroups:[`fleet.camunda.io`],resources:[`camundaclusterclaims`,`environments`],resourceNames:[],verbs:[`create`,`delete`,`get`,`list`,`patch`,`update`,`watch`]}],p={title:`setup/PermissionsTable`,component:l,args:{permissions:{rules:f,incomplete:!1}}},m={name:`Rules`},h={name:`Incomplete`,args:{permissions:{rules:f,incomplete:!0}}},g={name:`Read failed`,args:{permissions:{error:`selfsubjectrulesreviews.authorization.k8s.io is forbidden: User "system:serviceaccount:fleet:fleet-manager" cannot create resource`}}},_=[`Rules`,`Incomplete`,`ReadFailed`],m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "Rules"
}`,...m.parameters?.docs?.source}}},h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
  name: "Incomplete",
  args: {
    permissions: {
      rules,
      incomplete: true
    }
  }
}`,...h.parameters?.docs?.source}}},g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "Read failed",
  args: {
    permissions: {
      error: 'selfsubjectrulesreviews.authorization.k8s.io is forbidden: User "system:serviceaccount:fleet:fleet-manager" cannot create resource'
    }
  }
}`,...g.parameters?.docs?.source}}}})))()}v();export{h as Incomplete,g as ReadFailed,m as Rules,_ as __namedExportsOrder,p as default};