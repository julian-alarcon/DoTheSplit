// Shared glue for the three two-phase CSV importer pages: the standalone
// Splitwise and dothesplit flows (which create a brand-new group and are
// byte-identical apart from their preview endpoint) and the per-group
// expense importer (a trimmed flow that targets an existing group). Phase 1
// uploads the raw CSV to an SSR preview endpoint; phase 2 is a regular HTML
// form post. The Go service is the source of truth - these helpers only
// transport the file and render the dry-run preview the server returns.

import { moneyFormatter } from "@/lib/currencies";

export const MAX_BYTES = 256 * 1024;

export function el<T extends HTMLElement>(id: string): T | null {
  return document.getElementById(id) as T | null;
}

// Validates the picked file (presence, size, .csv extension, non-empty) and
// returns its text, or null after rendering the failure via showError.
export async function readCsvFile(
  file: File | undefined,
  showError: (msg: string) => void,
): Promise<string | null> {
  if (!file) return null;
  if (file.size > MAX_BYTES) {
    showError(`File is ${(file.size / 1024).toFixed(1)} KiB but the limit is 256 KiB.`);
    return null;
  }
  if (!/\.csv$/i.test(file.name)) {
    showError("File must have a .csv extension.");
    return null;
  }
  const raw = await file.text();
  if (!raw.trim()) {
    showError("File is empty.");
    return null;
  }
  return raw;
}

export function makeShowError(target: HTMLElement | null) {
  return (msg: string) => {
    if (!target) return;
    target.textContent = msg;
    target.classList.remove("hidden");
  };
}

export function showReviewPhase(
  phasePick: HTMLElement | null,
  phaseReview: HTMLElement | null,
) {
  if (phasePick) phasePick.classList.add("hidden");
  if (phaseReview) {
    phaseReview.classList.remove("hidden");
    phaseReview.classList.add("flex");
  }
}

export function bindBackButton(
  backBtn: HTMLButtonElement,
  phasePick: HTMLElement,
  phaseReview: HTMLElement,
  fileInput: HTMLInputElement,
) {
  backBtn.addEventListener("click", () => {
    phaseReview.classList.add("hidden");
    phaseReview.classList.remove("flex");
    phasePick.classList.remove("hidden");
    fileInput.value = "";
  });
}

export function renderCounts(
  counts: HTMLElement,
  expenseCount: number,
  skippedCount: number,
) {
  counts.textContent =
    `${expenseCount} expense${expenseCount === 1 ? "" : "s"}` +
    (skippedCount > 0 ? ` · ${skippedCount} skipped` : "");
}

export function renderCurrencyMixed(
  warning: HTMLElement | null,
  list: HTMLElement | null,
  currencies: string[],
) {
  if (!warning || !list) return;
  if (currencies.length > 1) {
    list.textContent = currencies.join(", ");
    warning.classList.remove("hidden");
  } else {
    warning.classList.add("hidden");
    list.textContent = "";
  }
}

const ROW_CLASS =
  "flex items-center justify-between gap-3 rounded-md border border-neutral-200 bg-white px-3 py-2 dark:border-neutral-800 dark:bg-neutral-900";

// Renders a description/meta/amount row used by both the expense preview and
// the settlement preview. `meta` is the secondary line under the title.
export function appendPreviewRow(
  listEl: HTMLElement,
  title: string,
  meta: string,
  amount: string,
) {
  const li = document.createElement("li");
  li.className = ROW_CLASS;
  const left = document.createElement("div");
  left.className = "min-w-0 flex flex-col";
  const desc = document.createElement("span");
  desc.className = "truncate font-medium";
  desc.textContent = title;
  const metaEl = document.createElement("span");
  metaEl.className = "truncate text-xs text-neutral-600 dark:text-neutral-400";
  metaEl.textContent = meta;
  left.appendChild(desc);
  left.appendChild(metaEl);
  const amountEl = document.createElement("span");
  amountEl.className = "shrink-0 font-mono tabular-nums";
  amountEl.textContent = amount;
  li.appendChild(left);
  li.appendChild(amountEl);
  listEl.appendChild(li);
}

type ExpenseRow = {
  description: string;
  incurred_at: string;
  amount_cents: number;
  category_slug: string;
};

// Renders the expense preview list plus its "+ N more not shown" footer.
// `metaLine` builds the secondary line so callers can vary the payer field
// (csv name vs display name) without duplicating the row markup.
export function renderExpensePreview<T extends ExpenseRow>(
  listEl: HTMLElement,
  moreEl: HTMLElement,
  rows: T[],
  expenseCount: number,
  fmt: Intl.NumberFormat,
  metaLine: (row: T) => string,
) {
  listEl.innerHTML = "";
  for (const row of rows) {
    appendPreviewRow(
      listEl,
      row.description,
      metaLine(row),
      fmt.format(row.amount_cents / 100),
    );
  }
  renderMore(moreEl, expenseCount - rows.length, "expense");
}

