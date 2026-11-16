import { i18n, type AppLocale } from "@/plugins/i18n";

const permissionDisplayNames: Record<AppLocale, Record<string, string>> = {
  "zh-CN": {
    "project:view": "项目管理",
    "project:create": "创建项目",
    "project:update": "编辑项目",
    "project:delete": "删除项目",
    "deploy:publish": "发布项目",
    "deploy:rollback": "回滚项目",
    "deploy:retry": "重新发布项目",
    "team:view": "项目组管理",
    "node:view": "节点管理",
    "account:view": "账号管理",
    "deploy:approve": "审批发布"
  },
  "en-US": {
    "project:view": "Project Management",
    "project:create": "Create Project",
    "project:update": "Edit Project",
    "project:delete": "Delete Project",
    "deploy:publish": "Publish Project",
    "deploy:rollback": "Rollback Project",
    "deploy:retry": "Retry Deployment",
    "team:view": "Team Management",
    "node:view": "Node Management",
    "account:view": "Account Management",
    "deploy:approve": "Approve Deployment"
  }
};

export function getPermissionDisplayName(code: string, name: string) {
  return permissionDisplayNames[i18n.global.locale.value][code] ?? name;
}
