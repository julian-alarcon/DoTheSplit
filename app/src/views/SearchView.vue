<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useAuth } from "@/composables/useAuth";
import { listCategories, listGroups } from "@/composables/useGroups";
import { search, type SearchGroupRef } from "@/composables/useSearch";
import type { components } from "@/lib/api/schema";
import { moneyFormatter, formatMoney } from "@/lib/currencies";
import { shortName } from "@/lib/short-name";
import AppLayout from "@/components/AppLayout.vue";
import Alert from "@/components/Alert.vue";
import Icon from "@/components/Icon.vue";
import CategoryIcon from "@/components/CategoryIcon.vue";
import MemberAvatar from "@/components/MemberAvatar.vue";

type SearchItem = components["schemas"]["TransactionItem"];
type Category = components["schemas"]["Category"];

const route = useRoute();
const router = useRouter();
const { state } = useAuth();

const q = ref(typeof route.query.q === "string" ? route.query.q : "");
const groupFilter = ref(typeof route.query.group_id === "string" ? route.query.group_id : "");
const categoryFilter = ref(typeof route.query.category_id === "string" ? route.query.category_id : "");

const items = ref<SearchItem[]>([]);
const resultGroups = ref<SearchGroupRef[]>([]);
const availableCategoryIDs = ref<string[]>([]);
const usedQuery = ref("");
const queryError = ref<string | null>(null);
const hasSearched = ref(false);

const allGroups = ref<SearchGroupRef[]>([]);
const categories = ref<Category[]>([]);
const filtersOpen = ref(false);

const groupDialog = ref<HTMLDialogElement | null>(null);
const categoryDialog = ref<HTMLDialogElement | null>(null);

const categoryByID = computed(() => new Map(categories.value.map((c) => [c.id, c])));
const viewerId = computed(() => state.user?.id ?? "");

const groupByID = computed(() => {
  const m = new Map<string, SearchGroupRef>();
  for (const g of resultGroups.value) m.set(g.id, g);
  for (const g of allGroups.value) if (!m.has(g.id)) m.set(g.id, g);
  return m;
});

const selectedGroupLabel = computed(
  () => allGroups.value.find((g) => g.id === groupFilter.value)?.name ?? "All",
);
const selectedCategory = computed(() => categoryByID.value.get(categoryFilter.value));
const selectedCategoryLabel = computed(() => selectedCategory.value?.label ?? "All");

const visibleCategories = computed(() => {
  if (!hasSearched.value) return categories.value;
  const set = new Set(availableCategoryIDs.value);
  if (categoryFilter.value) set.add(categoryFilter.value);
  return categories.value.filter((c) => set.has(c.id));
});

const monthFmt = new Intl.DateTimeFormat(undefined, { month: "short" });
const dayFmt = new Intl.DateTimeFormat(undefined, { day: "2-digit" });
const yearFmt = new Intl.DateTimeFormat(undefined, { year: "numeric" });

type ViewerStake = { kind: "lent"; cents: number } | { kind: "owes"; cents: number } | { kind: "none" };
function viewerStake(e: {
  payer_id: string;
  amount_cents: number;
  splits: Array<{ user_id: string; share_cents: number }>;
}): ViewerStake {
  const me = viewerId.value;
  if (!me) return { kind: "none" };
  const myShare = e.splits.find((s) => s.user_id === me)?.share_cents ?? 0;
  const iPaid = e.payer_id === me;
  if (!iPaid && !e.splits.some((s) => s.user_id === me)) return { kind: "none" };
  if (iPaid) {
    const lent = e.amount_cents - myShare;
    return lent > 0 ? { kind: "lent", cents: lent } : { kind: "none" };
  }
  return myShare > 0 ? { kind: "owes", cents: myShare } : { kind: "none" };
}

function groupForItem(item: SearchItem): SearchGroupRef | undefined {
  const gid = item.expense?.group_id ?? item.settlement?.group_id;
  return gid ? groupByID.value.get(gid) : undefined;
}

// Build group-header + item rows from the flat result list.
type Row =
  | { kind: "group-header"; group: SearchGroupRef; count: number }
  | { kind: "item"; item: SearchItem };
