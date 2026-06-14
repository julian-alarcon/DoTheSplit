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
    <header class="hero">
      <MemberAvatar :user-id="target.id" :display-name="target.display_name" :has-avatar="target.has_avatar" :size="48" />
      <div class="hero-text">
        <h1 class="name">{{ target.display_name }}</h1>
        <p class="email">{{ target.email ?? "(email scrubbed)" }}</p>
      </div>
    </header>

    <Alert v-if="okMsg" tone="success" class="banner">{{ okMsg }}</Alert>
    <Alert v-if="errMsg" tone="error" class="banner">{{ errMsg }}</Alert>

    <div class="stack">
      <section class="panel">
        <h2 class="panel-title">Profile</h2>
        <dl class="profile">
          <dt class="muted">Role</dt>
          <dd><span class="role-badge">{{ targetRole }}</span></dd>
          <dt class="muted">Created</dt>
          <dd>{{ fmtDate(target.created_at) }}</dd>
          <template v-if="isDeleted">
            <dt class="muted">Deleted</dt>
            <dd class="deleted">{{ fmtDate(target.deleted_at) }}</dd>
          </template>
        </dl>
        <p v-if="isSelf" class="hint mt">
          This is your own account. Role and delete actions are disabled here; use the account page or another admin.
        </p>
      </section>

      <template v-if="actionable">
        <section class="panel">
          <h2 class="panel-title">Role</h2>
          <p class="muted mb">
            {{ targetRole === "admin"
              ? "Demoting removes access to the admin surface. The last remaining admin cannot be demoted."
              : "Promoting gives this user full admin powers across the instance." }}
          </p>
          <div class="right">
            <button type="button" :class="targetRole === 'admin' ? 'btn-secondary' : 'btn-primary'" @click="roleOpen = true">
              {{ promoteLabel }}
            </button>
          </div>
        </section>

        <section class="panel">
          <h2 class="panel-title">Reset password</h2>
          <p class="muted mb">Revokes every active session and emails the user a 6-digit code so they can set a new password themselves.</p>
          <div class="right">
            <button type="button" class="btn-secondary" @click="resetOpen = true">Reset password</button>
          </div>
        </section>

        <section class="panel danger">
          <h2 class="danger-title">Danger zone</h2>
          <p class="muted mb">Soft-deletes the account. Email and password are scrubbed and the display name becomes a tombstone so historical ledger entries stay traceable.</p>
          <div class="right">
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

<style scoped>
.hero {
  margin-bottom: 1.5rem;
  display: flex;
  align-items: center;
  gap: 1rem;
}
.hero-text {
  min-width: 0;
}
.name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 1.5rem;
  font-weight: 600;
}
.email {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.banner {
  margin-bottom: 1rem;
}
.stack {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.panel {
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
}
.panel-title {
  margin-bottom: 0.5rem;
  font-weight: 500;
}
.profile {
  display: grid;
  gap: 0.5rem;
  font-size: 0.875rem;
}
@media (min-width: 640px) {
  .profile {
    grid-template-columns: 10rem minmax(0, 1fr);
  }
}
.role-badge {
  border-radius: 0.25rem;
  background: var(--muted);
  padding: 0.0625rem 0.375rem;
}
.muted {
  color: var(--muted-foreground);
}
.hint {
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.mt {
  margin-top: 0.75rem;
}
.mb {
  margin-bottom: 0.75rem;
  font-size: 0.875rem;
}
.deleted {
  color: var(--destructive);
}
.right {
  display: flex;
  justify-content: flex-end;
}
.danger {
  border-color: color-mix(in oklch, var(--destructive) 40%, var(--border));
}
.danger-title {
  margin-bottom: 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--destructive);
}
</style>
