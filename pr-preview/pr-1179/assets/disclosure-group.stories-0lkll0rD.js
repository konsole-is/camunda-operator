import{n as e}from"./rolldown-runtime-Bjp52BOI.js";import{t}from"./react-DZrhAQ_7.js";import{t as n}from"./jsx-runtime-CndCZlyt.js";import{n as r,t as i}from"./disclosure-group-BdHHiQWS.js";function a({startsOpen:e=!1,...t}){let[n,r]=(0,c.useState)(e);return(0,s.jsx)(i,{...t,open:n,onToggle:r})}function o(e){let t=u(e).getByRole(`button`,{name:/^Brokers/});return{row:t,body:document.getElementById(t.getAttribute(`aria-controls`))}}var s,c,l,u,d,f,p,m,h;function g(){return(g=e((()=>{s=n(),c=t(),r(),{expect:l,within:u}=__STORYBOOK_MODULE_TEST__,d={title:`fleet/DisclosureGroup`,component:a,args:{label:`Brokers`,counts:[{text:`0`}],children:(0,s.jsx)(`p`,{className:`text-sm text-muted`,children:`3 settings: 2 from the preset, 1 default value`})}},f={name:`Closed`,args:{children:(0,s.jsx)(`input`,{"aria-label":`replicas`,defaultValue:`3`})},play:async({canvasElement:e})=>{let{row:t,body:n}=o(e);await l(t).toHaveAttribute(`aria-expanded`,`false`),await l(n.getBoundingClientRect().height).toBe(0);let r=n.querySelector(`input`);await l(r.checkVisibility()).toBe(!1),r.focus(),await l(document.activeElement).not.toBe(r)}},p={name:`Open`,args:{startsOpen:!0},play:async({canvasElement:e})=>{await l(o(e).row).toHaveAttribute(`aria-expanded`,`true`),await l(u(e).getByText(/^3 settings:/)).toBeVisible()}},m={name:`Error count`,args:{heading:!0,counts:[{text:`1`,label:`1 changed in the claim`},{text:`2 errors`,bad:!0}]},play:async({canvasElement:e})=>{let t=u(e);await l(t.getByRole(`heading`,{level:3,name:`Brokers`})).toBeVisible(),await l(t.getByRole(`button`,{name:`Brokers 1 changed in the claim 2 errors`})).toBeVisible(),await l(t.getByText(`2 errors`)).toBeVisible()}},h=[`Closed`,`Open`,`ErrorCount`],f.parameters={...f.parameters,docs:{...f.parameters?.docs,source:{originalSource:`{
  name: "Closed",
  args: {
    children: <input aria-label="replicas" defaultValue="3" />
  },
  play: async ({
    canvasElement
  }) => {
    const {
      row,
      body: closed
    } = rowAndBody(canvasElement);
    await expect(row).toHaveAttribute("aria-expanded", "false");
    await expect(closed.getBoundingClientRect().height).toBe(0);
    const field = closed.querySelector("input")!;
    await expect(field.checkVisibility()).toBe(false);
    field.focus();
    await expect(document.activeElement).not.toBe(field);
  }
}`,...f.parameters?.docs?.source}}},p.parameters={...p.parameters,docs:{...p.parameters?.docs,source:{originalSource:`{
  name: "Open",
  args: {
    startsOpen: true
  },
  play: async ({
    canvasElement
  }) => {
    await expect(rowAndBody(canvasElement).row).toHaveAttribute("aria-expanded", "true");
    await expect(within(canvasElement).getByText(/^3 settings:/)).toBeVisible();
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
      name: "Brokers 1 changed in the claim 2 errors"
    })).toBeVisible();
    await expect(canvas.getByText("2 errors")).toBeVisible();
  }
}`,...m.parameters?.docs?.source}}}})))()}g();export{f as Closed,m as ErrorCount,p as Open,h as __namedExportsOrder,d as default};