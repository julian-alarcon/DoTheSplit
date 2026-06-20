<script setup lang="ts">
import { computed, ref, toRef } from "vue";
import { RouterLink, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { pixelateFile, toBase64NoPrefix } from "@/lib/avatar-pixelate";
import { useAvatarUrl } from "@/composables/useAvatarUrl";
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

// --- Preferences -------------------------------------------------------------
const weekStart = ref<0 | 1>(user.value.week_start === 0 ? 0 : 1);
async function savePrefs() {
  const res = await updateProfile({ week_start: weekStart.value });
  flash(res.ok ? "Preferences updated." : null, res.ok ? null : "Could not update your preferences.");
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
    <div class="mx-auto flex max-w-2xl flex-col gap-2">
      <div class="mb-2 flex items-center gap-4">
        <Avatar :display-name="user.display_name" :src="previewUrl ?? storedAvatarUrl" :size="56" />
        <div>
          <h1 class="text-2xl font-semibold">{{ user.display_name }}</h1>
          <p class="text-sm text-muted-foreground">{{ user.email }}</p>
        </div>
      </div>

      <Alert v-if="errMsg" tone="error">{{ errMsg }}</Alert>
      <Alert v-if="okMsg" tone="success">{{ okMsg }}</Alert>

      <!-- Display name -->
      <section class="rounded-md border border-border bg-card p-3">
        <h2 class="mb-3 font-medium">Display name</h2>
        <form class="flex flex-col gap-3" @submit.prevent="saveName">
          <Field v-model="displayName" label="Display name" type="text" required minlength="1" maxlength="80" autocomplete="name" error="Required" />
          <div class="flex justify-end"><button type="submit" class="btn-primary">Save name</button></div>
        </form>
      </section>

      <!-- Avatar -->
      <section class="rounded-md border border-border bg-card p-3">
        <h2 class="mb-3 font-medium">Avatar</h2>
        <p class="mb-3 text-sm text-subtle-foreground">
          Pick any image. We'll pixelate it in your browser to a fun 8×8 mosaic and upload only that 64-pixel tile.
        </p>
        <div class="flex items-center gap-4">
          <div class="shrink-0 overflow-hidden rounded-sm border border-border">
            <Avatar :display-name="user.display_name" :src="previewUrl ?? storedAvatarUrl" :size="64" />
          </div>
          <input type="file" accept="image/*" class="field-file" @change="onFilePick" />
        </div>
        <div class="mt-3 flex justify-end gap-2">
          <button v-if="user.has_avatar" type="button" class="btn-secondary" @click="removeAvatar">Remove avatar</button>
          <button type="button" class="btn-primary" :disabled="!pendingB64 || avatarBusy" @click="saveAvatar">Save avatar</button>
        </div>
      </section>

      <!-- Password -->
      <section class="rounded-md border border-border bg-card p-3">
        <h2 class="mb-3 font-medium">Password</h2>
        <p class="mb-3 text-sm text-subtle-foreground">Change your password or recover it by email.</p>
        <div class="flex justify-end">
          <RouterLink to="/settings/password" class="btn-primary">Manage password</RouterLink>
        </div>
      </section>

      <!-- Email -->
      <section class="rounded-md border border-border bg-card p-3">
        <h2 class="mb-3 font-medium">Email address</h2>
        <p class="mb-3 text-sm text-subtle-foreground">
          Current address: <strong>{{ user.email }}</strong>. Changing it sends a 6-digit code to the new address; your old address keeps working until you confirm.
        </p>
        <form class="flex flex-col gap-3" @submit.prevent="requestEmail">
          <Field v-model="newEmail" label="New email" type="email" required maxlength="254" autocomplete="email" error="Enter a valid email address" />
          <Field v-model="emailPassword" label="Current password" type="password" required autocomplete="current-password" error="Required" />
          <div class="flex justify-end"><button type="submit" class="btn-primary">Send code to new email</button></div>
        </form>
        <form v-if="showEmailConfirm" class="mt-4 flex flex-col gap-3 border-t border-border pt-4" @submit.prevent="confirmEmail">
          <p class="text-sm text-subtle-foreground">Enter the 6-digit code we just sent to your new address.</p>
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
            class="text-center tracking-[0.4em]"
            error="Enter the 6-digit code from your email"
          />
          <div class="flex justify-end"><button type="submit" class="btn-primary">Confirm new email</button></div>
        </form>
      </section>

      <!-- Notifications -->
      <section class="rounded-md border border-border bg-card p-3">
        <h2 class="mb-3 font-medium">Notifications</h2>
        <p class="mb-3 text-sm text-subtle-foreground">Choose which transactions should reach you.</p>
        <div class="flex justify-end">
          <RouterLink to="/settings/notifications" class="btn-primary">Manage notifications</RouterLink>
        </div>
      </section>

      <!-- Preferences -->
      <section class="rounded-md border border-border bg-card p-3">
        <h2 class="mb-3 font-medium">Preferences</h2>
        <form class="flex flex-col gap-3" @submit.prevent="savePrefs">
          <label class="field-select-row">
            <span>Week starts on</span>
            <select v-model.number="weekStart" class="field-select">
              <option :value="1">Monday</option>
              <option :value="0">Sunday</option>
            </select>
          </label>
          <p class="-mt-1 text-xs text-subtle-foreground">Used by the calendar in date pickers across the app.</p>
          <div class="flex justify-end"><button type="submit" class="btn-primary">Save preferences</button></div>
        </form>
      </section>

      <!-- Danger zone -->
      <section class="rounded-md border border-red-200 bg-card p-4 dark:border-red-900">
        <h2 class="mb-2 text-sm font-semibold uppercase tracking-wide text-red-600 dark:text-red-400">Danger zone</h2>
        <p class="mb-3 text-sm text-subtle-foreground">
          Deleting your account removes your email, password and avatar. Your name is replaced with a stable tombstone so shared expenses other members still depend on stay traceable. This cannot be undone.
        </p>
        <div class="flex justify-end">
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