export function renderMore(moreEl: HTMLElement, extra: number, noun: string) {
  if (extra > 0) {
    moreEl.textContent = `+ ${extra} more ${noun}${extra === 1 ? "" : "s"} not shown.`;
    moreEl.classList.remove("hidden");
  } else {
    moreEl.textContent = "";
    moreEl.classList.add("hidden");
  }
}

export function dateOnly(iso: string): string {
  return iso ? iso.slice(0, 10) : "";
}

type SkippedEls = {
  section: HTMLElement | null;
  counter: HTMLElement | null;
  list: HTMLElement | null;
  more: HTMLElement | null;
};

// Skipped rows: show the raw CSV lines the server couldn't import. The server
// caps the sample array, so display whatever it sent and note any overflow.
export function renderSkipped(
  els: SkippedEls,
  skipped: string[],
  skippedCount: number,
) {
  const { section, counter, list, more } = els;
  if (!section || !counter || !list || !more) return;
  if (skippedCount > 0 && skipped.length > 0) {
    counter.textContent = `(${skippedCount})`;
    list.textContent = skipped.join("\n");
    renderMore(more, skippedCount - skipped.length, "skipped row");
    section.classList.remove("hidden");
  } else {
    section.classList.add("hidden");
    list.textContent = "";
    counter.textContent = "";
    more.textContent = "";
    more.classList.add("hidden");
  }
}

// ---- Full importer (Splitwise / dothesplit) --------------------------------

type Balance = { csv_name: string; net_cents: number };

type SettlementPreview = {
  note: string;
  settled_at: string;
  amount_cents: number;
  currency: string;
  from_csv_name: string;
  to_csv_name: string;
};

type FullPreviewRow = ExpenseRow & { payer_csv_name: string };

type FullPreviewResponse = {
  group_name: string;
  default_currency: string;
  members: { csv_name: string; email: string }[];
  expense_count: number;
  settlement_count: number;
  skipped_count: number;
  skipped: string[];
  balances: Balance[];
  preview: FullPreviewRow[];
  settlement_preview: SettlementPreview[];
  csv_currencies: string[];
};

// "prost_2026-06-06_export.csv" → "Prost"
// "trip-to-tokio_2026-06-06_export.csv" → "Trip to tokio"
function guessGroupName(filename: string): string {
  let stem = filename.replace(/\.csv$/i, "");
  stem = stem.replace(/_\d{4}-\d{2}-\d{2}_export$/i, "");
  stem = stem.replace(/[_-]+/g, " ").trim();
  if (!stem) return "Imported group";
  return stem.charAt(0).toUpperCase() + stem.slice(1);
}

