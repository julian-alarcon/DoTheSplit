// Two-phase importer for the per-group expense CSV flow. Mirrors the
// dothesplit importer but trimmed: no member mapping, no group name,
// no currency picker - the group already pins all of that. Splits are
// derived server-side from the group's default rule.

export {};

const MAX_BYTES = 256 * 1024;

type PreviewRow = {
  description: string;
  incurred_at: string;
  amount_cents: number;
  currency: string;
  category_slug: string;
  payer_display_name: string;
};

type PreviewResponse = {
  expense_count: number;
  skipped_count: number;
  skipped: string[];
  preview: PreviewRow[];
  csv_currencies: string[];
};

const phasePick = document.getElementById("phase-pick") as HTMLElement | null;
const phaseReview = document.getElementById("phase-review") as HTMLElement | null;
const pickForm = document.getElementById("csv-pick-form") as HTMLFormElement | null;
const fileInput = document.getElementById("csv-file") as HTMLInputElement | null;
const pickError = document.getElementById("csv-pick-error") as HTMLElement | null;
const csvText = document.getElementById("csv-text") as HTMLInputElement | null;
const groupCurrency = document.getElementById("group-currency") as HTMLInputElement | null;
const previewList = document.getElementById("preview-list") as HTMLElement | null;
const previewMore = document.getElementById("preview-more") as HTMLElement | null;
const skippedSection = document.getElementById("skipped-section") as HTMLElement | null;
const skippedCounter = document.getElementById("skipped-counter") as HTMLElement | null;
const skippedList = document.getElementById("skipped-list") as HTMLElement | null;
const skippedMore = document.getElementById("skipped-more") as HTMLElement | null;
const currencyMixedWarning = document.getElementById("currency-mixed-warning") as HTMLElement | null;
const currencyMixedList = document.getElementById("currency-mixed-list") as HTMLElement | null;
const counts = document.getElementById("counts") as HTMLElement | null;
const backBtn = document.getElementById("phase-back") as HTMLButtonElement | null;

// Group id is encoded in the form's action (?id=<uuid>). Pull it out so
// the preview endpoint receives the same scope.
function groupIdFromAction(): string {
  const form = document.getElementById("csv-import-form") as HTMLFormElement | null;
  const action = form?.getAttribute("action") ?? "";
  const m = /[?&]id=([0-9a-fA-F-]{36})/.exec(action);
  return m ? m[1] : "";
}

if (
  phasePick && phaseReview && pickForm && fileInput && pickError && csvText &&
  previewList && counts && backBtn && previewMore && groupCurrency
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

    const id = groupIdFromAction();
    if (!id) {
      showError("Could not determine the group id.");
      return;
    }

    const res = await fetch(
      `/api/group-expenses-import-preview?id=${encodeURIComponent(id)}`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ csv: raw }),
      },
    );
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      showError(text || `Server returned ${res.status}.`);
      return;
    }
    const data = (await res.json()) as PreviewResponse;
    renderPhase2(raw, data);
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

function renderPhase2(raw: string, data: PreviewResponse) {
  if (!csvText || !previewList || !counts || !previewMore || !groupCurrency) return;

  csvText.value = raw;

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

  counts.textContent =
    `${data.expense_count} expense${data.expense_count === 1 ? "" : "s"}` +
    (data.skipped_count > 0 ? ` · ${data.skipped_count} skipped` : "");

  const fmt = new Intl.NumberFormat(undefined, {
    style: "currency",
    currency: groupCurrency.value || "EUR",
    currencyDisplay: "narrowSymbol",
  });

  previewList.innerHTML = "";
  for (const row of data.preview) {
    const li = document.createElement("li");
    li.className =
      "flex items-center justify-between gap-3 rounded-md border border-neutral-200 bg-white px-3 py-2 dark:border-neutral-800 dark:bg-neutral-900";
    const left = document.createElement("div");
    left.className = "min-w-0 flex flex-col";
    const desc = document.createElement("span");
    desc.className = "truncate font-medium";
    desc.textContent = row.description;
    const meta = document.createElement("span");
    meta.className = "truncate text-xs text-neutral-600 dark:text-neutral-400";
    const date = row.incurred_at ? row.incurred_at.slice(0, 10) : "";
    meta.textContent = `${date} · paid by ${row.payer_display_name} · ${row.category_slug}`;
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