const rows = computed<Row[]>(() => {
  const out: Row[] = [];
  let lastGroupID: string | null = null;
  let headerIndex = -1;
  let running = 0;
  for (const item of items.value) {
    const g = groupForItem(item);
    if (!g) continue;
    if (g.id !== lastGroupID) {
      if (headerIndex >= 0) (out[headerIndex] as Extract<Row, { kind: "group-header" }>).count = running;
      out.push({ kind: "group-header", group: g, count: 0 });
      headerIndex = out.length - 1;
      running = 0;
      lastGroupID = g.id;
    }
    out.push({ kind: "item", item });
    running++;
  }
  if (headerIndex >= 0) (out[headerIndex] as Extract<Row, { kind: "group-header" }>).count = running;
  return out;
});

function memberByID(g: SearchGroupRef) {
  return new Map(g.members.map((m) => [m.user_id, m]));
}

async function runSearch() {
  const term = q.value.trim();
  queryError.value = null;
  if (term.length === 1) {
    queryError.value = "Type at least 2 characters to search.";
    items.value = [];
    return;
  }
  if (term.length < 2) {
    items.value = [];
    hasSearched.value = false;
    return;
  }
  hasSearched.value = true;
  const { data, status } = await search(term, {
    groupId: groupFilter.value || undefined,
    categoryId: categoryFilter.value || undefined,
  });
  if (!data) {
    queryError.value = status === 400 ? "Search query is too short." : "Search failed.";
    items.value = [];
    return;
  }
  items.value = data.items;
  resultGroups.value = data.groups;
  availableCategoryIDs.value = data.available_category_ids;
  usedQuery.value = data.query;
}

// Reflect filters in the URL (shareable), then re-search.
function syncUrl() {
  const query: Record<string, string> = {};
  if (q.value.trim()) query.q = q.value.trim();
  if (groupFilter.value) query.group_id = groupFilter.value;
  if (categoryFilter.value) query.category_id = categoryFilter.value;
  router.replace({ query });
}

function onSubmit() {
  syncUrl();
  runSearch();
}
function pickGroup(id: string) {
  groupFilter.value = id;
  groupDialog.value?.close();
  syncUrl();
  runSearch();
}
function pickCategory(id: string) {
  categoryFilter.value = id;
  categoryDialog.value?.close();
  syncUrl();
  runSearch();
}

function isNewGroupRow(list: Category[], i: number): boolean {
  return i === 0 || list[i - 1].group_label !== list[i].group_label;
}

watch(
  () => route.query,
  () => {
    q.value = typeof route.query.q === "string" ? route.query.q : "";
    groupFilter.value = typeof route.query.group_id === "string" ? route.query.group_id : "";
    categoryFilter.value = typeof route.query.category_id === "string" ? route.query.category_id : "";
  },
);

onMounted(async () => {
  filtersOpen.value = Boolean(groupFilter.value || categoryFilter.value);
  const [groups, cats] = await Promise.all([listGroups(), listCategories()]);
  allGroups.value = groups.map((g) => ({
    id: g.id,
    name: g.name,
    default_currency: g.default_currency,
    members: g.members,
  }));
  categories.value = cats;
  if (q.value.trim().length >= 2) await runSearch();
});
</script>

