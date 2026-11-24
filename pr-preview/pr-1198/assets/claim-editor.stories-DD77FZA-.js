import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{a as n,x as r}from"./labels-Be1Q9KU6.js";import{i,t as a}from"./claim-editor-DBqge1mO.js";import{E as o,t as s}from"./merge-BMTaLSfQ.js";import{n as c,t as l}from"./cluster-edit-Dhkion7C.js";function u({version:e=`8.9.3`,suspended:t=!1,claimSpec:r={},claimStatus:i={},over:a={}}){let s={namespace:`team-payments`,name:`orders`},c={presetRef:`medium`,releaseRef:`camunda-8-9-3`,zeebe:{replicas:3},...t&&{suspend:!0}},l={apiVersion:`core.camunda.io/v1`,kind:`CamundaCluster`,metadata:{...s,labels:{[n]:`orders`}},spec:c,status:e?{management:{version:e}}:{}},u={apiVersion:`fleet.camunda.io/v1alpha1`,kind:`CamundaClusterClaim`,metadata:{...s,resourceVersion:`41`},spec:{presetRef:`medium`,releaseRef:`camunda-8-9-3`,cluster:{zeebe:{replicas:3}},...t&&{suspend:!0},...r},status:{environment:`prod`,...i}},d={kind:`Environment`,metadata:{name:`prod`},spec:{displayName:`Production`,presets:{camundaCluster:[`medium`,`large`]},releases:{allowed:[`camunda-8-9-1`,`camunda-8-9-3`,`camunda-8-11-0`],default:`camunda-8-9-3`}}},f=[_(`8.9.1`),_(`8.9.3`),_(`8.11.0`)],p=[v(`medium`),v(`large`)];return{cluster:l,claim:u,merged:o(p[0].spec,f[1].spec,c),environment:d,environments:[d],presets:p,releases:f,platformConfigs:[],managementAuthConfigs:[],sections:[],registrations:[],requests:[],backups:[],restores:[],backupStorage:{kind:`none`,schedules:[]},schedules:[],fields:[],writeMode:`direct`,now:`2026-11-10T12:00:00Z`,unread:[],...a}}var d,f,p,m,h,g,_,v,y,b,x,S,C,w,T,E,D,O,k,A;function j(){return(j=e((()=>{d=t(),r(),s(),i(),c(),{expect:f,fn:p,userEvent:m,within:h}=__STORYBOOK_MODULE_TEST__,g={preview:p(async()=>({objects:[],errors:[]})),apply:p(async()=>({ok:!0,mode:`direct`}))},_=e=>({kind:`CamundaRelease`,metadata:{name:`camunda-${e.replaceAll(`.`,`-`)}`},spec:{version:e}}),v=e=>({kind:`CamundaClusterPreset`,metadata:{name:e},spec:{cluster:{}}}),y={claimSpec:{plugins:[{section:`access`,registration:`gateway-access`,parameters:{hostPrefix:`orders`,sources:[`office`]}}]},claimStatus:{resolvedPolicy:{sections:{access:{allowed:[`gateway-access`]}},parameters:{"gateway-access":{hostPrefix:{claimEditable:!0,value:`camunda`},zone:{value:`payments.example.com`},sources:{claimEditable:!0,allowed:[{name:`office`,label:`Office`,value:`10.0.0.0/16`},{name:`vpn`,label:`VPN`,value:`10.8.0.0/16`}]}}}}},over:{sections:[{kind:`PluginSection`,metadata:{name:`access`},spec:{label:`Access`,selection:`One`,appliesTo:[`CamundaClusterClaim`]}}],registrations:[{kind:`PluginRegistration`,metadata:{name:`gateway-access`},spec:{label:`Gateway`,deletionPolicies:[`Delete`],parameters:{type:`object`,properties:{hostPrefix:{type:`string`,description:`The first label of each host name.`},zone:{type:`string`},sources:{type:`array`,items:{type:`string`}}}}}}]}},b={claimSpec:{attachments:{optimize:{managementAuthConfigRef:`keycloak`,externalUrl:`https://optimize.payments.example.com`},backupSchedule:{schedule:`0 2 * * *`,retained:{completed:7}}}},over:{storage:{kind:`SecondaryStorageConfig`,metadata:{namespace:`team-payments`,name:`orders-storage`},spec:{type:`elasticsearch`}},managementAuthConfigs:[`camunda`,`keycloak`].map(e=>({kind:`ManagementAuthConfig`,metadata:{name:e},spec:{}})),backupStorage:{kind:`elasticsearch`,bucket:{name:`backups`,location:`s3://acme-backups/orders`},repository:`camunda-backups`,schedules:[]}}},x=class e{static{this.CLOSED=2}addEventListener(){}close(){this.readyState=e.CLOSED}constructor(){this.readyState=0}},S={title:`cluster/edit/Claim editor`,component:a,args:{context:u({})},beforeEach:()=>{let e=globalThis.EventSource;return globalThis.EventSource=x,()=>{globalThis.EventSource=e}},decorators:[(e,{args:t})=>(0,d.jsx)(`div`,{className:`max-w-5xl`,children:(0,d.jsx)(l,{context:t.context,actions:g,children:(0,d.jsx)(e,{})},JSON.stringify(t.context))})]},C={name:`Read view with Edit buttons`},w={name:`Basics in edit`,args:{requested:`basics`},play:async({canvasElement:e})=>{let t=h(e).getByRole(`checkbox`,{name:`Suspend the cluster`});await m.click(t),await f(t).toBeChecked()}},T={name:`Release with a downgrade`,args:{requested:`release`},play:async({canvasElement:e})=>{let t=h(e);await m.selectOptions(t.getByRole(`combobox`,{name:`Release`}),`camunda-8-9-1`),await f(t.getByText(/is a downgrade\./)).toBeVisible()}},E={name:`Release while suspended`,args:{context:u({version:``,suspended:!0}),requested:`release`}},D={name:`A section with an editable parameter`,args:{context:u(y),requested:`section:access`}},O={name:`Attachments in edit`,args:{context:u(b),requested:`attachments`}},k={name:`Deletion with Keep refused`,args:{context:u(y),requested:`deletion`}},A=[`ReadViewWithEditButtons`,`BasicsInEdit`,`ReleaseWithADowngrade`,`ReleaseWhileSuspended`,`ASectionWithAnEditableParameter`,`AttachmentsInEdit`,`DeletionWithKeepRefused`],C.parameters={...C.parameters,docs:{...C.parameters?.docs,source:{originalSource:`{
  name: "Read view with Edit buttons"
}`,...C.parameters?.docs?.source}}},w.parameters={...w.parameters,docs:{...w.parameters?.docs,source:{originalSource:`{
  name: "Basics in edit",
  args: {
    requested: "basics"
  },
  play: async ({
    canvasElement
  }) => {
    const suspend = within(canvasElement).getByRole("checkbox", {
      name: "Suspend the cluster"
    });
    await userEvent.click(suspend);
    await expect(suspend).toBeChecked();
  }
}`,...w.parameters?.docs?.source}}},T.parameters={...T.parameters,docs:{...T.parameters?.docs,source:{originalSource:`{
  name: "Release with a downgrade",
  args: {
    requested: "release"
  },
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    await userEvent.selectOptions(canvas.getByRole("combobox", {
      name: "Release"
    }), "camunda-8-9-1");
    await expect(canvas.getByText(/is a downgrade\\./)).toBeVisible();
  }
}`,...T.parameters?.docs?.source}}},E.parameters={...E.parameters,docs:{...E.parameters?.docs,source:{originalSource:`{
  name: "Release while suspended",
  args: {
    context: contextOf({
      version: "",
      suspended: true
    }),
    requested: "release"
  }
}`,...E.parameters?.docs?.source}}},D.parameters={...D.parameters,docs:{...D.parameters?.docs,source:{originalSource:`{
  name: "A section with an editable parameter",
  args: {
    context: contextOf(ACCESS),
    requested: "section:access"
  }
}`,...D.parameters?.docs?.source}}},O.parameters={...O.parameters,docs:{...O.parameters?.docs,source:{originalSource:`{
  name: "Attachments in edit",
  args: {
    context: contextOf(ATTACHED),
    requested: "attachments"
  }
}`,...O.parameters?.docs?.source}}},k.parameters={...k.parameters,docs:{...k.parameters?.docs,source:{originalSource:`{
  name: "Deletion with Keep refused",
  args: {
    context: contextOf(ACCESS),
    requested: "deletion"
  }
}`,...k.parameters?.docs?.source}}}})))()}j();export{D as ASectionWithAnEditableParameter,O as AttachmentsInEdit,w as BasicsInEdit,k as DeletionWithKeepRefused,C as ReadViewWithEditButtons,E as ReleaseWhileSuspended,T as ReleaseWithADowngrade,A as __namedExportsOrder,S as default};