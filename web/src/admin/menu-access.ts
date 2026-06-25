import { Activity, FileText, Inbox, ListTodo, Sparkles, Users, type LucideIcon } from "lucide-react";

export interface NavItem {
  labelKey: string;
  to: string;
  icon: LucideIcon;
}

export interface NavGroup {
  titleKey: string;
  items: NavItem[];
}

export const groups: NavGroup[] = [
  {
    titleKey: "sidebar.group.overview",
    items: [{ labelKey: "sidebar.item.welcome", to: "/welcome", icon: Sparkles }],
  },
  {
    titleKey: "sidebar.group.processManagement",
    items: [
      { labelKey: "sidebar.item.processes", to: "/processes", icon: Activity },
      { labelKey: "sidebar.item.entities", to: "/entities", icon: FileText },
    ],
  },
  {
    titleKey: "sidebar.group.taskManagement",
    items: [
      { labelKey: "sidebar.item.todos", to: "/todos", icon: ListTodo },
      { labelKey: "sidebar.item.outbox", to: "/outbox", icon: Inbox },
    ],
  },
  {
    titleKey: "sidebar.group.system",
    items: [
      { labelKey: "sidebar.item.users", to: "/users", icon: Users },
    ],
  },
];
