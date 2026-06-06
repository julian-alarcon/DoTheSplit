// Two-phase Splitwise importer page glue.
//
// Phase 1: read the picked CSV, validate basic shape on the client, post the
// raw text to /api/import-splitwise-preview. The Go service is the only place
// the file is parsed; we just transport it. The preview response carries the
// member list (one entry per CSV user column), so this script renders an
// email input for each member dynamically.
//
// Phase 2: a regular HTML form posting to /api/import-splitwise. The CSV
// text travels in a hidden input so the commit doesn't re-upload the file.

// Force module scope so `const` declarations don't collide with the
// sibling import-dothesplit.ts during type-checking.
export {};

const MAX_BYTES = 256 * 1024;

type PreviewRow = {
  description: string;
  incurred_at: string;
  amount_cents: number;
  currency: string;
  category_slug: string;
  payer_csv_name: string;
};

type Balance = {
  csv_name: string;
  net_cents: number;
};

type SettlementPreview = {
  note: string;
  settled_at: string;
  amount_cents: number;
  currency: string;
  from_csv_name: string;
  to_csv_name: string;
};

type PreviewResponse = {
  group_name: string;
  default_currency: string;
  members: { csv_name: string; email: string }[];
  expense_count: number;
  settlement_count: number;
  skipped_count: number;
  skipped: string[];
  balances: Balance[];
  preview: PreviewRow[];
  settlement_preview: SettlementPreview[];
  csv_currencies: string[];
};

const phasePick = document.getElementById("phase-pick") as HTMLElement | null;
const phaseReview = document.getElementById("phase-review") as HTMLElement | null;
const pickForm = document.getElementById("csv-pick-form") as HTMLFormElement | null;
const fileInput = document.getElementById("csv-file") as HTMLInputElement | null;
const pickError = document.getElementById("csv-pick-error") as HTMLElement | null;
const csvText = document.getElementById("csv-text") as HTMLInputElement | null;
const groupName = document.getElementById("group-name") as HTMLInputElement | null;
const defaultCurrency = document.getElementById("default-currency") as HTMLSelectElement | null;
const memberFields = document.getElementById("member-fields") as HTMLElement | null;
const memberCount = document.getElementById("member-count") as HTMLInputElement | null;
const balancesList = document.getElementById("balances-list") as HTMLElement | null;
const previewList = document.getElementById("preview-list") as HTMLElement | null;
const previewMore = document.getElementById("preview-more") as HTMLElement | null;
const skippedSection = document.getElementById("skipped-section") as HTMLElement | null;
const skippedCounter = document.getElementById("skipped-counter") as HTMLElement | null;
const skippedList = document.getElementById("skipped-list") as HTMLElement | null;
const skippedMore = document.getElementById("skipped-more") as HTMLElement | null;
const settlementSection = document.getElementById("settlement-section") as HTMLElement | null;
const settlementCounts = document.getElementById("settlement-counts") as HTMLElement | null;
const settlementList = document.getElementById("settlement-list") as HTMLElement | null;
const settlementMore = document.getElementById("settlement-more") as HTMLElement | null;
const currencyMixedWarning = document.getElementById("currency-mixed-warning") as HTMLElement | null;
const currencyMixedList = document.getElementById("currency-mixed-list") as HTMLElement | null;
const counts = document.getElementById("counts") as HTMLElement | null;
const backBtn = document.getElementById("phase-back") as HTMLButtonElement | null;

if (
  phasePick && phaseReview && pickForm && fileInput && pickError && csvText &&
  groupName && defaultCurrency && memberFields && memberCount && previewList &&
  counts && backBtn && previewMore
) {
  pickForm.addEventListener("submit", async (ev) => {
    ev.preventDefault();
    pickError.classList.add("hidden");
    pickError.textContent = "";

    const file = fileInput.files?.[0];
    if (!file) return;
    if (file.size > MAX_BYTES) {
      showError(`File is ${(file.size / 1024).toFixed(1)} KiB but the limit is 256 KiB.`);
      return;
    }
    if (!/\.csv$/i.test(file.name)) {
      showError("File must have a .csv extension.");
      return;
    }
    const raw = await file.text();
    if (!raw.trim()) {
      showError("File is empty.");
      return;
    }

    const guessedName = guessGroupName(file.name);

    const res = await fetch("/api/import-splitwise-preview", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ csv: raw, group_name_hint: guessedName }),
    });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      showError(text || `Server returned ${res.status}.`);
      return;
    }
    const data = (await res.json()) as PreviewResponse;
    renderPhase2(raw, guessedName, data);
  });

  backBtn.addEventListener("click", () => {
    phaseReview.classList.add("hidden");
    phaseReview.classList.remove("flex");
    phasePick.classList.remove("hidden");
    fileInput.value = "";
  });
}

function showError(msg: string) {
  if (!pickError) return;
  pickError.textContent = msg;
  pickError.classList.remove("hidden");
}

// "prost_2026-06-06_export.csv" → "Prost"
// "trip-to-tokio_2026-06-06_export.csv" → "Trip to tokio"
function guessGroupName(filename: string): string {
  let stem = filename.replace(/\.csv$/i, "");
  stem = stem.replace(/_\d{4}-\d{2}-\d{2}_export$/i, "");
  stem = stem.replace(/[_-]+/g, " ").trim();
  if (!stem) return "Imported group";
  return stem.charAt(0).toUpperCase() + stem.slice(1);
}

