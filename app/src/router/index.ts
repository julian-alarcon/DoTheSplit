import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { useSetup } from "@/composables/useSetup";

const routes: RouteRecordRaw[] = [
  {
    path: "/setup",
    name: "setup",
    component: () => import("@/views/SetupView.vue"),
    meta: { public: true, setup: true },
  },
  {
    path: "/login",
    name: "login",
    component: () => import("@/views/LoginView.vue"),
    meta: { public: true },
  },
  {
    path: "/register",
    name: "register",
    component: () => import("@/views/RegisterView.vue"),
    meta: { public: true },
  },
  {
    path: "/verify",
    name: "verify",
    component: () => import("@/views/VerifyView.vue"),
    meta: { public: true },
  },
  {
    path: "/forgot",
    name: "forgot",
    component: () => import("@/views/ForgotView.vue"),
    meta: { public: true },
  },
  {
    path: "/reset",
    name: "reset",
    component: () => import("@/views/ResetView.vue"),
    meta: { public: true },
  },
  {
    path: "/about",
    name: "about",
    component: () => import("@/views/AboutView.vue"),
    meta: { public: true },
  },
  {
    path: "/groups",
    name: "groups",
    component: () => import("@/views/GroupsView.vue"),
  },
  {
    path: "/groups/new",
    name: "group-new",
    component: () => import("@/views/NewGroupView.vue"),
  },
  {
    path: "/groups/:id",
    name: "group-dashboard",
    component: () => import("@/views/GroupDashboardView.vue"),
  },
  {
    path: "/groups/:id/settle",
    name: "group-settle",
    component: () => import("@/views/SettleView.vue"),
  },
  {
    path: "/groups/:id/activity",
    name: "group-activity",
    component: () => import("@/views/ActivityView.vue"),
  },
  {
    path: "/groups/:id/recurring",
    name: "group-recurring",
    component: () => import("@/views/RecurringView.vue"),
  },
  {
    path: "/groups/:id/settings",
    name: "group-settings",
    component: () => import("@/views/GroupSettingsView.vue"),
  },
  {
    path: "/groups/:id/expenses/:eid",
    name: "expense-detail",
    component: () => import("@/views/ExpenseDetailView.vue"),
  },
  {
    path: "/groups/:id/settlements/:sid",
    name: "settlement-detail",
    component: () => import("@/views/SettlementDetailView.vue"),
  },
  {
    path: "/search",
    name: "search",
    component: () => import("@/views/SearchView.vue"),
  },
  {
    path: "/settings",
    name: "settings",
    component: () => import("@/views/SettingsView.vue"),
  },
  {
    path: "/settings/password",
    name: "settings-password",
    component: () => import("@/views/PasswordView.vue"),
  },
  {
    path: "/settings/notifications",
    name: "settings-notifications",
    component: () => import("@/views/NotificationsView.vue"),
  },
  {
    path: "/import",
    name: "import",
    component: () => import("@/views/import/ImportIndexView.vue"),
  },
  {
    path: "/import/splitwise",
    name: "import-splitwise",
    component: () => import("@/views/import/ImportSplitwiseView.vue"),
  },
  {
    path: "/import/dothesplit",
    name: "import-dothesplit",
    component: () => import("@/views/import/ImportDothesplitView.vue"),
  },
  {
    path: "/groups/:id/import-expenses",
    name: "group-import-expenses",
    component: () => import("@/views/import/ImportGroupExpensesView.vue"),
  },
  {
    path: "/groups/:id/export",
    name: "group-export",
    component: () => import("@/views/ExportView.vue"),
  },
  {
    path: "/admin",
    name: "admin",
    component: () => import("@/views/admin/AdminIndexView.vue"),
    meta: { admin: true },
  },
  {
    path: "/admin/users",
    name: "admin-users",
    component: () => import("@/views/admin/AdminUsersView.vue"),
    meta: { admin: true },
  },
  {
    path: "/admin/users/:id",
    name: "admin-user-detail",
    component: () => import("@/views/admin/AdminUserDetailView.vue"),
    meta: { admin: true },
  },
  {
    path: "/admin/groups",
    name: "admin-groups",
    component: () => import("@/views/admin/AdminGroupsView.vue"),
    meta: { admin: true },
  },
  {
    path: "/admin/smtp",
    name: "admin-smtp",
    component: () => import("@/views/admin/AdminSmtpView.vue"),
    meta: { admin: true },
  },
  {
    path: "/admin/audit",
    name: "admin-audit",
    component: () => import("@/views/admin/AdminAuditView.vue"),
    meta: { admin: true },
  },
  {
    // Authenticated landing: the groups list is the app's home.
    path: "/",
    name: "home",
    redirect: { name: "groups" },
  },
];

export const router = createRouter({
  history: createWebHistory(),
  routes,
});

// Guard order mirrors the Astro middleware:
//   1. Probe first-run setup state (cached after the first check). While the
//      instance is unlocked, funnel everything to /setup; once locked, keep
//      authenticated users away from /setup.
//   2. Wait for the boot refresh so a hard reload on a protected route doesn't
//      bounce to /login before the session is restored from the refresh cookie.
//   3. Gate non-public routes on an authenticated user; redirect logged-in
//      users away from the auth entry pages.
router.beforeEach(async (to) => {
  const { state: setup, ensureChecked } = useSetup();
  await ensureChecked();

  if (!setup.locked) {
    return to.name === "setup" ? true : { name: "setup" };
  }
  // Setup is done; a stale /setup bookmark goes to login/home.
  if (to.name === "setup") {
    return { name: "login" };
  }

  const { state, boot } = useAuth();
  if (!state.ready) await boot();

  const isPublic = to.meta.public === true;
  if (!isPublic && !state.user) {
    return { name: "login", query: { redirect: to.fullPath } };
  }
  if (state.user && (to.name === "login" || to.name === "register")) {
    return { name: "home" };
  }
  // Admin pages require the role flag; non-admins land on /groups (the API
  // also enforces this, so this only avoids a 403 flash).
  if (to.meta.admin === true && !state.user?.is_admin) {
    return { name: "groups" };
  }
  return true;
});
