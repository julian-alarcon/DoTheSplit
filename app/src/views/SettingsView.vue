<script setup lang="ts">
import { computed, ref } from "vue";
import { RouterLink, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { pixelateFile, toBase64NoPrefix } from "@/lib/avatar-pixelate";
import { useAvatarUrl } from "@/composables/useAvatarUrl";
import { toRef } from "vue";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";
import Icon from "@/components/Icon.vue";
import Field from "@/components/Field.vue";
import Avatar from "@/components/Avatar.vue";
import PasswordPromptDialog from "@/components/PasswordPromptDialog.vue";

const { state, updateProfile, requestEmailChange, confirmEmailChange, setAvatar, clearAvatar, deleteAccount } =
  useAuth();
const router = useRouter();

const user = computed(() => state.user!);

const okMsg = ref<string | null>(null);
const errMsg = ref<string | null>(null);

function flash(ok: string | null, err: string | null = null) {
  okMsg.value = ok;
  errMsg.value = err;
}

// --- Display name ------------------------------------------------------------
const displayName = ref(user.value.display_name);
async function saveName() {
  const res = await updateProfile({ display_name: displayName.value });
  flash(res.ok ? "Display name updated." : null, res.ok ? null : "Could not update your display name.");
}

// --- Avatar ------------------------------------------------------------------
const { url: storedAvatarUrl } = useAvatarUrl(
  toRef(() => user.value.id),
  toRef(() => user.value.has_avatar),
  toRef(() => user.value.avatar_updated_at ?? ""),
);
const previewUrl = ref<string | null>(null);
const pendingB64 = ref<string | null>(null);
const avatarBusy = ref(false);

async function onFilePick(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0];
  if (!file) {
    previewUrl.value = null;
    pendingB64.value = null;
    return;
  }
  try {
    const { uploadDataUrl, previewDataUrl } = await pixelateFile(file);
    previewUrl.value = previewDataUrl;
    pendingB64.value = toBase64NoPrefix(uploadDataUrl);
  } catch {
    previewUrl.value = null;
    pendingB64.value = null;
    flash(null, "Could not read that image. Pick a valid image file.");
  }
}
async function saveAvatar() {
  if (!pendingB64.value) return;
  avatarBusy.value = true;
  const res = await setAvatar(pendingB64.value);
  avatarBusy.value = false;
  if (res.ok) {
    previewUrl.value = null;
    pendingB64.value = null;
    flash("Avatar updated.");
  } else {
    flash(null, "Could not save that avatar. It must be a valid image the browser can read.");
  }
}
async function removeAvatar() {
  const res = await clearAvatar();
  flash(res.ok ? "Avatar removed." : null, res.ok ? null : "Could not remove the avatar.");
}

// --- Email change ------------------------------------------------------------
const newEmail = ref("");
const emailPassword = ref("");
const showEmailConfirm = ref(false);
const emailCode = ref("");
async function requestEmail() {
  const res = await requestEmailChange(newEmail.value, emailPassword.value);
  if (res.ok) {
    showEmailConfirm.value = true;
    emailPassword.value = "";
    flash("Code sent to your new address. Enter it below to finish.");
  } else {
    const map: Record<string, string> = {
      email_taken: "That email is already in use.",
      email_password: "Current password is incorrect.",
      email_invalid: "Could not start the email change. Try a different address.",
    };
    flash(null, map[res.code ?? "email_invalid"]);
  }
}
async function confirmEmail() {
  const res = await confirmEmailChange(emailCode.value);
  if (res.ok) {
    showEmailConfirm.value = false;
    newEmail.value = "";
    emailCode.value = "";
    flash("Email updated.");
  } else {
    const map: Record<string, string> = {
      email_confirm_expired: "Code expired. Request a new email change.",
      email_confirm_invalid: "That code didn't match. Try again.",
    };
    flash(null, map[res.code ?? "email_confirm_invalid"]);
  }
}

// --- Preferences + timezone --------------------------------------------------
const weekStart = ref<0 | 1>(user.value.week_start === 0 ? 0 : 1);
async function savePrefs() {
  const res = await updateProfile({ week_start: weekStart.value });
  flash(res.ok ? "Preferences updated." : null, res.ok ? null : "Could not update your preferences.");
}

