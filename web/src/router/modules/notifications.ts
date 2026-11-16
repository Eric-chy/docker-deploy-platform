const Layout = () => import("@/layout/index.vue");

export default {
  path: "/notifications",
  name: "Notifications",
  component: Layout,
  redirect: "/notifications/channels",
  meta: {
    icon: "ep/clock",
    title: "通知管理",
    rank: 3
  },
  children: [
    {
      path: "/notifications/channels",
      name: "NotificationChannels",
      component: () => import("@/views/notifications/index.vue"),
      meta: {
        title: "通知管理",
        icon: "ep/clock",
        roles: ["admin"]
      }
    }
  ]
} satisfies RouteConfigsTable;
