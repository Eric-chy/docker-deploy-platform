import type { translate, translateTitle } from "@/plugins/i18n";

declare module "vue" {
  export interface ComponentCustomProperties {
    $tt: typeof translate;
    $routeTitle: typeof translateTitle;
  }
}

export {};
