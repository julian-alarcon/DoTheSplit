<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import {
  getSmtp,
  revealSmtpPassword,
  sendSmtpTestEmail,
  testSmtp,
  updateSmtp,
  type SmtpTlsMode,
} from "@/composables/useAdmin";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";
import Icon from "@/components/Icon.vue";

const loaded = ref(false);
const notConfigured = ref(false);
const passwordSet = ref(false);

const form = ref({
  host: "",
  port: 587,
  tls_mode: "starttls" as SmtpTlsMode,
  username: "",
  from_address: "",
  smtp_password: "",
});
const showPassword = ref(false);

const okMsg = ref<string | null>(null);
const errMsg = ref<string | null>(null);
const testMsg = ref<{ ok: boolean; msg: string } | null>(null);

const tlsWarning = computed(() => form.value.tls_mode === "none");

const testMessages: Record<string, string> = {
  dial_timeout: "Could not connect (dial timeout).",
  tls_handshake_failed: "TLS handshake failed.",
  starttls_failed: "STARTTLS upgrade failed.",
  smtp_handshake_failed: "SMTP handshake failed.",
  auth_failed: "Authentication failed.",
  password_decrypt_failed: "Stored password could not be decrypted.",
  noop_failed: "NOOP command failed.",
  not_configured: "SMTP is not configured yet.",
  error: "SMTP test errored.",
};
const sendMessages: Record<string, string> = {
  fail: "Test email failed to send. Double-check host, port, TLS mode, and credentials.",
  not_configured: "SMTP is not configured yet.",
  error: "Test email errored.",
};

async function onSave() {
  okMsg.value = null;
  errMsg.value = null;
  testMsg.value = null;
  // Empty string clears the password; we always send the field's current
  // value as the source of truth (reveal-and-resave model).
  const res = await updateSmtp({
    host: form.value.host,
    port: Number(form.value.port),
    tls_mode: form.value.tls_mode,
    username: form.value.username || null,
    from_address: form.value.from_address,
    smtp_password: form.value.smtp_password,
    allow_plaintext_credentials: form.value.tls_mode === "none",
  });
  if (!res.ok) {
    errMsg.value = "Could not save SMTP configuration.";
    return;
  }
  okMsg.value = "Saved.";
  notConfigured.value = false;
  // Save-and-test in one go.
  const t = await testSmtp();
  testMsg.value = t.success
    ? { ok: true, msg: "SMTP test connection succeeded." }
    : { ok: false, msg: testMessages[t.error ?? "error"] ?? "SMTP test errored." };
}

async function onSendTest() {
  testMsg.value = null;
  const r = await sendSmtpTestEmail();
  testMsg.value = r.success
    ? { ok: true, msg: "Test email sent. Check your inbox (and spam folder)." }
    : { ok: false, msg: sendMessages[r.error ?? "error"] ?? "Test email errored." };
}

onMounted(async () => {
  const { config, notConfigured: nc } = await getSmtp();
  notConfigured.value = nc;
  if (config) {
    form.value.host = config.host;
    form.value.port = config.port;
    form.value.tls_mode = config.tls_mode;
    form.value.username = config.username ?? "";
    form.value.from_address = config.from_address;
    passwordSet.value = config.password_set;
    // Pre-load the stored cleartext so "what's in the field is what's saved".
    // The reveal is audit-logged server-side.
    if (config.password_set) form.value.smtp_password = await revealSmtpPassword();
  }
  loaded.value = true;
});
</script>

<template>
  <AppLayout :back="{ to: '/admin', label: 'Admin' }">
    <h1 class="mb-1 text-2xl font-semibold">SMTP configuration</h1>
    <p class="mb-4 text-sm text-muted-foreground">
      Outbound email settings. The password is encrypted at rest with the same key used for emails. The cleartext is never returned by the API.
    </p>

    <Alert v-if="okMsg" tone="success" class="mb-4">{{ okMsg }}</Alert>
    <Alert v-if="errMsg" tone="error" class="mb-4">{{ errMsg }}</Alert>
    <Alert v-if="testMsg" :tone="testMsg.ok ? 'success' : 'error'" class="mb-4">{{ testMsg.msg }}</Alert>
    <Alert v-if="notConfigured" tone="info" class="mb-4">SMTP is not configured yet.</Alert>

    <form v-if="loaded" class="grid gap-3" @submit.prevent="onSave">
      <label class="field">
        <input v-model="form.host" required class="field-input" placeholder=" " />
        <span class="field-label" data-required>Host (e.g. smtp.example.com)</span>
      </label>
      <label class="field">
        <input v-model.number="form.port" type="number" min="1" max="65535" required class="field-input" placeholder=" " />
        <span class="field-label" data-required>Port</span>
      </label>
      <label class="field-select-row">
        <span>TLS mode</span>
        <select v-model="form.tls_mode" class="field-select">
          <option value="starttls">STARTTLS</option>
          <option value="tls">Implicit TLS</option>
          <option value="none">None (plaintext)</option>
        </select>
      </label>
      <Alert v-if="tlsWarning" tone="info">
        Warning: TLS is disabled. The username, password, and every email body will travel in plain text. Not recommended outside trusted local networks.
      </Alert>
      <label class="field">
        <input v-model="form.username" class="field-input" placeholder=" " />
        <span class="field-label">Username (optional)</span>
      </label>
      <label class="field">
        <input v-model="form.from_address" type="email" required class="field-input" placeholder=" " />
        <span class="field-label" data-required>From address</span>
      </label>
      <label class="field relative">
        <input
          v-model="form.smtp_password"
          :type="showPassword ? 'text' : 'password'"
          autocomplete="new-password"
          class="field-input pr-10"
          placeholder=" "
        />
        <span class="field-label">SMTP password</span>
        <button
          type="button"
          class="absolute right-2 top-1/2 grid -translate-y-1/2 cursor-pointer place-items-center rounded-sm p-1 text-muted-foreground hover:text-foreground"
          :aria-label="showPassword ? 'Hide password' : 'Show password'"
          :aria-pressed="showPassword"
          @click="showPassword = !showPassword"
        >
          <Icon :name="showPassword ? 'eye-slash' : 'eye'" />
        </button>
      </label>

      <div class="flex flex-wrap justify-end gap-2">
        <button type="button" class="btn-secondary" :disabled="notConfigured" @click="onSendTest">Send test email</button>
        <button type="submit" class="btn-primary">Save and test connection</button>
      </div>
    </form>
  </AppLayout>
</template>
