<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { deleteUser, getUser, resetUserPassword, setUserRole, type AdminUser } from "@/composables/useAdmin";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";
import Icon from "@/components/Icon.vue";
import MemberAvatar from "@/components/MemberAvatar.vue";
import PasswordPromptDialog from "@/components/PasswordPromptDialog.vue";

const route = useRoute();
const router = useRouter();
const { state } = useAuth();
const userId = computed(() => String(route.params.id));

const target = ref<AdminUser | null>(null);
const loaded = ref(false);
const okMsg = ref<string | null>(null);
const errMsg = ref<string | null>(null);

const roleOpen = ref(false);
const resetOpen = ref(false);
const deleteOpen = ref(false);

const isSelf = computed(() => target.value?.id === state.user?.id);
const isDeleted = computed(() => !!target.value?.deleted_at);
const actionable = computed(() => !isSelf.value && !isDeleted.value);
const targetRole = computed(() => target.value?.role ?? "user");
const newRole = computed<"user" | "admin">(() => (targetRole.value === "admin" ? "user" : "admin"));
const promoteLabel = computed(() => (targetRole.value === "admin" ? "Demote to user" : "Promote to admin"));
const promoteTitle = computed(() => (targetRole.value === "admin" ? "Demote this user?" : "Promote this user to admin?"));
const promoteMessage = computed(() =>
  targetRole.value === "admin"
    ? "They will lose access to the admin surface. Their existing sessions stay valid; the change takes effect on their next request."
    : "They will gain full admin powers (manage users and groups, configure SMTP, view the audit log). Re-enter your password to confirm.",
);

const roleErr: Record<string, string> = {
  step_up: "Step-up password did not match.",
  locked: "Too many failed step-up attempts; wait a minute.",
  last_admin: "Cannot demote the last admin.",
  error: "Could not change role.",
};
const resetErr: Record<string, string> = {
  step_up: "Step-up password did not match.",
  locked: "Too many failed step-up attempts; wait a minute.",
  smtp: "Configure SMTP first: the user receives a code by email to set a new password.",
  error: "Password reset failed.",
};
const deleteErr: Record<string, string> = {
  step_up: "Step-up password did not match.",
  locked: "Too many failed step-up attempts; wait a minute.",
  last_admin: "Cannot delete the last admin.",
  error: "Delete failed.",
};

function fmtDate(s?: string | null) {
  return s ? new Date(s).toISOString().slice(0, 10) : "";
}

async function onRole(password: string) {
  okMsg.value = null;
  errMsg.value = null;
  const res = await setUserRole(userId.value, newRole.value, password);
  if (res.ok) {
    okMsg.value = "Role updated.";
    await load();
  } else {
    errMsg.value = roleErr[res.code ?? "error"];
  }
}
async function onReset(password: string) {
  okMsg.value = null;
  errMsg.value = null;
  const res = await resetUserPassword(userId.value, password);
  if (res.ok) okMsg.value = "Reset email sent. The user's sessions are revoked and they can set a new password from the email.";
  else errMsg.value = resetErr[res.code ?? "error"];
}
async function onDelete(password: string) {
  errMsg.value = null;
  const res = await deleteUser(userId.value, password);
  if (res.ok) await router.replace("/admin/users");
  else errMsg.value = deleteErr[res.code ?? "error"];
}

async function load() {
  target.value = await getUser(userId.value);
  loaded.value = true;
}
onMounted(load);
</script>

