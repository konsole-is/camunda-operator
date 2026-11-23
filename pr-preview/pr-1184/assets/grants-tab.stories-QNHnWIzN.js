import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,r as n}from"./grants-tab-CkXeYwQF.js";import{a as r,i,o as a,t as o}from"./fixtures-kgATjJaf.js";var s,c,l,u,d,f,p,m,h,g,_;function v(){return(v=e((()=>{i(),n(),s=a(`aws-s3`,{grantTemplates:[{name:`bucket-access`,description:`Lets the cluster read and write its backup bucket.`,format:`aws-iam-statement`,template:{Effect:`Allow`,Action:[`s3:GetObject`,`s3:PutObject`,`s3:DeleteObject`],Resource:`arn:aws:s3:::{{ .outputs.bucket }}/*`},requiresInputs:[`principal`]}]}),c=(e,t,n)=>r(`team-orders`,e,{template:t,cluster:{name:`orders`}},{"plugins.camunda.io/registration":`aws-s3`,"plugins.camunda.io/request":`orders-backup-storage`},n),l=[o(`Applied`,`True`,`Healthy`),o(`MatchesTemplate`,`True`,`Healthy`),o(`Ready`,`True`,`Healthy`)],u=(e,t,n)=>c(e,`bucket-access`,[o(`Applied`,`False`,t,n),o(`MatchesTemplate`,`True`,`Healthy`),o(`Ready`,`False`,t,n)]),d={title:`setup/plugins/GrantsTab`,component:t,args:{registration:s,grants:[c(`orders-bucket-access`,`bucket-access`,l),u(`billing-bucket-access`,`OverCeiling`,`s3:DeleteObject is outside the ceiling of the identity`),c(`payments-bucket-access`,`bucket-access`,[])],unread:[],now:`2026-11-13T09:00:00Z`}},f={name:`One template with a ready, a refused, and a waiting Grant`},p={name:`Each refusal reason`,args:{grants:[u(`a-bucket-access`,`OverCeiling`,`s3:DeleteObject is outside the ceiling`),u(`b-bucket-access`,`OverSizeLimit`,`The policy would be 6144 bytes`),u(`c-bucket-access`,`Rejected`,`MalformedPolicyDocument: invalid ARN`),c(`d-bucket-access`,`bucket-access`,[o(`Applied`,`True`,`Healthy`),o(`MatchesTemplate`,`False`,`TemplateChanged`,`The template changed`),o(`Ready`,`False`,`TemplateChanged`,`The template changed`)])]}},m={name:`No grant templates`,args:{registration:a(`rustfs`,{}),grants:[]}},h={name:`Grant of a removed template`,args:{grants:[c(`orders-bucket-access`,`bucket-access`,l),c(`orders-kms-access`,`kms-access`,[o(`Ready`,`False`,`TemplateChanged`,`The template is gone`)])]}},g={name:`Grant not read`,args:{unread:[`Grant`]}},_=[`OneTemplate`,`EachReason`,`NoTemplates`,`RemovedTemplate`,`GrantNotRead`],f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "One template with a ready, a refused, and a waiting Grant"
}`,...f.parameters?.docs?.source}}},p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Each refusal reason",
  args: {
    grants: [refused("a-bucket-access", "OverCeiling", "s3:DeleteObject is outside the ceiling"), refused("b-bucket-access", "OverSizeLimit", "The policy would be 6144 bytes"), refused("c-bucket-access", "Rejected", "MalformedPolicyDocument: invalid ARN"), grant("d-bucket-access", "bucket-access", [condition("Applied", "True", "Healthy"), condition("MatchesTemplate", "False", "TemplateChanged", "The template changed"), condition("Ready", "False", "TemplateChanged", "The template changed")])]
  }
}`,...p.parameters?.docs?.source}}},m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "No grant templates",
  args: {
    registration: pluginRegistration("rustfs", {}),
    grants: []
  }
}`,...m.parameters?.docs?.source}}},h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
  name: "Grant of a removed template",
  args: {
    grants: [grant("orders-bucket-access", "bucket-access", healthy), grant("orders-kms-access", "kms-access", [condition("Ready", "False", "TemplateChanged", "The template is gone")])]
  }
}`,...h.parameters?.docs?.source}}},g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "Grant not read",
  args: {
    unread: ["Grant"]
  }
}`,...g.parameters?.docs?.source}}}})))()}v();export{p as EachReason,g as GrantNotRead,m as NoTemplates,f as OneTemplate,h as RemovedTemplate,_ as __namedExportsOrder,d as default};