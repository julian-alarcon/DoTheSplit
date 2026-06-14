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
</script>

<template>
  <AppLayout>
    <div class="about">
      <h1 class="h1">About</h1>
      <p class="lead">
        {{ project.name }} is a small, self-hosted expense-sharing app.
      </p>

      <section class="panel">
        <h2 class="h2">{{ project.name }}</h2>
        <p class="body">
          {{ project.copyright }}. Released under the
          <a :href="`${project.url}/blob/main/LICENSE`" class="link" rel="noopener noreferrer" target="_blank">{{ project.license }} license</a>.
        </p>
        <p class="body mt">
          Source on
          <a :href="project.url" class="link" rel="noopener noreferrer" target="_blank">GitHub</a>.
          Support development via
          <a :href="project.sponsorUrl" class="link" rel="noopener noreferrer" target="_blank">GitHub Sponsors</a>.
        </p>
      </section>

      <h2 class="h2 mt-lg">Credits</h2>
      <p class="lead">Built on open-source software. Thanks to all the maintainers below.</p>

      <section class="panel">
        <h2 class="h2">Font Awesome Free Icons</h2>
        <p class="body">
          Icons by
          <a :href="fontAwesome.creatorUrl" class="link" rel="noopener noreferrer" target="_blank">{{ fontAwesome.creator }}</a>,
          licensed under
          <a :href="fontAwesome.licenseUrl" class="link" rel="noopener noreferrer" target="_blank">{{ fontAwesome.license }}</a>
          (<a :href="fontAwesome.licensePageUrl" class="link" rel="noopener noreferrer" target="_blank">license page</a>).
          {{ fontAwesome.modificationStatement }}
        </p>
      </section>

      <section v-if="inter" class="panel">
        <h2 class="h2">Inter Font</h2>
        <p class="body">
          Body type by
          <a :href="inter.creatorUrl" class="link" rel="noopener noreferrer" target="_blank">{{ inter.creator }}</a>,
          licensed under the
          <a :href="inter.licenseUrl" class="link" rel="noopener noreferrer" target="_blank">{{ inter.license }}</a>.
          {{ inter.modificationStatement }}
        </p>
      </section>

      <section class="block">
        <h2 class="h2">Backend dependencies</h2>
        <p class="sub">Direct Go modules from <code class="mono">api/go.mod</code>.</p>
        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>Module</th>
                <th>Version</th>
                <th>License</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="d in backend" :key="d.name">
                <td><a :href="d.url" class="link" rel="noopener noreferrer" target="_blank">{{ d.name }}</a></td>
                <td class="mono small">{{ d.version }}</td>
                <td>{{ d.license }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="block">
        <h2 class="h2">Frontend dependencies</h2>
        <p class="sub">Direct npm packages.</p>
        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr>
                <th>Package</th>
                <th>Version</th>
                <th>License</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="d in frontend" :key="d.name">
                <td><a :href="d.url" class="link" rel="noopener noreferrer" target="_blank">{{ d.name }}</a></td>
                <td class="mono small">{{ d.version }}</td>
                <td>{{ d.license }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="block muted">
        <p>
          Full transitive attribution lives in
          <a :href="`${project.url}/blob/main/THIRD_PARTY_LICENSES.md`" class="link" rel="noopener noreferrer" target="_blank">THIRD_PARTY_LICENSES.md</a>.
        </p>
        <p class="mt">
          CycloneDX SBOMs are published as artifacts on each
          <a :href="`${project.url}/releases`" class="link" rel="noopener noreferrer" target="_blank">GitHub Release</a>.
        </p>
      </section>
    </div>
  </AppLayout>
</template>

<style scoped>
.about {
  margin-inline: auto;
  max-width: 48rem;
}
.h1 {
  font-size: 1.5rem;
  font-weight: 600;
  margin-bottom: 0.5rem;
}
.h2 {
  font-size: 1.125rem;
  font-weight: 600;
  margin-bottom: 0.5rem;
}
.mt-lg {
  margin-top: 2rem;
}
.lead {
  margin-bottom: 1.5rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.panel {
  margin-bottom: 2rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem;
}
.block {
  margin-bottom: 2rem;
}
.body {
  font-size: 0.875rem;
}
.sub {
  margin-bottom: 0.75rem;
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
.mt {
  margin-top: 0.5rem;
}
.link {
  text-decoration: underline;
}
.table-wrap {
  overflow-x: auto;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
}
.table {
  width: 100%;
  font-size: 0.875rem;
  border-collapse: collapse;
}
.table th {
  background: var(--muted);
  text-align: left;
  padding: 0.5rem 0.75rem;
  font-weight: 500;
}
.table td {
  border-top: 1px solid var(--border);
  padding: 0.5rem 0.75rem;
}
.mono {
  font-family: var(--font-mono);
}
.small {
  font-size: 0.75rem;
}
.muted {
  font-size: 0.875rem;
  color: var(--muted-foreground);
}
</style>
