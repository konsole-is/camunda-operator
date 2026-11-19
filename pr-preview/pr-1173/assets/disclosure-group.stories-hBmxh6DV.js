import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{n as r,t as i}from"./disclosure-group-D_vdDLJs.js";function a({startsOpen:e=!1,...t}){let[n,r]=(0,s.useState)(e);return(0,o.jsx)(i,{...t,open:n,onToggle:r})}var o,s,c,l,u,d,f,p,m,h;function g(){return(g=e((()=>{o=n(),s=t(),r(),{expect:c,userEvent:l,within:u}=__STORYBOOK_MODULE_TEST__,d={title:`fleet/DisclosureGroup`,component:a,args:{label:`Brokers`,counts:[{text:`0`}],children:(0,o.jsx)(`p`,{className:`text-sm text-muted`,children:`3 settings: 2 from the preset, 1 default value`})}},f={name:`Closed`,play:async({canvasElement:e})=>{let t=u(e).getByRole(`button`,{name:`Brokers 0`});await c(t).toHaveAttribute(`aria-expanded`,`false`),await l.click(t),await c(t).toHaveAttribute(`aria-expanded`,`true`),await c(u(e).getByText(/^3 settings:/)).toBeVisible()}},p={name:`Open`,args:{startsOpen:!0}},m={name:`Error count`,args:{heading:!0,counts:[{text:`1`,label:`1 changed in the claim`},{text:`2 errors`,label:`2 settings have an error`,bad:!0}]},play:async({canvasElement:e})=>{let t=u(e);await c(t.getByRole(`heading`,{level:3,name:`Brokers`})).toBeVisible(),await c(t.getByRole(`button`,{name:`Brokers 1 changed in the claim 2 settings have an error`})).toBeVisible(),await c(t.getByText(`2 errors`)).toBeVisible()}},h=[`Closed`,`Open`,`ErrorCount`],f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "Closed",
  play: async ({
    canvasElement
  }) => {
    const row = within(canvasElement).getByRole("button", {
      name: "Brokers 0"
    });
    await expect(row).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(row);
    await expect(row).toHaveAttribute("aria-expanded", "true");
    await expect(within(canvasElement).getByText(/^3 settings:/)).toBeVisible();
  }
}`,...f.parameters?.docs?.source}}},p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Open",
  args: {
    startsOpen: true
  }
}`,...p.parameters?.docs?.source}}},m.parameters={...m.parameters,docs:{...m.parameters?.docs,source:{originalSource:`{
  name: "Error count",
  args: {
    heading: true,
    counts: [{
      text: "1",
      label: "1 changed in the claim"
    }, {
      text: "2 errors",
      label: "2 settings have an error",
      bad: true
    }]
  },
  play: async ({
    canvasElement
  }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole("heading", {
      level: 3,
      name: "Brokers"
    })).toBeVisible();
    await expect(canvas.getByRole("button", {
      name: "Brokers 1 changed in the claim 2 settings have an error"
    })).toBeVisible();
    await expect(canvas.getByText("2 errors")).toBeVisible();
  }
}`,...m.parameters?.docs?.source}}}})))()}g();export{f as Closed,m as ErrorCount,p as Open,h as __namedExportsOrder,d as default};