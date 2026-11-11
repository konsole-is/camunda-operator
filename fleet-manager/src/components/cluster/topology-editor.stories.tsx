import type { Story } from "@/gallery/registry";
import { TopologyChangeStory } from "./topology-change-story";

export const topologyEditorStories: Story[] = [
  {
    component: "TopologyChangePanel",
    name: "Gateway becomes embedded",
    render: () => <TopologyChangeStory fixture="embedded" />,
  },
  {
    component: "TopologyChangePanel",
    name: "Operate becomes standalone with prepared settings",
    render: () => <TopologyChangeStory fixture="standalone" />,
  },
  {
    component: "TopologyChangePanel",
    name: "Locked mode",
    render: () => <TopologyChangeStory fixture="locked" />,
  },
];