<template>
  <AppLayout v-if="target" :back="{ to: '/admin/users', label: 'Users' }">
    <header class="mb-6 flex items-center gap-4">
      <MemberAvatar :user-id="target.id" :display-name="target.display_name" :has-avatar="target.has_avatar" :size="48" />
      <div class="min-w-0">
        <h1 class="truncate text-2xl font-semibold">{{ target.display_name }}</h1>
        <p class="truncate text-sm text-muted-foreground">{{ target.email ?? "(email scrubbed)" }}</p>
      </div>
    </header>

    <Alert v-if="okMsg" tone="success" class="mb-4">{{ okMsg }}</Alert>
    <Alert v-if="errMsg" tone="error" class="mb-4">{{ errMsg }}</Alert>

    <div class="flex flex-col gap-4">
      <section class="rounded-md border border-border bg-card p-3">
        <h2 class="mb-2 font-medium">Profile</h2>
        <dl class="grid gap-2 text-sm sm:grid-cols-[10rem_minmax(0,1fr)]">
          <dt class="text-muted-foreground">Role</dt>
          <dd><span class="rounded-sm bg-muted px-1.5 py-px">{{ targetRole }}</span></dd>
          <dt class="text-muted-foreground">Created</dt>
          <dd>{{ fmtDate(target.created_at) }}</dd>
          <template v-if="isDeleted">
            <dt class="text-muted-foreground">Deleted</dt>
            <dd class="text-[var(--destructive)]">{{ fmtDate(target.deleted_at) }}</dd>
          </template>
        </dl>
        <p v-if="isSelf" class="mt-3 text-xs text-muted-foreground">
          This is your own account. Role and delete actions are disabled here; use the account page or another admin.
        </p>
      </section>

      <template v-if="actionable">
        <section class="rounded-md border border-border bg-card p-3">
          <h2 class="mb-2 font-medium">Role</h2>
          <p class="mb-3 text-sm text-muted-foreground">
            {{ targetRole === "admin"
              ? "Demoting removes access to the admin surface. The last remaining admin cannot be demoted."
              : "Promoting gives this user full admin powers across the instance." }}
          </p>
          <div class="flex justify-end">
            <button type="button" :class="targetRole === 'admin' ? 'btn-secondary' : 'btn-primary'" @click="roleOpen = true">
              {{ promoteLabel }}
            </button>
          </div>
        </section>

        <section class="rounded-md border border-border bg-card p-3">
          <h2 class="mb-2 font-medium">Reset password</h2>
          <p class="mb-3 text-sm text-muted-foreground">Revokes every active session and emails the user a 6-digit code so they can set a new password themselves.</p>
          <div class="flex justify-end">
            <button type="button" class="btn-secondary" @click="resetOpen = true">Reset password</button>
          </div>
        </section>

        <section class="rounded-md border border-red-200 bg-card p-4 dark:border-red-900">
          <h2 class="mb-2 text-sm font-semibold uppercase tracking-wide text-red-600 dark:text-red-400">Danger zone</h2>
          <p class="mb-3 text-sm text-[var(--subtle-foreground)]">Soft-deletes the account. Email and password are scrubbed and the display name becomes a tombstone so historical ledger entries stay traceable.</p>
          <div class="flex justify-end">
            <button type="button" class="btn-danger" @click="deleteOpen = true">
              <Icon name="trash" /><span>Delete user</span>
            </button>
          </div>
        </section>
      </template>
    </div>

    <PasswordPromptDialog
      v-model:open="roleOpen"
      :title="promoteTitle"
      :message="promoteMessage"
      :confirm-label="promoteLabel"
      confirm-variant="primary"
      @confirm="onRole"
    />
    <PasswordPromptDialog
      v-model:open="resetOpen"
      title="Reset this user's password"
      message="The user receives a 6-digit code by email to set a new password themselves. Their existing sessions are revoked immediately."
      confirm-label="Send reset email"
      confirm-variant="primary"
      @confirm="onReset"
    />
    <PasswordPromptDialog
      v-model:open="deleteOpen"
      title="Delete this user?"
      message="The account is soft-deleted: their email/password are scrubbed and their display name becomes a tombstone. Existing ledger entries stay traceable."
      confirm-label="Delete user"
      confirm-icon="trash"
      @confirm="onDelete"
    />
  </AppLayout>
</template>
