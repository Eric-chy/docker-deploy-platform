const Layout = () => import("@/layout/index.vue");

export default {
  path: "/accounts",
  name: "Accounts",
  component: Layout,
  redirect: "/accounts/users",
  meta: {
    icon: "ep/user",
    title: "账号管理",
    rank: 2
  },
  children: [
    {
      path: "/accounts/permissions",
      name: "AccountsPermissions",
      component: () => import("@/views/deploy/permissions/index.vue"),
      meta: {
        title: "权限管理",
        icon: "ep/folder",
        roles: ["admin"]
      }
    },
    {
      path: "/accounts/roles",
      name: "AccountsRoles",
      component: () => import("@/views/deploy/roles/index.vue"),
      meta: {
        title: "角色管理",
        icon: "ep/collection",
        roles: ["admin"]
      }
    },
    {
      path: "/accounts/users",
      name: "AccountsUsers",
      component: () => import("@/views/deploy/users/index.vue"),
      meta: {
        title: "账号管理",
        icon: "ep/user",
        roles: ["admin"]
      }
    }
  ]
} satisfies RouteConfigsTable;
