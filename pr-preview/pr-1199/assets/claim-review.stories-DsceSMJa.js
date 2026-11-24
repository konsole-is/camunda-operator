import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{c as t,t as n}from"./session-C0vHNhSg.js";import{i as r,s as i,t as a}from"./fixtures-WaccYZgN.js";var o,s,c,l,u,d,f,p,m,h,g,_,v;function y(){return(y=e((()=>{n(),r(),{expect:o,fn:s,waitFor:c,within:l}=__STORYBOOK_MODULE_TEST__,u=[{field:`spec`},{field:`cluster`},{field:`zeebe`},{field:`replicas`}],d={title:`fleet/ClaimReview`,component:a,args:{claim:{...i,metadata:{...i.metadata,resourceVersion:`41`}},changes:[{steps:u,value:5}],mode:`direct`,impacts:[`Broker pods restart.`],preview:s(async()=>({objects:[],errors:[]})),apply:s(async()=>({ok:!0,mode:`direct`})),show:s(),onWritten:s(),onRefused:s(),onCancel:s()}},f={name:`Direct`},p={name:`Git`,args:{mode:`git`}},m={name:`With caution`,args:{caution:`A backup of orders runs now. The rollout makes it fail.`}},h={name:`Refused by the page`,args:{blocking:[`zeebe.resources: The CPU limit of 1 is below the CPU request of 2. A field that you cleared stays in the claim that the save leaves, or gets its default value again. That causes this error. Go back and change the edit, or take the fields over when the review offers that.`]},play:async({canvasElement:e,args:t})=>{let n=l(e);await c(()=>o(t.preview).toHaveBeenCalled()),await o(n.getByRole(`alert`)).toHaveTextContent(`A field that you cleared stays in the claim that the save leaves`),await o(n.getByRole(`button`,{name:`Save`})).toBeDisabled()}},g={name:`Preview refused`,args:{changes:[{steps:u,value:0}],impacts:[],preview:s(async()=>({objects:[],errors:[{code:`Invalid`,message:`Invalid value: 0: must be greater than or equal to 1`,path:`spec.cluster.zeebe.replicas`}]}))}},_={name:`Stale`,args:{preview:s(async()=>({objects:[],errors:[{code:`Stale`,message:t}]}))}},v=[`Direct`,`Git`,`WithCaution`,`RefusedByThePage`,`PreviewRefused`,`Stale`],f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "Direct"
}`,...f.parameters?.docs?.source}}},p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Git",
  args: {
    mode: "git"
  }
}`,...p.parameters?.docs?.source}}},m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "With caution",
  args: {
    caution: "A backup of orders runs now. The rollout makes it fail."
  }
}`,...m.parameters?.docs?.source}}},h.parameters={...h.parameters,docs:{...h.parameters?.docs,source:{originalSource:`{
  name: "Refused by the page",
  args: {
    blocking: ["zeebe.resources: The CPU limit of 1 is below the CPU request of 2. A field that you cleared stays in the claim that the save leaves, or gets its default value again. That causes this error. Go back and change the edit, or take the fields over when the review offers that."]
  },
  play: async ({
    canvasElement,
    args
  }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(args.preview).toHaveBeenCalled());
    await expect(canvas.getByRole("alert")).toHaveTextContent("A field that you cleared stays in the claim that the save leaves");
    await expect(canvas.getByRole("button", {
      name: "Save"
    })).toBeDisabled();
  }
}`,...h.parameters?.docs?.source}}},g.parameters={...g.parameters,docs:{...g.parameters?.docs,source:{originalSource:`{
  name: "Preview refused",
  args: {
    changes: [{
      steps: replicas,
      value: 0
    }],
    impacts: [],
    preview: fn(async (): Promise<Preview> => ({
      objects: [],
      errors: [{
        code: "Invalid",
        message: "Invalid value: 0: must be greater than or equal to 1",
        path: "spec.cluster.zeebe.replicas"
      }]
    }))
  }
}`,...g.parameters?.docs?.source}}},_.parameters={..._.parameters,docs:{..._.parameters?.docs,source:{originalSource:`{
  name: "Stale",
  args: {
    preview: fn(async (): Promise<Preview> => ({
      objects: [],
      errors: [{
        code: "Stale",
        message: STALE_CLAIM_MESSAGE
      }]
    }))
  }
}`,..._.parameters?.docs?.source}}}})))()}y();export{f as Direct,p as Git,g as PreviewRefused,h as RefusedByThePage,_ as Stale,m as WithCaution,v as __namedExportsOrder,d as default};