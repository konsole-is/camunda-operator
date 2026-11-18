import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{f as t,n,p as r,r as i,t as a}from"./preview-Bt74LHbP.js";import{i as o,n as s,r as c,t as l}from"./template-preview-D775VIZB.js";import{n as u,r as d}from"./fixtures-CnXBPwFn.js";var f,p,m,h,g,_,v;function y(){return(y=e((()=>{c(),t(),u(),n(),s(),f=d(),p=f.environments[0],m=f.registrations.find(e=>e.metadata.name===`aws-connectors-role`),h=f.sections.find(e=>e.metadata.name===`connectors-permissions`),g={title:`setup/environments/TemplatePreview`,component:l},_={name:`Inline policy`,args:{claims:i(f,`production`,`aws-connectors-role`),empty:a(f,`production`),render:e=>r(p.spec.parameters?.[`aws-connectors-role`]?.inlinePolicy?.value,o({claim:e,section:h,registration:m,environment:p,registrations:f.registrations,requests:f.requests}),`parameter`)}},v=[`InlinePolicy`],_.parameters={..._.parameters,docs:{..._.parameters?.docs,source:{originalSource:`{
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
}`,..._.parameters?.docs?.source}}}})))()}y();export{_ as InlinePolicy,v as __namedExportsOrder,g as default};