import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{_ as r,a as i,g as a,t as o}from"./draft-BzfpT11Y.js";import{t as s}from"./session-BrKF1EYu.js";import{n as c,t as l}from"./attachment-fields-DECWTId0.js";import{n as u,t as d}from"./wizard-stepper-C_hv7euc.js";function f({value:e,summary:t,managementAuthConfigs:n,lock:r,set:i,reset:a,invalid:o,unread:s}){return(0,p.jsx)(l,{value:e,set:i,reset:a,invalid:o,context:{secondaryStorage:t.secondaryStorageType,backupStorage:t.backupStorage!==void 0,managementAuthConfigs:n,lock:r,errors:[],pending:0,lockedUrl:t.optimizeUrl?`Known once the access option is ready`:`None: the access option gives no Optimize URL`,unread:s}})}var p;function m(){return(m=e((()=>{p=n(),c(),f.__docgenInfo={description:`ClusterAttachmentsStep is step 4 of the cluster create flow: Optimize and
the backup schedule of the draft, with the rules of AttachmentFields over
the step 2 summary. lock gives the lock of a claim path of the draft. A
locked Optimize URL says whether the access option fills it later.
unread are the kinds that could not be read.`,methods:[],displayName:`ClusterAttachmentsStep`,props:{value:{required:!0,tsType:{name:`union`,raw:`ClusterAttachments | undefined`,elements:[{name:`ClusterAttachments`},{name:`undefined`}]},description:``},summary:{required:!0,tsType:{name:`StorageSummary`},description:``},managementAuthConfigs:{required:!0,tsType:{name:`unknown`},description:``},lock:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[]) => FieldLock | undefined`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`}],return:{name:`union`,raw:`FieldLock | undefined`,elements:[{name:`FieldLock`},{name:`undefined`}]}}},description:``},set:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[], value: unknown) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`},{type:{name:`unknown`},name:`value`}],return:{name:`void`}}},description:``},reset:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[]) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`}],return:{name:`void`}}},description:``},invalid:{required:!0,tsType:{name:`signature`,type:`function`,raw:`(steps: Step[], message: string | undefined) => void`,signature:{arguments:[{type:{name:`Array`,elements:[{name:`Step`}],raw:`Step[]`},name:`steps`},{type:{name:`union`,raw:`string | undefined`,elements:[{name:`string`},{name:`undefined`}]},name:`message`}],return:{name:`void`}}},description:``},unread:{required:!1,tsType:{name:`unknown`},description:``}}}})))()}function h({initial:e,access:t,...n}){let[i,s]=(0,_.useState)({spec:{attachments:e}});return(0,g.jsx)(d,{steps:[...o],current:`attachments`,children:(0,g.jsx)(f,{...n,value:i.spec?.attachments,lock:e=>{let n=e.at(-1);return t&&n&&`field`in n&&n.field===`externalUrl`?y:void 0},set:(e,t)=>{n.set(e,t),s(n=>a(n,e,t))},reset:e=>{n.reset(e),s(t=>r(t,e))}})})}var g,_,v,y,b,x,S,C,w,T,E,D;function O(){return(O=e((()=>{g=n(),_=t(),u(),i(),s(),m(),{fn:v}=__STORYBOOK_MODULE_TEST__,y={reason:`filled`,section:`access`,sectionLabel:`Access`},b=(e={})=>({secondaryStorageType:`elasticsearch`,backupStorage:{section:`backup-storage`,registration:`bucket`},optimizeUrl:!1,slots:[],picks:[],...e}),x={title:`create/ClusterAttachmentsStep`,component:h,args:{initial:void 0,access:!1,summary:b(),managementAuthConfigs:[`central`,`keycloak`],set:v(),reset:v(),invalid:v()},render:e=>(0,g.jsx)(h,{...e},JSON.stringify(e))},S={name:`Optimize with access`,args:{initial:{optimize:{managementAuthConfigRef:`central`}},access:!0,summary:b({optimizeUrl:!0})}},C={name:`Optimize without access`,args:{initial:{optimize:{managementAuthConfigRef:`central`,externalUrl:`https://optimize.orders.example.com`},backupSchedule:{schedule:`0 2 * * *`,retained:{completed:7,failed:2}}}}},w={name:`PostgreSQL secondary storage`,args:{summary:b({secondaryStorageType:`rdbms`})}},T={name:`No ManagementAuthConfig`,args:{managementAuthConfigs:[]}},E={name:`Schedule without backup storage`,args:{summary:b({backupStorage:void 0})}},D=[`OptimizeWithAccess`,`OptimizeWithoutAccess`,`PostgreSQLSecondaryStorage`,`NoManagementAuthConfig`,`ScheduleWithoutBackupStorage`],S.parameters={...S.parameters,docs:{...S.parameters?.docs,source:{originalSource:`{
  name: "Optimize with access",
  args: {
    initial: {
      optimize: {
        managementAuthConfigRef: "central"
      }
    },
    access: true,
    summary: summary({
      optimizeUrl: true
    })
  }
}`,...S.parameters?.docs?.source}}},C.parameters={...C.parameters,docs:{...C.parameters?.docs,source:{originalSource:`{
  name: "Optimize without access",
  args: {
    initial: {
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
}`,...C.parameters?.docs?.source}}},w.parameters={...w.parameters,docs:{...w.parameters?.docs,source:{originalSource:`{
  name: "PostgreSQL secondary storage",
  args: {
    summary: summary({
      secondaryStorageType: "rdbms"
    })
  }
}`,...w.parameters?.docs?.source}}},T.parameters={...T.parameters,docs:{...T.parameters?.docs,source:{originalSource:`{
  name: "No ManagementAuthConfig",
  args: {
    managementAuthConfigs: []
  }
}`,...T.parameters?.docs?.source}}},E.parameters={...E.parameters,docs:{...E.parameters?.docs,source:{originalSource:`{
  name: "Schedule without backup storage",
  args: {
    summary: summary({
      backupStorage: undefined
    })
  }
}`,...E.parameters?.docs?.source}}}})))()}O();export{T as NoManagementAuthConfig,S as OptimizeWithAccess,C as OptimizeWithoutAccess,w as PostgreSQLSecondaryStorage,E as ScheduleWithoutBackupStorage,D as __namedExportsOrder,x as default};