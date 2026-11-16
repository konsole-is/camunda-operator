import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./grant-preview-C0xm53s1.js";var r,i,a,o,s,c,l;function u(){return(u=e((()=>{t(),r={section:`backup-storage`,sectionLabel:`Backup storage`,registration:`s3`,optionLabel:`S3 bucket`,template:`backup-access`,description:`Lets the cluster read and write its backups.`,result:{value:{Effect:`Allow`,Action:[`s3:GetObject`,`s3:PutObject`],Resource:`<bucket ARN from Backup storage>`}}},i={section:`connectors-permissions`,sectionLabel:`Connector permissions`,registration:`aws-connectors-role`,optionLabel:`AWS role for connectors`,template:`policies`,result:{value:{Effect:`Allow`,Action:`sts:AssumeRole`,Policy:`<policy: SQS access>`}}},a={title:`create/GrantPreviewList`,component:n},o={name:`Picked choice and placeholder`,args:{previews:[r,i]}},s={name:`Template error`,args:{previews:[r,{...i,result:{error:`parameter "managedPolicyArns" is a list: a template prints only a string, a number, or a boolean`,field:`parameters.managedPolicyArns`,path:`Policy`}}]}},c={name:`Registrations could not be read`,args:{previews:[r],unread:[`PluginRegistration`]}},l=[`PickedChoiceAndPlaceholder`,`TemplateError`,`RegistrationsUnread`],o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "Picked choice and placeholder",
  args: {
    previews: [read, connectors]
  }
}`,...o.parameters?.docs?.source}}},s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Template error",
  args: {
    previews: [read, {
      ...connectors,
      result: {
        error: 'parameter "managedPolicyArns" is a list: a template prints only a string, a number, or a boolean',
        field: "parameters.managedPolicyArns",
        path: "Policy"
      }
    }]
  }
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Registrations could not be read",
  args: {
    previews: [read],
    unread: ["PluginRegistration"]
  }
}`,...c.parameters?.docs?.source}}}})))()}u();export{o as PickedChoiceAndPlaceholder,c as RegistrationsUnread,s as TemplateError,l as __namedExportsOrder,a as default};