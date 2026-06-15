import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { useSetup } from "@/composables/useSetup";

declare module "vue-router" {
  interface RouteMeta {
    public?: boolean;
    setup?: boolean;
    admin?: boolean;
    // Tab/bookmark title for this route, rendered as `${title} · DoTheSplit`
    // by the afterEach hook. Dynamic routes (group/expense detail) use a
    // generic label since the entity name isn't known at navigation time.
    title?: string;
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: "/setup",
    name: "setup",
    component: () => import("@/views/SetupView.vue"),
    meta: { public: true, setup: true, title: "First-run setup" },
  },
  {
    path: "/login",
    name: "login",
    component: () => import("@/views/LoginView.vue"),
    meta: { public: true, title: "Log in" },
  },
  {
    path: "/register",
    name: "register",
    component: () => import("@/views/RegisterView.vue"),
    meta: { public: true, title: "Create account" },
  },
  {
    path: "/verify",
    name: "verify",
    component: () => import("@/views/VerifyView.vue"),
    meta: { public: true, title: "Verify email" },
  },
  {
    path: "/forgot",
    name: "forgot",
    component: () => import("@/views/ForgotView.vue"),
    meta: { public: true, title: "Forgot password" },
  },
  {
    path: "/reset",
    name: "reset",
    component: () => import("@/views/ResetView.vue"),
    meta: { public: true, title: "Reset your password" },
  },
  {
    path: "/about",
    name: "about",
    component: () => import("@/views/AboutView.vue"),
    meta: { public: true, title: "About" },
  },
  {
    path: "/groups",
    name: "groups",
    component: () => import("@/views/GroupsView.vue"),
    meta: { title: "Groups" },
  },
  {
    path: "/groups/new",
    name: "group-new",
    component: () => import("@/views/NewGroupView.vue"),
    meta: { title: "New group" },
  },
  {
    path: "/groups/:id",
    name: "group-dashboard",
    component: () => import("@/views/GroupDashboardView.vue"),
    meta: { title: "Group" },
  },
  {
    path: "/groups/:id/settle",
    name: "group-settle",
    component: () => import("@/views/SettleView.vue"),
    meta: { title: "Settle up" },
  },
  {
    path: "/groups/:id/activity",
    name: "group-activity",
    component: () => import("@/views/ActivityView.vue"),
    meta: { title: "Activity" },
  },
  {
    path: "/groups/:id/recurring",
    name: "group-recurring",
    component: () => import("@/views/RecurringView.vue"),
    meta: { title: "Recurring expenses" },
  },
  {
    path: "/groups/:id/settings",
    name: "group-settings",
    component: () => import("@/views/GroupSettingsView.vue"),
    meta: { title: "Group settings" },
  },
  {
    path: "/groups/:id/expenses/:eid",
    name: "expense-detail",
    component: () => import("@/views/ExpenseDetailView.vue"),
    meta: { title: "Expense" },
  },
  {
    path: "/groups/:id/settlements/:sid",
    name: "settlement-detail",
    component: () => import("@/views/SettlementDetailView.vue"),
    meta: { title: "Settlement" },
  },
  {
    path: "/search",
    name: "search",
    component: () => import("@/views/SearchView.vue"),
    meta: { title: "Search" },
  },
  {
    path: "/settings",
    name: "settings",
    component: () => import("@/views/SettingsView.vue"),
    meta: { title: "Settings" },
  },
  {
    path: "/settings/password",
    name: "settings-password",
    component: () => import("@/views/PasswordView.vue"),
    meta: { title: "Change password" },
  },
  {
    path: "/settings/notifications",
    name: "settings-notifications",
    component: () => import("@/views/NotificationsView.vue"),
    meta: { title: "Notifications" },
  },
  {
    path: "/import",
    name: "import",
    component: () => import("@/views/import/ImportIndexView.vue"),
    meta: { title: "Import" },
  },
  {
    path: "/import/splitwise",
    name: "import-splitwise",
    component: () => import("@/views/import/ImportSplitwiseView.vue"),
    meta: { title: "Import from Splitwise" },
  },
  {
    path: "/import/dothesplit",
    name: "import-dothesplit",
    component: () => import("@/views/import/ImportDothesplitView.vue"),
    meta: { title: "Import DoTheSplit" },
  },
  {
    path: "/groups/:id/import-expenses",
    name: "group-import-expenses",
    component: () => import("@/views/import/ImportGroupExpensesView.vue"),
    meta: { title: "Import group" },
  },
  {
    path: "/groups/:id/export",
    name: "group-export",
    component: () => import("@/views/ExportView.vue"),
    meta: { title: "Export" },
  },
  {
    path: "/admin",
    name: "admin",
    component: () => import("@/views/admin/AdminIndexView.vue"),
    meta: { admin: true, title: "Admin" },
  },
  {
    path: "/admin/users",
    name: "admin-users",
    component: () => import("@/views/admin/AdminUsersView.vue"),
    meta: { admin: true, title: "Admin · Users" },
  },
  {
    path: "/admin/users/:id",
    name: "admin-user-detail",
    component: () => import("@/views/admin/AdminUserDetailView.vue"),
    meta: { admin: true, title: "Admin · User" },
  },
  {
    path: "/admin/groups",
    name: "admin-groups",
    component: () => import("@/views/admin/AdminGroupsView.vue"),
    meta: { admin: true, title: "Admin · Groups" },
  },
  {
    path: "/admin/smtp",
    name: "admin-smtp",
    component: () => import("@/views/admin/AdminSmtpView.vue"),
    meta: { admin: true, title: "Admin · SMTP" },
  },
  {
    path: "/admin/audit",
    name: "admin-audit",
    component: () => import("@/views/admin/AdminAuditView.vue"),
    meta: { admin: true, title: "Admin · Audit" },
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

// Guard order:
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

// Set `<title>{title} · DoTheSplit</title>` on each navigation (CSR has no
// per-page server render to do it).
router.afterEach((to) => {
  document.title = to.meta.title
    ? `${to.meta.title} · DoTheSplit`
    : "DoTheSplit";
});
