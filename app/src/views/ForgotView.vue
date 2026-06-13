<script setup lang="ts">
import { ref } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import AppLayout from "@/components/AppLayout.vue";
import Field from "@/components/Field.vue";

const { requestPasswordReset } = useAuth();
const route = useRoute();
const router = useRouter();

const email = ref(typeof route.query.email === "string" ? route.query.email : "");
const submitting = ref(false);

async function onSubmit() {
  submitting.value = true;
  await requestPasswordReset(email.value);
  submitting.value = false;
  // The API returns 204 either way (no enumeration); always advance to /reset.
  await router.push({ path: "/reset", query: { email: email.value } });
}
</script>

<template>
  <AppLayout>
    <section class="card">
      <h1 class="title">Reset your password</h1>
      <p class="lead">
        Type your email and we'll send you a 6-digit code to set a new password.
        If the email is registered, the code will arrive in a moment.
      </p>
      <form class="form" @submit.prevent="onSubmit">
        <Field
          v-model="email"
          label="Email"
          type="email"
          required
          autocomplete="email"
          error="Enter a valid email address"
        />
        <button type="submit" class="btn-primary submit" :disabled="submitting">
          Send code
        </button>
      </form>
      <p class="foot">
        <RouterLink to="/login" class="link">Back to login</RouterLink>
      </p>
    </section>
  </AppLayout>
</template>

<style scoped>
.card {
  margin-inline: auto;
  max-width: 24rem;
  padding-block: 1.5rem;
}
.title {
  font-size: 1.5rem;
  font-weight: 600;
  margin-bottom: 0.5rem;
}
.lead {
  margin-bottom: 1.5rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.form {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
}
.submit {
  margin-top: 0.5rem;
  align-self: flex-end;
}
.foot {
  margin-top: 1rem;
  font-size: 0.875rem;
}
.link {
  text-decoration: underline;
}
</style>
