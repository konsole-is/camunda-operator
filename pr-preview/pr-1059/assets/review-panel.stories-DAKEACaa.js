import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./review-panel-lkTXyH9m.js";import{r,s as i}from"./fixtures-DU7FxC4G.js";var a,o,s,c,l,u;function d(){return(d=e((()=>{r(),t(),{fn:a}=__STORYBOOK_MODULE_TEST__,o={title:`fleet/ReviewPanel`,component:n,args:{lines:i,mode:`direct`,confirmLabel:`Change the claim`,effects:[`Zeebe gets two more brokers.`],onConfirm:a()}},s={name:`Direct mode`,args:{effects:[`Zeebe gets two more brokers.`,`The partitions move to the new brokers.`]}},c={name:`Git mode`,args:{mode:`git`}},l={name:`With caution`,args:{danger:!0,confirmLabel:`Change anyway`,caution:`A backup of orders runs now. The rollout makes it fail. A backup that starts after the change waits until the rollout is done.`}},u=[`DirectMode`,`GitMode`,`WithCaution`],s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Direct mode",
  args: {
    effects: ["Zeebe gets two more brokers.", "The partitions move to the new brokers."]
  }
}`,...s.parameters?.docs?.source}}},c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Git mode",
  args: {
    mode: "git"
  }
}`,...c.parameters?.docs?.source}}},l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "With caution",
  args: {
    danger: true,
    confirmLabel: "Change anyway",
    caution: "A backup of orders runs now. The rollout makes it fail. A backup that starts after the change waits until the rollout is done."
  }
}`,...l.parameters?.docs?.source}}}})))()}d();export{s as DirectMode,c as GitMode,l as WithCaution,u as __namedExportsOrder,o as default};