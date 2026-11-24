import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{c as t,i as n,o as r,t as i}from"./fixtures-kgATjJaf.js";import{n as a,t as o}from"./health-tab-DdO6CLK0.js";var s,c,l,u,d;function f(){return(f=e((()=>{n(),a(),s={title:`setup/plugins/HealthTab`,component:o,args:{registration:r(`aws-kms`,{},[i(`Accepted`,`True`,`Healthy`,`All checks passed`),i(`Ready`,`True`,`Healthy`,`The controller can assume its role`)]),events:[t(`aws-kms`,`Normal`,`Accepted`,`The registration is accepted`)],unread:[],now:`2026-11-13T09:00:00Z`}},c={name:`Healthy`},l={name:`Not ready with a warning event`,args:{registration:r(`aws-kms`,{},[i(`Accepted`,`True`,`Healthy`,`All checks passed`),i(`Ready`,`False`,`CredentialsInvalid`,`The controller cannot assume the role arn:aws:iam::123456789012:role/cpo-aws-kms`)]),events:[t(`aws-kms`,`Warning`,`CredentialsInvalid`,`The controller cannot assume its role`)]}},u={name:`No events`,args:{events:[]}},d=[`Healthy`,`NotReady`,`NoEvents`],c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Healthy"
}`,...c.parameters?.docs?.source}}},l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Not ready with a warning event",
  args: {
    registration: pluginRegistration("aws-kms", {}, [condition("Accepted", "True", "Healthy", "All checks passed"), condition("Ready", "False", "CredentialsInvalid", "The controller cannot assume the role arn:aws:iam::123456789012:role/cpo-aws-kms")]),
    events: [registrationEvent("aws-kms", "Warning", "CredentialsInvalid", "The controller cannot assume its role")]
  }
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "No events",
  args: {
    events: []
  }
}`,...u.parameters?.docs?.source}}}})))()}f();export{c as Healthy,u as NoEvents,l as NotReady,d as __namedExportsOrder,s as default};