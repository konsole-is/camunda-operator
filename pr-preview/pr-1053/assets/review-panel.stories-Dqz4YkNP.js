import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./review-panel-sy8m1Rmw.js";import{n as r,t as i}from"./diff-BLIgMNCc.js";var a,o,s,c;function l(){return(l=e((()=>{t(),i(),{fn:a}=__STORYBOOK_MODULE_TEST__,o={title:`cluster/ReviewPanel`,component:n,args:{onConfirm:a()}},s={name:`Typed confirm`,args:{lines:r({spec:{cluster:{zeebe:{replicas:3}}}},{spec:{cluster:{zeebe:{replicas:5}}}}),mode:`direct`,confirmLabel:`Change anyway`,danger:!0,typed:`orders`,effects:[`Broker pods restart.`]}},c=[`TypedConfirm`],s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "Typed confirm",
  args: {
    lines: objectDiff({
      spec: {
        cluster: {
          zeebe: {
            replicas: 3
          }
        }
      }
    }, {
      spec: {
        cluster: {
          zeebe: {
            replicas: 5
          }
        }
      }
    }),
    mode: "direct",
    confirmLabel: "Change anyway",
    danger: true,
    typed: "orders",
    effects: ["Broker pods restart."]
  }
}`,...s.parameters?.docs?.source}}}})))()}l();export{s as TypedConfirm,c as __namedExportsOrder,o as default};