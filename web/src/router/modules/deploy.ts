const Layout = () => import("@/layout/index.vue");

export default {
  path: "/deploy",
  name: "Deploy",
  component: Layout,
  redirect: "/deploy/projects",
  meta: {
    icon: "ep/box",
    title: "部署管理",
    rank: 1
  },
  children: [
    {
      path: "/deploy/projects",
      name: "DeployProjects",
      component: () => import("@/views/deploy/projects/index.vue"),
      meta: {
        title: "项目管理",
        icon: "ep/folder",
        perms: ["project:view"]
      }
    },
    {
      path: "/deploy/teams",
      name: "DeployTeams",
      component: () => import("@/views/deploy/teams/index.vue"),
      meta: {
        title: "项目组",
        icon: "ep/collection",
        roles: ["admin"]
      }
    },
    {
      path: "/deploy/nodes",
      name: "DeployNodes",
      component: () => import("@/views/deploy/nodes/index.vue"),
      meta: {
        title: "节点管理",
        icon: "ep/connection",
        roles: ["admin"]
      }
    },
    {
      path: "/deploy/history",
      name: "DeployHistory",
      component: () => import("@/views/deploy/history/index.vue"),
      meta: {
        title: "发布历史",
        icon: "ep/clock",
        perms: ["project:view"]
      }
    }
  ]
} satisfies RouteConfigsTable;
