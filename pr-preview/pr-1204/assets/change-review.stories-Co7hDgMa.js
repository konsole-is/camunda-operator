import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./jsx-runtime-CndCZlyt.js";import{n,t as r}from"./diff-BLIgMNCc.js";import{n as i,r as a,t as o}from"./toast-D6prtgUR.js";import{n as s,t as c}from"./change-review-DewEOhOl.js";function l(e){let{toast:t,show:n,dismiss:r}=a();return(0,u.jsxs)(`div`,{className:`flex flex-col gap-4`,children:[(0,u.jsx)(c,{...e,show:(t,r)=>{e.show(t,r),n(t,r)}}),t&&(0,u.jsx)(o,{result:t,onDismiss:r})]})}var u,d,f,p,m,h,g,_,v,y,b,x,S,C,w,T,E;function D(){return(D=e((()=>{u=t(),r(),s(),i(),{fn:d}=__STORYBOOK_MODULE_TEST__,f={"backup-storage":{allowed:[`rustfs`]}},p={apiVersion:`fleet.camunda.io/v1alpha1`,kind:`Environment`,metadata:{name:`staging`},spec:{displayName:`Staging`,deletionPolicy:`Delete`,sections:f}},m={...p,spec:{displayName:`Stage`,deletionPolicy:`Orphan`}},h={...m,spec:{...m.spec,sections:f}},g={objects:[{ref:{apiVersion:`fleet.camunda.io/v1alpha1`,kind:`Environment`,name:`staging`},before:p,after:h,diff:n(p,h)}],errors:[]},_={title:`fleet/ChangeReview`,component:c,args:{change:{intent:`Change the environment staging`,apply:[m],preconditions:{"Environment//staging":`12`}},mode:`direct`,diffLabel:`Changes to the environment`,effects:[`Changes apply to new claims only.`],confirm:{label:`Save`},savedMessage:`The environment staging is saved.`,preview:d(async()=>g),apply:d(async()=>({ok:!0,mode:`direct`})),show:d()},render:e=>(0,u.jsx)(l,{...e},JSON.stringify([e.change,e.mode]))},v={apiVersion:`fleet.camunda.io/v1alpha1`,kind:`Environment`,name:`staging`},y={name:`Kept field`,args:{preview:d(async()=>({...g,ownership:[{ref:v,changed:[],cleared:[{path:`spec.sections`,steps:[{field:`spec`},{field:`sections`}],managers:[{name:`kubectl-client-side-apply`,operation:`Update`,kind:`other`,known:!0}]}]}]}))}},b={name:`Git`,args:{mode:`git`}},x=[{ref:v,changed:[{path:`spec.displayName`,steps:[{field:`spec`},{field:`displayName`}],managers:[{name:`kubectl-client-side-apply`,operation:`Update`,kind:`other`,known:!0}]}],cleared:[]}],S={...p,spec:{...p.spec,displayName:`Stage`}},C={ref:v,before:p,after:S,diff:n(p,S)},w={name:`A field that kubectl set`,args:{change:{intent:`Change the environment staging`,apply:[S],preconditions:{"Environment//staging":`12`}},preview:d(async e=>e.takeOver?{objects:[C],errors:[],ownership:x}:{objects:[],errors:[{code:`Conflict`,message:`conflict with "kubectl-client-side-apply"`,path:`spec.displayName`,manager:`kubectl-client-side-apply`,ref:v}],ownership:x,blocked:[C]})}},T={name:`A field that kubectl set, in Git mode`,args:{mode:`git`,change:{intent:`Change the environment staging`,apply:[m],preconditions:{"Environment//staging":`12`}}}},E=[`KeptField`,`Git`,`KubectlField`,`KubectlFieldGit`],y.parameters={...y.parameters,docs:{...y.parameters?.docs,source:{originalSource:`{
  name: "Kept field",
  args: {
    preview: fn(async (): Promise<Preview> => ({
      ...keptPreview,
      ownership: [{
        ref,
        changed: [],
        cleared: [{
          path: "spec.sections",
          steps: [{
            field: "spec"
          }, {
            field: "sections"
          }],
          managers: [{
            name: "kubectl-client-side-apply",
            operation: "Update",
            kind: "other",
            known: true
          }]
        }]
      }]
    }))
  }
}`,...y.parameters?.docs?.source}}},b.parameters={...b.parameters,docs:{...b.parameters?.docs,source:{originalSource:`{
  name: "Git",
  args: {
    mode: "git"
  }
}`,...b.parameters?.docs?.source}}},w.parameters={...w.parameters,docs:{...w.parameters?.docs,source:{originalSource:`{
  name: "A field that kubectl set",
  args: {
    change: {
      intent: "Change the environment staging",
      apply: [takenOver],
      preconditions: {
        "Environment//staging": "12"
      }
    },
    preview: fn(async (change: Change): Promise<Preview> => change.takeOver ? {
      objects: [takenOverObject],
      errors: [],
      ownership
    } : {
      objects: [],
      errors: [{
        code: "Conflict" as const,
        message: 'conflict with "kubectl-client-side-apply"',
        path: "spec.displayName",
        manager: "kubectl-client-side-apply",
        ref
      }],
      ownership,
      blocked: [takenOverObject]
    })
  }
}`,...w.parameters?.docs?.source}}},T.parameters={...T.parameters,docs:{...T.parameters?.docs,source:{originalSource:`{
  name: "A field that kubectl set, in Git mode",
  args: {
    mode: "git",
    change: {
      intent: "Change the environment staging",
      apply: [after],
      preconditions: {
        "Environment//staging": "12"
      }
    }
  }
}`,...T.parameters?.docs?.source}}}})))()}D();export{b as Git,y as KeptField,w as KubectlField,T as KubectlFieldGit,E as __namedExportsOrder,_ as default};