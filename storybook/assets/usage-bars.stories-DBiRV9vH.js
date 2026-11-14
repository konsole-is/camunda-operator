import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{n as t,t as n}from"./usage-bars-ZqI0wd0x.js";var r,i,a,o,s,c;function l(){return(l=e((()=>{t(),r=2**30,i={title:`fleet/UsageBars`,component:n},a={name:`Near the limit`,args:{usage:{pod:`x`,cpu:{usedCores:.95,limitCores:1},memory:{usedBytes:3.8*r,limitBytes:4*r}}}},o={name:`Request and limit`,args:{usage:{pod:`x`,cpu:{usedCores:.8,requestCores:.5,limitCores:1},memory:{usedBytes:1.2*r,requestBytes:1*r,limitBytes:2*r}}}},s={name:`No limit`,args:{usage:{pod:`x`,cpu:{usedCores:.2},memory:{usedBytes:.5*r}}}},c=[`NearTheLimit`,`RequestAndLimit`,`NoLimit`],a.parameters={...a.parameters,docs:{...a.parameters?.docs,source:{originalSource:`{
  name: "Near the limit",
  args: {
    usage: {
      pod: "x",
      cpu: {
        usedCores: 0.95,
        limitCores: 1
      },
      memory: {
        usedBytes: 3.8 * GiB,
        limitBytes: 4 * GiB
      }
    }
  }
}`,...a.parameters?.docs?.source}}},o.parameters={...o.parameters,docs:{...o.parameters?.docs,source:{originalSource:`{
  name: "Request and limit",
  args: {
    usage: {
      pod: "x",
      cpu: {
        usedCores: 0.8,
        requestCores: 0.5,
        limitCores: 1
      },
      memory: {
        usedBytes: 1.2 * GiB,
        requestBytes: 1 * GiB,
        limitBytes: 2 * GiB
      }
    }
  }
}`,...o.parameters?.docs?.source}}},s.parameters={...s.parameters,docs:{...s.parameters?.docs,source:{originalSource:`{
  name: "No limit",
  args: {
    usage: {
      pod: "x",
      cpu: {
        usedCores: 0.2
      },
      memory: {
        usedBytes: 0.5 * GiB
      }
    }
  }
}`,...s.parameters?.docs?.source}}}})))()}l();export{a as NearTheLimit,s as NoLimit,o as RequestAndLimit,c as __namedExportsOrder,i as default};