// Wires up the standalone importer flow. The Splitwise and dothesplit pages
// share identical DOM and behavior; only `previewEndpoint` differs.
export function setupFullImporter(previewEndpoint: string) {
  const phasePick = el<HTMLElement>("phase-pick");
  const phaseReview = el<HTMLElement>("phase-review");
  const pickForm = el<HTMLFormElement>("csv-pick-form");
  const fileInput = el<HTMLInputElement>("csv-file");
  const pickError = el<HTMLElement>("csv-pick-error");
  const csvText = el<HTMLInputElement>("csv-text");
  const groupName = el<HTMLInputElement>("group-name");
  const defaultCurrency = el<HTMLSelectElement>("default-currency");
  const memberFields = el<HTMLElement>("member-fields");
  const memberCount = el<HTMLInputElement>("member-count");
  const balancesList = el<HTMLElement>("balances-list");
  const previewList = el<HTMLElement>("preview-list");
  const previewMore = el<HTMLElement>("preview-more");
  const currencyMixedWarning = el<HTMLElement>("currency-mixed-warning");
  const currencyMixedList = el<HTMLElement>("currency-mixed-list");
  const counts = el<HTMLElement>("counts");
  const backBtn = el<HTMLButtonElement>("phase-back");
  const settlementSection = el<HTMLElement>("settlement-section");
  const settlementCounts = el<HTMLElement>("settlement-counts");
  const settlementList = el<HTMLElement>("settlement-list");
  const settlementMore = el<HTMLElement>("settlement-more");
  const skipped: SkippedEls = {
    section: el<HTMLElement>("skipped-section"),
    counter: el<HTMLElement>("skipped-counter"),
    list: el<HTMLElement>("skipped-list"),
    more: el<HTMLElement>("skipped-more"),
  };

  const showError = makeShowError(pickError);

  if (
    !(phasePick && phaseReview && pickForm && fileInput && pickError && csvText &&
      groupName && defaultCurrency && memberFields && memberCount && previewList &&
      counts && backBtn && previewMore)
  ) {
    return;
  }

  function renderMemberInputs(members: { csv_name: string }[]) {
    if (!memberFields) return;
    memberFields.innerHTML = "";
    for (let i = 0; i < members.length; i++) {
      const csvName = members[i].csv_name;

      const wrapper = document.createElement("label");
      wrapper.className = "field";

      const input = document.createElement("input");
      input.type = "email";
      input.name = `member_${i}_email`;
      input.id = `member-${i}-email`;
      input.required = true;
      input.placeholder = " ";
      input.className = "field-input";

      const label = document.createElement("span");
      label.className = "field-label";
      label.setAttribute("data-required", "");
      label.appendChild(document.createTextNode("Email for "));
      const nameSpan = document.createElement("span");
      nameSpan.className = "font-mono";
      nameSpan.textContent = csvName;
      label.appendChild(nameSpan);

      wrapper.appendChild(input);
      wrapper.appendChild(label);

      const hidden = document.createElement("input");
      hidden.type = "hidden";
      hidden.name = `member_${i}_name`;
      hidden.value = csvName;

      memberFields.appendChild(wrapper);
      memberFields.appendChild(hidden);
    }
  }

  function renderBalances(balances: Balance[], currency: string) {
    if (!balancesList) return;
    balancesList.innerHTML = "";
    const fmt = moneyFormatter(currency);
    for (const b of balances) {
      const li = document.createElement("li");
      li.className = "flex items-center justify-between gap-2";

      const name = document.createElement("span");
      name.className = "truncate";
      name.textContent = b.csv_name;

      const amount = document.createElement("span");
      // Preview convention matches /v1/groups/{id}/balances: positive = the
      // member is owed money (green), negative = the member owes (red).
      const positive = b.net_cents >= 0;
      amount.className = `shrink-0 font-mono ${positive ? "text-emerald-700 dark:text-emerald-400" : "text-red-700 dark:text-red-400"}`;
      amount.textContent = fmt.format(b.net_cents / 100);

      li.appendChild(name);
      li.appendChild(amount);
      balancesList.appendChild(li);
    }
  }

  function renderSettlements(settlements: SettlementPreview[], total: number) {
    if (!settlementSection || !settlementCounts || !settlementList || !settlementMore) return;
    if (total === 0) {
      settlementSection.classList.add("hidden");
      settlementList.innerHTML = "";
      settlementCounts.textContent = "";
      settlementMore.textContent = "";
      settlementMore.classList.add("hidden");
      return;
    }
    settlementSection.classList.remove("hidden");
    settlementCounts.textContent = `${total} settlement${total === 1 ? "" : "s"}`;
    settlementList.innerHTML = "";
    for (const st of settlements) {
      const fmt = moneyFormatter(st.currency);
      const arrow = `${st.from_csv_name} → ${st.to_csv_name}`;
      appendPreviewRow(
        settlementList,
        st.note || arrow,
        `${dateOnly(st.settled_at)} · ${arrow}`,
        fmt.format(st.amount_cents / 100),
      );
    }
    renderMore(settlementMore, total - settlements.length, "settlement");
  }

  function renderPhase2(raw: string, guessedName: string, data: FullPreviewResponse) {
    if (!csvText || !groupName || !defaultCurrency || !previewList || !counts || !previewMore || !memberCount) return;

    csvText.value = raw;
    groupName.value = data.group_name && data.group_name !== "Imported group" ? data.group_name : guessedName;

    if (data.default_currency) {
      const opt = Array.from(defaultCurrency.options).find((o) => o.value === data.default_currency);
      if (opt) defaultCurrency.value = data.default_currency;
    }

    // Mixed-currency CSVs are not first-class: dothesplit groups are
    // single-currency, so we surface a warning explaining that amounts will
    // be stored under whichever currency the user picks.
    renderCurrencyMixed(currencyMixedWarning, currencyMixedList, data.csv_currencies ?? []);

    renderMemberInputs(data.members);
    memberCount.value = String(data.members.length);

    const currency = data.default_currency || "EUR";
    renderBalances(data.balances ?? [], currency);
    renderSettlements(data.settlement_preview ?? [], data.settlement_count ?? 0);

    renderCounts(counts, data.expense_count, data.skipped_count);
    renderExpensePreview(
      previewList,
      previewMore,
      data.preview,
      data.expense_count,
      moneyFormatter(currency),
      (row) => `${dateOnly(row.incurred_at)} · paid by ${row.payer_csv_name} · ${row.category_slug}`,
    );
    renderSkipped(skipped, data.skipped ?? [], data.skipped_count);

    showReviewPhase(phasePick, phaseReview);
  }

  pickForm.addEventListener("submit", async (ev) => {
    ev.preventDefault();
    pickError.classList.add("hidden");
    pickError.textContent = "";

    const raw = await readCsvFile(fileInput.files?.[0], showError);
    if (raw === null) return;

    const guessedName = guessGroupName(fileInput.files?.[0]?.name ?? "");

    const res = await fetch(previewEndpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ csv: raw, group_name_hint: guessedName }),
    });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      showError(text || `Server returned ${res.status}.`);
      return;
    }
    const data = (await res.json()) as FullPreviewResponse;
    renderPhase2(raw, guessedName, data);
  });

  bindBackButton(backBtn, phasePick, phaseReview, fileInput);
}
