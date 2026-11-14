import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{a as t,f as n,n as r,p as i,r as a,t as o}from"./preview-BK2C0ifv.js";import{n as s,t as c}from"./template-preview-RhI46vLv.js";import{n as l,r as u}from"./fixtures-ODd7kMxO.js";var d,f,p,m,h,g,_;function v(){return(v=e((()=>{n(),l(),r(),s(),d=u(),f=d.environments[0],p=d.registrations.find(e=>e.metadata.name===`aws-connectors-role`),m=d.sections.find(e=>e.metadata.name===`connectors-permissions`),h={title:`setup/environments/TemplatePreview`,component:c},g={name:`Inline policy`,args:{claims:a(d,`production`,`aws-connectors-role`),empty:o(d,`production`),render:e=>i(f.spec.parameters?.[`aws-connectors-role`]?.inlinePolicy?.value,t({claim:e,section:m,registration:p,environment:f,registrations:d.registrations,requests:d.requests}),`parameter`)}},_=[`InlinePolicy`],g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "Inline policy",
  args: {
    claims: previewClaims(sources, "production", "aws-connectors-role"),
    empty: emptyPreviewText(sources, "production"),
    render: claim => renderJSON(production.spec.parameters?.["aws-connectors-role"]?.inlinePolicy?.value, templateData({
      claim,
      section: permissions,
      registration: role,
      environment: production,
      registrations: sources.registrations,
      requests: sources.requests
    }), "parameter")
  }
}`,...g.parameters?.docs?.source}}}})))()}v();export{g as InlinePolicy,_ as __namedExportsOrder,h as default};