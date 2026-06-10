// Two-phase importer for the per-group expense CSV flow. Trimmed version of
// the standalone importer: no member mapping, no group name, no currency
// picker - the group already pins all of that. Splits are derived server-side
// from the group's default rule. Shared rendering lives in import-csv-common.
import {
  bindBackButton,
  dateOnly,
  el,
  makeShowError,
  moneyFormatter,
  readCsvFile,
  renderCounts,
  renderCurrencyMixed,
  renderExpensePreview,
  renderSkipped,
  showReviewPhase,
} from "./import-csv-common";

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

const phasePick = el<HTMLElement>("phase-pick");
const phaseReview = el<HTMLElement>("phase-review");
const pickForm = el<HTMLFormElement>("csv-pick-form");
const fileInput = el<HTMLInputElement>("csv-file");
const pickError = el<HTMLElement>("csv-pick-error");
const csvText = el<HTMLInputElement>("csv-text");
const groupCurrency = el<HTMLInputElement>("group-currency");
const previewList = el<HTMLElement>("preview-list");
const previewMore = el<HTMLElement>("preview-more");
const currencyMixedWarning = el<HTMLElement>("currency-mixed-warning");
const currencyMixedList = el<HTMLElement>("currency-mixed-list");
const counts = el<HTMLElement>("counts");
const backBtn = el<HTMLButtonElement>("phase-back");
const skipped = {
  section: el<HTMLElement>("skipped-section"),
  counter: el<HTMLElement>("skipped-counter"),
  list: el<HTMLElement>("skipped-list"),
  more: el<HTMLElement>("skipped-more"),
};

const showError = makeShowError(pickError);

// Group id is encoded in the form's action (?id=<uuid>). Pull it out so
// the preview endpoint receives the same scope.
function groupIdFromAction(): string {
  const form = el<HTMLFormElement>("csv-import-form");
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

    const raw = await readCsvFile(fileInput.files?.[0], showError);
    if (raw === null) return;

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

  bindBackButton(backBtn, phasePick, phaseReview, fileInput);
}

function renderPhase2(raw: string, data: PreviewResponse) {
  if (!csvText || !previewList || !counts || !previewMore || !groupCurrency) return;

  csvText.value = raw;

  renderCurrencyMixed(currencyMixedWarning, currencyMixedList, data.csv_currencies ?? []);
  renderCounts(counts, data.expense_count, data.skipped_count);
  renderExpensePreview(
    previewList,
    previewMore,
    data.preview,
    data.expense_count,
    moneyFormatter(groupCurrency.value),
    (row) => `${dateOnly(row.incurred_at)} · paid by ${row.payer_display_name} · ${row.category_slug}`,
  );
  renderSkipped(skipped, data.skipped ?? [], data.skipped_count);

  showReviewPhase(phasePick, phaseReview);
}