const timezone = ref(user.value.timezone ?? "");
const allTimezones = (() => {
  const fn = (Intl as unknown as { supportedValuesOf?: (k: string) => string[] }).supportedValuesOf;
  return typeof fn === "function"
    ? fn("timeZone")
    : ["UTC", "Europe/Madrid", "Europe/London", "Europe/Berlin", "America/New_York", "America/Los_Angeles", "Asia/Tokyo"];
})();
async function saveTimezone() {
  const res = await updateProfile({ timezone: timezone.value });
  flash(res.ok ? "Timezone updated." : null, res.ok ? null : "Could not update your timezone.");
}

// --- Delete account ----------------------------------------------------------
const deleteOpen = ref(false);
async function onDelete(password: string) {
  const res = await deleteAccount(password);
  if (res.ok) {
    await router.replace("/login");
  } else {
    const map: Record<string, string> = {
      delete_locked: "Too many failed password attempts. Try again in a minute.",
      delete_password: "Password is incorrect. Account was not deleted.",
    };
    flash(null, map[res.code ?? "delete_password"]);
  }
}
</script>

<template>
  <AppLayout>
    <div class="wrap">
      <div class="hero">
        <Avatar :display-name="user.display_name" :src="previewUrl ?? storedAvatarUrl" :size="56" />
        <div>
          <h1 class="name">{{ user.display_name }}</h1>
          <p class="email">{{ user.email }}</p>
        </div>
      </div>

      <Alert v-if="errMsg" tone="error">{{ errMsg }}</Alert>
      <Alert v-if="okMsg" tone="success">{{ okMsg }}</Alert>

      <!-- Display name -->
      <section class="panel">
        <h2 class="panel-title">Display name</h2>
        <form class="form" @submit.prevent="saveName">
          <Field v-model="displayName" label="Display name" type="text" required minlength="1" maxlength="80" error="Required" />
          <div class="right"><button type="submit" class="btn-primary">Save name</button></div>
        </form>
      </section>

      <!-- Avatar -->
      <section class="panel">
        <h2 class="panel-title">Avatar</h2>
        <p class="muted mb">
          Pick any image. We'll pixelate it in your browser to a fun 8×8 mosaic and upload only that 64-pixel tile.
        </p>
        <div class="avatar-row">
          <div class="avatar-slot">
            <Avatar :display-name="user.display_name" :src="previewUrl ?? storedAvatarUrl" :size="64" />
          </div>
          <input type="file" accept="image/*" class="file" @change="onFilePick" />
        </div>
        <div class="right gap mt">
          <button v-if="user.has_avatar" type="button" class="btn-secondary" @click="removeAvatar">Remove avatar</button>
          <button type="button" class="btn-primary" :disabled="!pendingB64 || avatarBusy" @click="saveAvatar">Save avatar</button>
        </div>
      </section>

      <!-- Password -->
      <section class="panel">
        <h2 class="panel-title">Password</h2>
        <p class="muted mb">Change your password or recover it by email.</p>
        <div class="right">
          <RouterLink to="/settings/password" class="btn-primary">Manage password</RouterLink>
        </div>
      </section>

      <!-- Email -->
      <section class="panel">
        <h2 class="panel-title">Email address</h2>
        <p class="muted mb">
          Current address: <strong>{{ user.email }}</strong>. Changing it sends a 6-digit code to the new address; your old address keeps working until you confirm.
        </p>
        <form class="form" @submit.prevent="requestEmail">
          <Field v-model="newEmail" label="New email" type="email" required maxlength="254" error="Enter a valid email address" />
          <Field v-model="emailPassword" label="Current password" type="password" required autocomplete="current-password" error="Required" />
          <div class="right"><button type="submit" class="btn-primary">Send code to new email</button></div>
        </form>
        <form v-if="showEmailConfirm" class="form confirm" @submit.prevent="confirmEmail">
          <p class="muted">Enter the 6-digit code we just sent to your new address.</p>
          <Field
            v-model="emailCode"
            label="6-digit code"
            type="text"
            required
            inputmode="numeric"
            autocomplete="one-time-code"
            pattern="[0-9]{6}"
            minlength="6"
            maxlength="6"
            class="code-input"
            error="Enter the 6-digit code from your email"
          />
          <div class="right"><button type="submit" class="btn-primary">Confirm new email</button></div>
        </form>
      </section>

      <!-- Notifications -->
      <section class="panel">
        <h2 class="panel-title">Notifications</h2>
        <p class="muted mb">Choose which transactions should reach you.</p>
        <div class="right">
          <RouterLink to="/settings/notifications" class="btn-primary">Manage notifications</RouterLink>
        </div>
      </section>

      <!-- Preferences -->
      <section class="panel">
        <h2 class="panel-title">Preferences</h2>
        <form class="form" @submit.prevent="savePrefs">
          <label class="field-select-row">
            <span>Week starts on</span>
            <select v-model.number="weekStart" class="field-select">
              <option :value="1">Monday</option>
              <option :value="0">Sunday</option>
            </select>
          </label>
          <p class="hint">Used by the calendar in date pickers across the app.</p>
          <div class="right"><button type="submit" class="btn-primary">Save preferences</button></div>
        </form>
      </section>

      <!-- Timezone -->
      <section class="panel">
        <h2 class="panel-title">Timezone</h2>
        <form class="form" @submit.prevent="saveTimezone">
          <label class="field-select-row">
            <span>Display times in</span>
            <select v-model="timezone" class="field-select">
              <option value="">Use device timezone</option>
              <option v-for="z in allTimezones" :key="z" :value="z">{{ z }}</option>
            </select>
          </label>
          <p class="hint">
            "Use device timezone" follows your browser/OS automatically. Override only if you want a fixed zone.
          </p>
          <div class="right"><button type="submit" class="btn-primary">Save timezone</button></div>
        </form>
      </section>

      <!-- Danger zone -->
      <section class="panel danger">
        <h2 class="danger-title">Danger zone</h2>
        <p class="muted mb">
          Deleting your account removes your email, password and avatar. Your name is replaced with a stable tombstone so shared expenses other members still depend on stay traceable. This cannot be undone.
        </p>
        <div class="right">
          <button type="button" class="btn-danger" @click="deleteOpen = true">
            <Icon name="trash" /><span>Delete account</span>
          </button>
        </div>
      </section>
    </div>

    <PasswordPromptDialog
      v-model:open="deleteOpen"
      title="Delete your account?"
      message="Re-enter your password to confirm. Expenses you paid for stay visible under a tombstone so other members can still trace shared ledgers. This cannot be undone."
      confirm-label="Delete account"
      confirm-icon="trash"
      @confirm="onDelete"
    />
  </AppLayout>
