import type { RouteMetadata } from "@tanstack/react-router";

export interface AdminRouteStaticData {
  breadcrumb?: {
    label: string;
  };
}

// Augment the @tanstack/react-router types
declare module "@tanstack/react-router" {
  interface RouteMeta {
    breadcrumb?: string;
  }
}
