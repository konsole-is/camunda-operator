import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./about-tab-DkvomBQQ.js";import{i as r,l as i,o as a}from"./fixtures-kgATjJaf.js";var o,s,c,l,u,d;function f(){return(f=e((()=>{t(),r(),o=i(),s=a(`aws-kms`,{label:`New KMS key`,description:`Makes a customer managed KMS key for the volumes of the cluster.`,docsUrl:`https://example.com/docs/aws-kms`,controller:{name:`aws-kms`,version:`0.4.1`},serviceAccount:{name:`aws-kms`,namespace:`camunda-plugins`},limits:[{label:`Regions`,value:`eu-west-1, eu-central-1`,description:`A key can only be made in these regions.`},{label:`Permissions boundary`,value:`cpo-plugin-boundary`}]}),c={title:`setup/plugins/AboutTab`,component:n,args:{registration:s,registrations:[s],unread:[]}},l={name:`With limits`},u={name:`Without limits`,args:{registration:o.registrations.find(e=>e.metadata.name===`elasticsearch`),registrations:o.registrations}},d=[`WithLimits`,`WithoutLimits`],l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "With limits"
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "Without limits",
  args: {
    registration: sources.registrations.find(r => r.metadata.name === "elasticsearch")!,
    registrations: sources.registrations
  }
}`,...u.parameters?.docs?.source}}}})))()}f();export{l as WithLimits,u as WithoutLimits,d as __namedExportsOrder,c as default};