</template>

<style scoped>
.wrap {
  margin-inline: auto;
  display: flex;
  max-width: 42rem;
  flex-direction: column;
  gap: 0.5rem;
}
.hero {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 0.5rem;
}
.name {
  font-size: 1.5rem;
  font-weight: 600;
}
.email {
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.panel {
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
}
.panel-title {
  margin-bottom: 0.75rem;
  font-weight: 500;
}
.form {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.confirm {
  margin-top: 1rem;
  border-top: 1px solid var(--border);
  padding-top: 1rem;
}
.right {
  display: flex;
  justify-content: flex-end;
}
.gap {
  gap: 0.5rem;
}
.mt {
  margin-top: 0.75rem;
}
.mb {
  margin-bottom: 0.75rem;
}
.muted {
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.hint {
  margin-top: -0.25rem;
  font-size: 0.75rem;
  color: var(--muted-foreground);
}
.avatar-row {
  display: flex;
  align-items: center;
  gap: 1rem;
}
.avatar-slot {
  flex-shrink: 0;
  overflow: hidden;
  border-radius: 0.25rem;
  border: 1px solid var(--border);
}
.file {
  display: block;
  width: 100%;
  font-size: 0.875rem;
}
.file::file-selector-button {
  margin-right: 0.75rem;
  border: 0;
  border-radius: 0.375rem;
  background: var(--primary);
  color: var(--primary-foreground);
  padding: 0.5rem 0.75rem;
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
}
.file::file-selector-button:hover {
  filter: brightness(0.92);
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
:deep(.code-input) {
  text-align: center;
  letter-spacing: 0.4em;
}
</style>
