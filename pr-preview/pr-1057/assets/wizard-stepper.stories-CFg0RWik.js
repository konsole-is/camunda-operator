import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./wizard-stepper-CuBTIwOo.js";var i,a,o,s,c,l,u,d;function f(){return(f=e((()=>{i=t(),n(),{fn:a}=__STORYBOOK_MODULE_TEST__,o=e=>[{id:`namespace`,label:`Namespace`},{id:`storage`,label:`Storage and access`,errors:e},{id:`settings`,label:`Settings`},{id:`review`,label:`Review`}],s={title:`fleet/WizardStepper`,component:r,args:{steps:o(),current:`storage`,children:(0,i.jsx)(`p`,{className:`text-sm text-text-2`,children:`The section cards of the step go here.`}),onNext:a()}},c={name:`Step without errors`},l={name:`Step with errors`,args:{steps:o([`Choose a secondary storage option`])}},u={name:`Last step`,args:{current:`review`,children:(0,i.jsx)(`p`,{className:`text-sm text-text-2`,children:`The review goes here.`})}},d=[`StepWithoutErrors`,`StepWithErrors`,`LastStep`],c.parameters={...c.parameters,docs:{...c.parameters?.docs,source:{originalSource:`{
  name: "Step without errors"
}`,...c.parameters?.docs?.source}}},l.parameters={...l.parameters,docs:{...l.parameters?.docs,source:{originalSource:`{
  name: "Step with errors",
  args: {
    steps: steps(["Choose a secondary storage option"])
  }
}`,...l.parameters?.docs?.source}}},u.parameters={...u.parameters,docs:{...u.parameters?.docs,source:{originalSource:`{
  name: "Last step",
  args: {
    current: "review",
    children: <p className="text-sm text-text-2">The review goes here.</p>
  }
}`,...u.parameters?.docs?.source}}}})))()}f();export{u as LastStep,l as StepWithErrors,c as StepWithoutErrors,d as __namedExportsOrder,s as default};