function renderBalances(balances: Balance[], currency: string) {
  if (!balancesList) return;
  balancesList.innerHTML = "";
  const fmt = new Intl.NumberFormat(undefined, {
    style: "currency",
    currency,
    currencyDisplay: "narrowSymbol",
  });
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
    const fmt = new Intl.NumberFormat(undefined, {
      style: "currency",
      currency: st.currency || "EUR",
      currencyDisplay: "narrowSymbol",
    });
    const li = document.createElement("li");
    li.className = "flex items-center justify-between gap-3 rounded-md border border-neutral-200 bg-white px-3 py-2 dark:border-neutral-800 dark:bg-neutral-900";
    const left = document.createElement("div");
    left.className = "min-w-0 flex flex-col";
    const desc = document.createElement("span");
    desc.className = "truncate font-medium";
    desc.textContent = st.note || `${st.from_csv_name} → ${st.to_csv_name}`;
    const meta = document.createElement("span");
    meta.className = "truncate text-xs text-neutral-600 dark:text-neutral-400";
    const date = st.settled_at ? st.settled_at.slice(0, 10) : "";
    meta.textContent = `${date} · ${st.from_csv_name} → ${st.to_csv_name}`;
    left.appendChild(desc);
    left.appendChild(meta);
    const amount = document.createElement("span");
    amount.className = "shrink-0 font-mono tabular-nums";
    amount.textContent = fmt.format(st.amount_cents / 100);
    li.appendChild(left);
    li.appendChild(amount);
    settlementList.appendChild(li);
  }
  const extra = total - settlements.length;
  if (extra > 0) {
    settlementMore.textContent = `+ ${extra} more settlement${extra === 1 ? "" : "s"} not shown.`;
    settlementMore.classList.remove("hidden");
  } else {
    settlementMore.textContent = "";
    settlementMore.classList.add("hidden");
  }
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
    const labelText = document.createTextNode("Email for ");
    const nameSpan = document.createElement("span");
    nameSpan.className = "font-mono";
    nameSpan.textContent = csvName;
    label.appendChild(labelText);
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

function renderPhase2(raw: string, guessedName: string, data: PreviewResponse) {
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
  const csvCurrencies = data.csv_currencies ?? [];
  if (currencyMixedWarning && currencyMixedList) {
    if (csvCurrencies.length > 1) {
      currencyMixedList.textContent = csvCurrencies.join(", ");
      currencyMixedWarning.classList.remove("hidden");
    } else {
      currencyMixedWarning.classList.add("hidden");
      currencyMixedList.textContent = "";
    }
  }

  renderMemberInputs(data.members);
  memberCount.value = String(data.members.length);

  renderBalances(data.balances ?? [], data.default_currency || "EUR");
  renderSettlements(data.settlement_preview ?? [], data.settlement_count ?? 0);

  counts.textContent = `${data.expense_count} expense${data.expense_count === 1 ? "" : "s"}` +
    (data.skipped_count > 0 ? ` · ${data.skipped_count} skipped` : "");

  previewList.innerHTML = "";
  const fmt = new Intl.NumberFormat(undefined, {
    style: "currency",
    currency: data.default_currency || "EUR",
    currencyDisplay: "narrowSymbol",
  });
  for (const row of data.preview) {
    const li = document.createElement("li");
    li.className = "flex items-center justify-between gap-3 rounded-md border border-neutral-200 bg-white px-3 py-2 dark:border-neutral-800 dark:bg-neutral-900";
    const left = document.createElement("div");
    left.className = "min-w-0 flex flex-col";
    const desc = document.createElement("span");
    desc.className = "truncate font-medium";
    desc.textContent = row.description;
    const meta = document.createElement("span");
    meta.className = "truncate text-xs text-neutral-600 dark:text-neutral-400";
    const date = row.incurred_at ? row.incurred_at.slice(0, 10) : "";
    meta.textContent = `${date} · paid by ${row.payer_csv_name} · ${row.category_slug}`;
    left.appendChild(desc);
    left.appendChild(meta);
    const amount = document.createElement("span");
    amount.className = "shrink-0 font-mono tabular-nums";
    amount.textContent = fmt.format(row.amount_cents / 100);
    li.appendChild(left);
    li.appendChild(amount);
    previewList.appendChild(li);
  }
  const extra = data.expense_count - data.preview.length;
  if (extra > 0) {
    previewMore.textContent = `+ ${extra} more expense${extra === 1 ? "" : "s"} not shown.`;
    previewMore.classList.remove("hidden");
  } else {
    previewMore.classList.add("hidden");
    previewMore.textContent = "";
  }

  // Skipped rows: show the raw CSV lines so the user can see exactly what
  // didn't import. The server caps the array, so display whatever it sent
  // and note when there are more skips than samples.
  if (skippedSection && skippedCounter && skippedList && skippedMore) {
    const samples = data.skipped ?? [];
    if (data.skipped_count > 0 && samples.length > 0) {
      skippedCounter.textContent = `(${data.skipped_count})`;
      skippedList.textContent = samples.join("\n");
      const hidden = data.skipped_count - samples.length;
      if (hidden > 0) {
        skippedMore.textContent = `+ ${hidden} more skipped row${hidden === 1 ? "" : "s"} not shown.`;
        skippedMore.classList.remove("hidden");
      } else {
        skippedMore.textContent = "";
        skippedMore.classList.add("hidden");
      }
      skippedSection.classList.remove("hidden");
    } else {
      skippedSection.classList.add("hidden");
      skippedList.textContent = "";
      skippedCounter.textContent = "";
      skippedMore.textContent = "";
      skippedMore.classList.add("hidden");
    }
  }

  if (phasePick) phasePick.classList.add("hidden");
  if (phaseReview) {
    phaseReview.classList.remove("hidden");
    phaseReview.classList.add("flex");
  }
}