<template>
  <AppLayout :back="{ to: '/groups', label: 'Groups' }">
    <h1 class="title">Search</h1>

    <form class="search-form" @submit.prevent="onSubmit">
      <label class="field">
        <input
          v-model="q"
          type="search"
          minlength="2"
          maxlength="200"
          autocomplete="off"
          placeholder=" "
          class="field-input"
        />
        <span class="field-label">Search expense and settlement notes</span>
      </label>
      <div class="search-actions">
        <button type="submit" class="btn-primary">
          <Icon name="magnifying-glass" /><span>Search</span>
        </button>
      </div>

      <details :open="filtersOpen" class="filters">
        <summary class="filter-toggle">
          <Icon name="filter" :size="12" />
          <span>Filter</span>
        </summary>
        <div class="filter-row">
          <span class="filter-label">Filter:</span>
          <button type="button" class="chip" @click="groupDialog?.showModal()">
            <span class="muted">Group:</span>
            <span class="chip-val">{{ selectedGroupLabel }}</span>
            <Icon name="chevron-down" :size="10" />
          </button>
          <button type="button" class="chip" @click="categoryDialog?.showModal()">
            <span class="muted">Category:</span>
            <CategoryIcon
              v-if="selectedCategory"
              :slug="selectedCategory.slug"
              :group-label="selectedCategory.group_label"
              :size="14"
            />
            <span class="chip-val">{{ selectedCategoryLabel }}</span>
            <Icon name="chevron-down" :size="10" />
          </button>
        </div>
      </details>
    </form>

    <Alert v-if="queryError" tone="info" class="banner">{{ queryError }}</Alert>

    <p v-if="q.trim().length < 2" class="muted">
      Type at least 2 characters to search across all your groups.
    </p>
    <p v-else-if="items.length === 0 && !queryError" class="muted">
      No matches for <span class="strong">{{ usedQuery }}</span
      >.
    </p>

    <ul v-else class="results">
      <template v-for="(row, i) in rows" :key="i">
        <li v-if="row.kind === 'group-header'" class="group-header">
          <RouterLink :to="`/groups/${row.group.id}`" class="gh-name">{{ row.group.name }}</RouterLink>
          <span class="gh-count">{{ row.count }} {{ row.count === 1 ? "match" : "matches" }}</span>
        </li>
        <li v-else-if="row.item.expense">
          <RouterLink
            :to="`/groups/${row.item.expense.group_id}/expenses/${row.item.expense.id}`"
            class="hit"
          >
            <div class="hit-left">
              <span class="hit-date" :title="categoryByID.get(row.item.expense.category_id)?.label ?? ''">
                <CategoryIcon
                  :slug="categoryByID.get(row.item.expense.category_id)?.slug"
                  :group-label="categoryByID.get(row.item.expense.category_id)?.group_label"
                  :size="28"
                />
                <span class="hit-month">{{ monthFmt.format(new Date(row.item.expense.incurred_at)) }}</span>
                <span class="hit-day">{{ dayFmt.format(new Date(row.item.expense.incurred_at)) }}</span>
                <span class="hit-year">{{ yearFmt.format(new Date(row.item.expense.incurred_at)) }}</span>
              </span>
              <div class="hit-body">
                <div class="hit-desc">{{ row.item.expense.description }}</div>
                <div class="hit-sub">
                  <span>paid by</span>
                  <MemberAvatar
                    :user-id="row.item.expense.payer_id"
                    :display-name="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.expense.payer_id)?.display_name ?? '?' : '?'"
                    :has-avatar="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.expense.payer_id)?.has_avatar : false"
                    :avatar-updated-at="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.expense.payer_id)?.avatar_updated_at : null"
                    :size="12"
                  />
                  <span class="trunc">{{ shortName(groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.expense.payer_id)?.display_name : "?") }}</span>
                </div>
              </div>
            </div>
            <div class="hit-right">
              <span class="hit-stake">
                <template v-if="viewerStake(row.item.expense).kind === 'lent'">
                  <span class="stake-lent">you lent </span>
                  <span class="mono">{{ moneyFormatter(row.item.expense.currency).format((viewerStake(row.item.expense) as { cents: number }).cents / 100) }}</span>
                </template>
                <template v-else-if="viewerStake(row.item.expense).kind === 'owes'">
                  <span class="stake-owes">you owe </span>
                  <span class="mono">{{ moneyFormatter(row.item.expense.currency).format((viewerStake(row.item.expense) as { cents: number }).cents / 100) }}</span>
                </template>
                <span v-else class="muted">not involved</span>
              </span>
              <span class="hit-amount">{{ moneyFormatter(row.item.expense.currency).format(row.item.expense.amount_cents / 100) }}</span>
            </div>
          </RouterLink>
        </li>
        <li v-else-if="row.item.settlement">
          <RouterLink
            :to="`/groups/${row.item.settlement.group_id}/settlements/${row.item.settlement.id}`"
            class="hit hit-settlement"
          >
            <div class="hit-left">
              <span class="hit-date" title="Settlement">
                <span class="settle-icon"><Icon name="arrow-right" :size="14" /></span>
                <span class="hit-month">{{ monthFmt.format(new Date(row.item.settlement.settled_at)) }}</span>
                <span class="hit-day">{{ dayFmt.format(new Date(row.item.settlement.settled_at)) }}</span>
                <span class="hit-year">{{ yearFmt.format(new Date(row.item.settlement.settled_at)) }}</span>
              </span>
              <div class="hit-body">
                <div class="hit-settle-title">
                  <MemberAvatar
                    :user-id="row.item.settlement.from_user_id"
                    :display-name="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.from_user_id)?.display_name ?? '?' : '?'"
                    :has-avatar="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.from_user_id)?.has_avatar : false"
                    :avatar-updated-at="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.from_user_id)?.avatar_updated_at : null"
                    :size="16"
                  />
                  <span class="trunc">{{ shortName(groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.from_user_id)?.display_name : "?") }}</span>
                  <span class="muted">paid</span>
                  <MemberAvatar
                    :user-id="row.item.settlement.to_user_id"
                    :display-name="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.to_user_id)?.display_name ?? '?' : '?'"
                    :has-avatar="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.to_user_id)?.has_avatar : false"
                    :avatar-updated-at="groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.to_user_id)?.avatar_updated_at : null"
                    :size="16"
                  />
                  <span class="trunc">{{ shortName(groupForItem(row.item) ? memberByID(groupForItem(row.item)!).get(row.item.settlement.to_user_id)?.display_name : "?") }}</span>
                </div>
                <div class="hit-sub">
                  <span>settlement</span>
                  <span v-if="row.item.settlement.note" class="trunc">· {{ row.item.settlement.note }}</span>
                </div>
              </div>
            </div>
            <span class="hit-amount">{{ formatMoney(row.item.settlement.amount_cents, groupForItem(row.item)?.default_currency ?? "EUR") }}</span>
          </RouterLink>
        </li>
      </template>
    </ul>

    <!-- Group filter dialog -->
    <dialog ref="groupDialog" class="filter-dialog" aria-modal="true" aria-label="Filter by group">
      <div class="filter-dialog-body">
        <div class="filter-dialog-head">
          <h3 class="filter-dialog-title">Filter by group</h3>
          <button type="button" class="fd-close" aria-label="Close" @click="groupDialog?.close()">
            <Icon name="xmark" :size="14" />
          </button>
        </div>
        <ul class="fd-list">
          <li>
            <button type="button" class="field-category-option" @click="pickGroup('')">
              <Icon name="layer-group" :size="16" /><span>All groups</span>
            </button>
          </li>
          <li v-for="g in allGroups" :key="g.id">
            <button type="button" class="field-category-option" @click="pickGroup(g.id)">
              <Icon name="users" :size="16" /><span>{{ g.name }}</span>
            </button>
          </li>
        </ul>
      </div>
    </dialog>

    <!-- Category filter dialog -->
    <dialog ref="categoryDialog" class="filter-dialog" aria-modal="true" aria-label="Filter by category">
      <div class="filter-dialog-body">
        <div class="filter-dialog-head">
          <h3 class="filter-dialog-title">Filter by category</h3>
          <button type="button" class="fd-close" aria-label="Close" @click="categoryDialog?.close()">
            <Icon name="xmark" :size="14" />
          </button>
        </div>
        <ul class="fd-list">
          <li>
            <button type="button" class="field-category-option" @click="pickCategory('')">
              <Icon name="layer-group" :size="16" /><span>All categories</span>
            </button>
          </li>
          <template v-for="(c, i) in visibleCategories" :key="c.id">
            <li v-if="isNewGroupRow(visibleCategories, i)" class="field-category-group">{{ c.group_label }}</li>
            <li>
              <button type="button" class="field-category-option" @click="pickCategory(c.id)">
                <CategoryIcon :slug="c.slug" :group-label="c.group_label" :size="28" />
                <span>{{ c.label }}</span>
              </button>
            </li>
          </template>
        </ul>
      </div>
    </dialog>
  </AppLayout>
