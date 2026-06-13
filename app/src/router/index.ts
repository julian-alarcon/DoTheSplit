import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";
import { useAuth } from "@/composables/useAuth";

const routes: RouteRecordRaw[] = [
  {
    path: "/login",
    name: "login",
    component: () => import("@/views/LoginView.vue"),
    meta: { public: true },
  },
  {
    path: "/",
    name: "home",
    component: () => import("@/views/HomeView.vue"),
  },
];

export const router = createRouter({
  history: createWebHistory(),
  routes,
});

// Auth guard. Waits for the boot refresh to settle so a hard reload on a
// protected route doesn't bounce to /login before the session is restored.
router.beforeEach(async (to) => {
  const { state, boot } = useAuth();
  if (!state.ready) await boot();

  const isPublic = to.meta.public === true;
  if (!isPublic && !state.user) {
    return { name: "login", query: { redirect: to.fullPath } };
  }
  if (isPublic && state.user && to.name === "login") {
    return { name: "home" };
  }
  return true;
});
