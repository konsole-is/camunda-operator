import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{_ as r,g as i}from"./draft-DzeiDakb.js";import{t as a}from"./session-yRWjmJHF.js";import{n as o,t as s}from"./attachment-fields-BwSU46R2.js";function c(e){let[t,n]=(0,u.useState)({spec:{attachments:e.value}});return(0,l.jsx)(s,{...e,value:t.spec?.attachments,set:(t,r)=>{e.set(t,r),n(e=>i(e,t,r))},reset:t=>{e.reset(t),n(e=>r(e,t))}})}var l,u,d,f,p,m,h,g,_,v;function y(){return(y=e((()=>{l=n(),u=t(),a(),o(),{fn:d}=__STORYBOOK_MODULE_TEST__,f={reason:`filled`,section:`access`,sectionLabel:`Access`},p=e=>{let t=e.at(-1);return t&&`field`in t&&t.field===`externalUrl`?f:void 0},m={title:`cluster/AttachmentFields`,component:s,args:{value:void 0,set:d(),reset:d(),invalid:d(),context:{secondaryStorage:`elasticsearch`,backupStorage:!0,managementAuthConfigs:[`central`,`keycloak`],lock:()=>void 0,errors:[],pending:0}},decorators:[e=>(0,l.jsx)(`div`,{className:`max-w-3xl`,children:(0,l.jsx)(e,{})})],render:e=>(0,l.jsx)(c,{...e},JSON.stringify(e.value))},h={name:`Both on`,args:{value:{optimize:{managementAuthConfigRef:`central`,externalUrl:`https://optimize.orders.example.com`},backupSchedule:{schedule:`0 2 * * *`,retained:{completed:7,failed:2}}}}},g={name:`Locked URL`,args:{value:{optimize:{managementAuthConfigRef:`central`}},context:{...m.args.context,lock:p,lockedUrl:`https://optimize.orders.team-a.example.com`}}},_={name:`Cannot attach`,args:{context:{...m.args.context,secondaryStorage:`rdbms`,backupStorage:!1}}},v=[`BothOn`,`LockedUrl`,`CannotAttach`],h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
  name: "Both on",
  args: {
    value: {
      optimize: {
        managementAuthConfigRef: "central",
        externalUrl: "https://optimize.orders.example.com"
      },
      backupSchedule: {
        schedule: "0 2 * * *",
        retained: {
          completed: 7,
          failed: 2
        }
      }
    }
  }
}`,...h.parameters?.docs?.source}}},g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "Locked URL",
  args: {
    value: {
      optimize: {
        managementAuthConfigRef: "central"
      }
    },
    context: {
      ...meta.args.context,
      lock: urlLock,
      lockedUrl: "https://optimize.orders.team-a.example.com"
    }
  }
}`,...g.parameters?.docs?.source}}},_.parameters={..._.parameters,docs:{..._.parameters?.docs,source:{originalSource:`{
  name: "Cannot attach",
  args: {
    context: {
      ...meta.args.context,
      secondaryStorage: "rdbms",
      backupStorage: false
    }
  }
}`,..._.parameters?.docs?.source}}}})))()}y();export{h as BothOn,_ as CannotAttach,g as LockedUrl,v as __namedExportsOrder,m as default};