import type { Story } from "@/gallery/registry";
import { Badge } from "./badge";
import { Button } from "./button";
import { Checkbox } from "./checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "./dialog";
import { Input } from "./input";
import { Popover, PopoverContent, PopoverTrigger } from "./popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "./select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "./table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "./tabs";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "./tooltip";

export const uiStories: Story[] = [
  {
    component: "Button",
    name: "Variants",
    render: () => (
      <div className="flex flex-wrap gap-3">
        <Button>Create cluster</Button>
        <Button variant="secondary">Cancel</Button>
        <Button variant="outline">Edit</Button>
        <Button variant="ghost">More</Button>
        <Button variant="destructive">Delete</Button>
        <Button variant="link">View</Button>
        <Button disabled>Saving</Button>
      </div>
    ),
  },
  {
    component: "Button",
    name: "Sizes",
    render: () => (
      <div className="flex flex-wrap items-center gap-3">
        <Button size="xs">Extra small</Button>
        <Button size="sm">Small</Button>
        <Button>Default</Button>
        <Button size="lg">Large</Button>
      </div>
    ),
  },
  {
    component: "Input",
    name: "Labelled and disabled",
    render: () => (
      <div className="flex max-w-sm flex-col gap-2">
        <label htmlFor="story-name" className="text-sm font-medium text-muted">
          Name
        </label>
        <Input id="story-name" defaultValue="orders" />
        <label htmlFor="story-ns" className="text-sm font-medium text-muted">
          Namespace
        </label>
        <Input id="story-ns" defaultValue="team-orders" disabled />
      </div>
    ),
  },
  {
    component: "Select",
    name: "Closed",
    render: () => (
      <div className="flex max-w-sm flex-col gap-2">
        <span id="story-env" className="text-sm font-medium text-muted">
          Environment
        </span>
        <Select defaultValue="prod">
          <SelectTrigger aria-labelledby="story-env">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="prod">prod</SelectItem>
            <SelectItem value="staging">staging</SelectItem>
          </SelectContent>
        </Select>
      </div>
    ),
  },
  {
    component: "Checkbox",
    name: "Checked and unchecked",
    render: () => (
      <div className="flex flex-col gap-3">
        <label className="flex items-center gap-2">
          <Checkbox defaultChecked /> Keep the resources
        </label>
        <label className="flex items-center gap-2">
          <Checkbox /> Delete the backups
        </label>
      </div>
    ),
  },
  {
    component: "Tabs",
    name: "Overview selected",
    render: () => (
      <div className="flex flex-col gap-6">
        <Tabs defaultValue="overview">
          <TabsList>
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="logs">Logs</TabsTrigger>
          </TabsList>
          <TabsContent value="overview">Overview of the cluster</TabsContent>
          <TabsContent value="logs">Logs of the cluster</TabsContent>
        </Tabs>
        <Tabs defaultValue="overview">
          <TabsList variant="line">
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="logs">Logs</TabsTrigger>
          </TabsList>
          <TabsContent value="overview">Overview of the cluster</TabsContent>
          <TabsContent value="logs">Logs of the cluster</TabsContent>
        </Tabs>
      </div>
    ),
  },
  {
    component: "Tooltip",
    name: "Open",
    render: () => (
      <div className="pt-12">
        <TooltipProvider>
          <Tooltip open>
            <TooltipTrigger asChild>
              <Button variant="outline">Version</Button>
            </TooltipTrigger>
            <TooltipContent>Set by the release</TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </div>
    ),
  },
  {
    component: "Dialog",
    name: "Open",
    render: () => (
      <Dialog open>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete cluster orders</DialogTitle>
            <DialogDescription>
              The claim goes. The backups stay.
            </DialogDescription>
          </DialogHeader>
        </DialogContent>
      </Dialog>
    ),
  },
  {
    component: "Popover",
    name: "Open",
    render: () => (
      <Popover open>
        <PopoverTrigger asChild>
          <Button variant="outline">Filters</Button>
        </PopoverTrigger>
        <PopoverContent>Only clusters that are not ready</PopoverContent>
      </Popover>
    ),
  },
  {
    component: "Badge",
    name: "Variants",
    render: () => (
      <div className="flex flex-wrap gap-2">
        <Badge>Default</Badge>
        <Badge variant="secondary">Optional</Badge>
        <Badge variant="outline">Required</Badge>
        <Badge variant="destructive">Refused</Badge>
        <Badge variant="ghost">Draft</Badge>
        <Badge variant="link">Details</Badge>
      </div>
    ),
  },
  {
    component: "Table",
    name: "Three rows",
    render: () => (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Cluster</TableHead>
            <TableHead>Namespace</TableHead>
            <TableHead>Version</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>orders</TableCell>
            <TableCell>team-orders</TableCell>
            <TableCell>8.9.23</TableCell>
          </TableRow>
          <TableRow>
            <TableCell>payments</TableCell>
            <TableCell>team-payments</TableCell>
            <TableCell>8.9.23</TableCell>
          </TableRow>
          <TableRow>
            <TableCell>shipping</TableCell>
            <TableCell>team-orders</TableCell>
            <TableCell>8.9.22</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    ),
  },
  {
    component: "Table",
    name: "Wide",
    render: () => (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Cluster</TableHead>
            <TableHead>Namespace</TableHead>
            <TableHead>Environment</TableHead>
            <TableHead>Release</TableHead>
            <TableHead>Image</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>orders</TableCell>
            <TableCell>team-orders</TableCell>
            <TableCell>production-eu-north-1</TableCell>
            <TableCell>8.9 long-term support</TableCell>
            <TableCell>registry.example.com/camunda/zeebe:8.9.23</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    ),
  },
];