</template>

<style scoped>
.title {
  margin-bottom: 1rem;
  font-size: 1.5rem;
  font-weight: 600;
}
.search-form {
  margin-bottom: 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.search-actions {
  display: flex;
  justify-content: flex-end;
}
.filters {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.filter-toggle {
  display: inline-flex;
  width: fit-content;
  cursor: pointer;
  align-items: center;
  gap: 0.375rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.375rem 0.75rem;
  font-size: 0.75rem;
  font-weight: 500;
  list-style: none;
}
.filter-toggle::-webkit-details-marker {
  display: none;
}
.filter-row {
  margin-top: 0.75rem;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}
.filter-label {
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-foreground);
}
.chip {
  display: inline-flex;
  cursor: pointer;
  align-items: center;
  gap: 0.375rem;
  border-radius: 9999px;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.25rem 0.75rem;
  font-size: 0.75rem;
  font-weight: 500;
}
.chip:hover {
  background: var(--muted);
}
.chip-val {
  max-width: 10rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.muted {
  color: var(--muted-foreground);
}
.strong {
  font-weight: 500;
}
.banner {
  margin-bottom: 1rem;
}
.results {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  list-style: none;
}
.group-header {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.75rem 0.25rem 0;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-foreground);
}
.gh-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.gh-name:hover {
  text-decoration: underline;
}
.gh-count {
  font-variant-numeric: tabular-nums;
  text-transform: none;
}
.hit {
  display: flex;
  align-items: stretch;
  justify-content: space-between;
  gap: 0.375rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border);
  background: var(--card);
  padding: 0.75rem 1rem;
}
.hit:hover {
  background: var(--muted);
}
:root[data-theme="dark"] .hit:hover,
:root[data-theme="high-contrast"] .hit:hover {
  background: var(--accent);
}
.hit-settlement {
  border-color: color-mix(in oklch, var(--primary) 40%, var(--border));
}
.hit-left {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.75rem;
}
.hit-date {
  flex-shrink: 0;
  display: inline-flex;
  width: 1.75rem;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  line-height: 1;
}
.settle-icon {
  display: inline-flex;
  height: 1.75rem;
  width: 1.75rem;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: color-mix(in oklch, var(--primary) 20%, var(--card));
  color: var(--primary);
}
.hit-month,
.hit-year {
  font-size: 8px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--muted-foreground);
}
.hit-day {
  font-size: 0.75rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--muted-foreground);
}
.hit-body {
  min-width: 0;
}
.hit-desc {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}
.hit-settle-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem;
  font-weight: 500;
}
.hit-sub {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem;
  font-size: 11px;
  color: var(--muted-foreground);
}
.trunc {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.hit-right {
  display: flex;
  flex-shrink: 0;
  flex-direction: column;
  align-items: flex-end;
  justify-content: space-between;
  gap: 0.25rem;
}
.hit-stake {
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}
.stake-lent {
  color: var(--primary);
}
.stake-owes {
  color: oklch(0.6 0.13 60);
}
.mono {
  font-family: var(--font-mono);
}
.hit-amount {
  align-self: center;
  flex-shrink: 0;
  font-family: var(--font-mono);
  font-size: 1.125rem;
  font-variant-numeric: tabular-nums;
}
.filter-dialog {
  position: fixed;
  inset: 0;
  margin: auto;
  width: calc(100% - 2rem);
  max-width: 24rem;
  border: 1px solid var(--border);
  border-radius: 0.375rem;
  background: var(--popover);
  color: var(--popover-foreground);
  padding: 0;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.35);
}
.filter-dialog::backdrop {
  background: rgba(20, 20, 20, 0.4);
}
.filter-dialog-body {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1.25rem;
}
.filter-dialog-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}
.filter-dialog-title {
  font-size: 1.125rem;
  font-weight: 500;
}
.fd-close {
  border-radius: 0.375rem;
  padding: 0.25rem 0.5rem;
  color: var(--muted-foreground);
  cursor: pointer;
}
.fd-list {
  display: flex;
  max-height: 60vh;
  flex-direction: column;
  gap: 0.125rem;
  overflow: auto;
  list-style: none;
}
</style>
