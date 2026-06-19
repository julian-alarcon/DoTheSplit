<script setup lang="ts">
import AppLayout from "@/components/AppLayout.vue";
import credits from "@/lib/credits.json";

// `inter` is optional: the SPA uses system fonts, so newer credits.json files
// drop the self-hosted-Inter block. Read it defensively so the page renders
// whether or not the key is present.
const { project, fontAwesome, backend, frontend } = credits;
const inter = (credits as { inter?: {
  creator: string;
  creatorUrl: string;
  license: string;
  licenseUrl: string;
  modificationStatement: string;
} }).inter;

// Build identity, baked in at image-build time via Vite define (see
// vite.config.ts). Falls back to "dev" for local dev.
const buildCommit = (import.meta.env.VITE_BUILD_COMMIT ?? "dev").slice(0, 12);
const buildVersion = import.meta.env.VITE_BUILD_VERSION ?? "dev";
const isReleasedVersion =
  buildVersion !== "dev" && !buildVersion.includes("-dev");
</script>

<template>
  <AppLayout>
    <div class="mx-auto max-w-3xl">
      <h1 class="mb-2 text-2xl font-semibold">About</h1>
      <p class="mb-6 text-sm text-muted-foreground">
        {{ project.name }} is a small, self-hosted expense-sharing app.
      </p>

      <section class="mb-8 rounded-md border border-border bg-card p-3 text-center">
        <h2 class="text-lg font-semibold">{{ project.name }}</h2>
        <p class="mb-3 text-sm text-muted-foreground">
          <a
            v-if="isReleasedVersion"
            :href="`${project.url}/releases/tag/v${buildVersion}`"
            class="underline transition-colors hover:text-primary [font-family:var(--font-mono)]"
            rel="noopener noreferrer"
            target="_blank"
            >v{{ buildVersion }}</a
          >
          <code v-else class="[font-family:var(--font-mono)]">{{ buildVersion }}</code>
          <template v-if="buildCommit !== 'dev'">
            (<a
              :href="`${project.url}/commit/${buildCommit}`"
              class="underline transition-colors hover:text-primary [font-family:var(--font-mono)]"
              rel="noopener noreferrer"
              target="_blank"
              >{{ buildCommit }}</a
            >)
          </template>
        </p>
        <p class="text-sm">
          {{ project.copyright }}. Released under the
          <a :href="`${project.url}/blob/main/LICENSE`" class="underline transition-colors hover:text-primary" rel="noopener noreferrer" target="_blank">{{ project.license }} license</a>.
        </p>
        <p class="mt-2 text-sm">
          Source on
          <a :href="project.url" class="underline transition-colors hover:text-primary" rel="noopener noreferrer" target="_blank">GitHub</a>.
          Support development via
          <a :href="project.sponsorUrl" class="underline transition-colors hover:text-primary" rel="noopener noreferrer" target="_blank">GitHub Sponsors</a>.
        </p>
      </section>

      <h2 class="mb-2 mt-8 text-lg font-semibold">Credits</h2>
      <p class="mb-6 text-sm text-muted-foreground">Built on open-source software. Thanks to all the maintainers below.</p>

      <section class="mb-8 rounded-md border border-border bg-card p-3">
        <h2 class="mb-2 text-lg font-semibold">Font Awesome Free Icons</h2>
        <p class="text-sm">
          Icons by
          <a :href="fontAwesome.creatorUrl" class="underline transition-colors hover:text-primary" rel="noopener noreferrer" target="_blank">{{ fontAwesome.creator }}</a>,
          licensed under
          <a :href="fontAwesome.licenseUrl" class="underline transition-colors hover:text-primary" rel="noopener noreferrer" target="_blank">{{ fontAwesome.license }}</a>
          (<a :href="fontAwesome.licensePageUrl" class="underline transition-colors hover:text-primary" rel="noopener noreferrer" target="_blank">license page</a>).
          {{ fontAwesome.modificationStatement }}
        </p>
      </section>

      <section v-if="inter" class="mb-8 rounded-md border border-border bg-card p-3">
        <h2 class="mb-2 text-lg font-semibold">Inter Font</h2>
        <p class="text-sm">
          Body type by
          <a :href="inter.creatorUrl" class="underline transition-colors hover:text-primary" rel="noopener noreferrer" target="_blank">{{ inter.creator }}</a>,
          licensed under the
          <a :href="inter.licenseUrl" class="underline transition-colors hover:text-primary" rel="noopener noreferrer" target="_blank">{{ inter.license }}</a>.
          {{ inter.modificationStatement }}
        </p>
      </section>

      <section class="mb-8">
        <h2 class="mb-2 text-lg font-semibold">Backend dependencies</h2>
        <p class="mb-3 text-sm text-muted-foreground">Direct Go modules from <code class="[font-family:var(--font-mono)]">api/go.mod</code>.</p>
        <div class="overflow-x-auto rounded-md border border-border [&_th]:bg-muted [&_th]:text-left [&_th]:px-3 [&_th]:py-2 [&_th]:font-medium [&_td]:border-t [&_td]:border-border [&_td]:px-3 [&_td]:py-2">
          <table class="w-full border-collapse text-sm">
            <thead>
              <tr>
                <th>Module</th>
                <th>Version</th>
                <th>License</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="d in backend" :key="d.name">
                <td><a :href="d.url" class="underline transition-colors hover:text-primary" rel="noopener noreferrer" target="_blank">{{ d.name }}</a></td>
                <td class="text-xs [font-family:var(--font-mono)]">{{ d.version }}</td>
                <td>{{ d.license }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="mb-8">
        <h2 class="mb-2 text-lg font-semibold">Frontend dependencies</h2>
        <p class="mb-3 text-sm text-muted-foreground">Direct npm packages.</p>
        <div class="overflow-x-auto rounded-md border border-border [&_th]:bg-muted [&_th]:text-left [&_th]:px-3 [&_th]:py-2 [&_th]:font-medium [&_td]:border-t [&_td]:border-border [&_td]:px-3 [&_td]:py-2">
          <table class="w-full border-collapse text-sm">
            <thead>
              <tr>
                <th>Package</th>
                <th>Version</th>
                <th>License</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="d in frontend" :key="d.name">
                <td><a :href="d.url" class="underline transition-colors hover:text-primary" rel="noopener noreferrer" target="_blank">{{ d.name }}</a></td>
                <td class="text-xs [font-family:var(--font-mono)]">{{ d.version }}</td>
                <td>{{ d.license }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="mb-8 text-sm text-muted-foreground">
        <p>
          Full transitive attribution lives in
          <a :href="`${project.url}/blob/main/THIRD_PARTY_LICENSES.md`" class="underline transition-colors hover:text-primary" rel="noopener noreferrer" target="_blank">THIRD_PARTY_LICENSES.md</a>.
        </p>
        <p class="mt-2">
          CycloneDX SBOMs are published as artifacts on each
          <a :href="`${project.url}/releases`" class="underline transition-colors hover:text-primary" rel="noopener noreferrer" target="_blank">GitHub Release</a>.
        </p>
      </section>
    </div>
  </AppLayout>
</template>
