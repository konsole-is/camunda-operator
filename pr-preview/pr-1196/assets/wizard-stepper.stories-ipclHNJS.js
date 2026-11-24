import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./wizard-stepper-z_QpS6pl.js";var i,a,o,s,c,l,u,d,f,p;function m(){return(m=e((()=>{i=t(),n(),{fn:a}=__STORYBOOK_MODULE_TEST__,o=(e,t)=>[{id:`namespace`,label:`Namespace`,errors:t},{id:`storage`,label:`Storage and access`,errors:e},{id:`settings`,label:`Settings`},{id:`review`,label:`Review`}],s={title:`fleet/WizardStepper`,component:r,args:{steps:o(),current:`storage`,children:(0,i.jsx)(`p`,{className:`text-sm text-text-2`,children:`The section cards of the step go here.`}),onNext:a(),onBack:a(),onStep:a()}},c={name:`Step without errors`},l={name:`Step with errors`,args:{steps:o([`Choose a secondary storage option`])}},u={name:`First step`,args:{current:`namespace`}},d={name:`Done step with errors`,args:{current:`settings`,steps:o(void 0,[`The namespace team-a is gone`])}},f={name:`Last step`,args:{current:`review`,children:(0,i.jsx)(`p`,{className:`text-sm text-text-2`,children:`The review goes here.`})}},p=[`StepWithoutErrors`,`StepWithErrors`,`FirstStep`,`DoneStepWithErrors`,`LastStep`],c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Step without errors"
}`,...c.parameters?.docs?.source}}},l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Step with errors",
  args: {
    steps: steps(["Choose a secondary storage option"])
  }
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "First step",
  args: {
    current: "namespace"
  }
}`,...u.parameters?.docs?.source}}},d.parameters={...d.parameters,docs:{...d.parameters?.docs,source:{originalSource:`{
  name: "Done step with errors",
  args: {
    current: "settings",
    steps: steps(undefined, ["The namespace team-a is gone"])
  }
}`,...d.parameters?.docs?.source}}},f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "Last step",
  args: {
    current: "review",
    children: <p className="text-sm text-text-2">The review goes here.</p>
  }
}`,...f.parameters?.docs?.source}}}})))()}m();export{d as DoneStepWithErrors,u as FirstStep,f as LastStep,l as StepWithErrors,c as StepWithoutErrors,p as __namedExportsOrder,s as default};