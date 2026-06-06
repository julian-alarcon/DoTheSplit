# Importing and exporting a group

DoTheSplit can hydrate a brand-new group from a CSV export of another
expense-sharing service, and any group member can download a CSV of the
group's full ledger from settings. Two import sources ship today:

- **Splitwise** ([Splitwise section](#splitwise)): the source format the
  rest of this document was written against.
- **DoTheSplit** ([DoTheSplit section](#dothesplit)): the file produced
  by the export endpoint, a strict superset of the Splitwise format.

More importers (Tricount and friends) will land behind the same
`/import` page.

## Splitwise

### What you need

- A CSV export of a Splitwise group (any size from 2 up to 32 members). In
  Splitwise: open the group, pick **Export as spreadsheet**, save the
  resulting `.csv`.
- The members' display names exactly as they appear in the column headers of
  the exported file.
- An email per member. The email of the user running the import becomes a
  group member as usual; any email not already registered with DoTheSplit is
  mapped to a placeholder account that the real owner can claim later.

### Doing the import

1. Sign in and go to `/groups`.
2. Click **Import group** at the bottom of the page.
3. Pick **Splitwise**.
4. Upload the CSV. The page parses it server-side and shows you a summary:
   - the suggested group name (taken from the file name with the
     `_YYYY-MM-DD_export.csv` suffix stripped),
   - the dominant currency from the rows,
   - one row per member from the CSV header, with an email field next to each,
   - up to six expense rows previewed in the same style they will look like
     once imported.
5. Confirm the group name, currency, and an email per CSV name. Click
   **Import**. You land on the new group page with all expenses created.

### What the importer maps

| Splitwise column | DoTheSplit field                                         |
|------------------|----------------------------------------------------------|
| `Date`           | `incurred_at` (parsed as `YYYY-MM-DD`)                   |
| `Description`    | `description` (suffixed `[k/K]` for multi-payer rows)    |
| `Category`       | category, mapped to the closest seeded label (see below) |
| `Cost`           | `amount_cents`                                           |
| `Currency`       | per-expense currency                                     |
| `<UserN>`        | signed cents net balance change for user N               |

Splitwise stores per-user signed cents as a *net balance change* for the
row, matching the convention of the trailing `Total balance` footer: a
positive value means the user paid more than they owe (creditor, is owed
money); a negative value means they owe more than they paid (debtor).
Across all users the values sum to zero (within rounding).

### Single-payer vs. multi-payer rows

- **Single payer** (one positive value, others negative or zero): the row
  becomes one DoTheSplit expense at the row's full cost. The creditor is
  the payer; each debtor's share equals the absolute value of their CSV
  entry.

- **Multiple payers** (more than one positive value): the row is decomposed
  into one DoTheSplit expense per payer, each at the payer's positive
  balance. The description gets a `[k/K]` suffix
  (e.g. `Brunch [1/2]`, `Brunch [2/2]`) so the imported group is still
  browsable. The remainder of the row's cost (each payer's self-share) is
  dropped because DoTheSplit only supports one payer per expense; **net
  balances are preserved exactly** even with the dropped self-shares.

### Payments (settlements)

Splitwise rows whose **Category** is `Payment` are not expenses, they are
cash transfers between two members to settle a debt (e.g. *"Fernanda D.
paid Nathaly V."*). The importer recognises them and creates a DoTheSplit
**settlement** instead of an expense. Sign convention: the member with the
positive value is the payer (`from_user`, their owed-to-others balance went
up because they handed money over), the negative value is the recipient
(`to_user`). The `Description` becomes the settlement note and the row's
`Cost` is the settled amount.

A well-formed Payment row has exactly one positive and one negative entry
with equal magnitude that also matches `Cost`. Anything else (multi-party
payments, asymmetric magnitudes) is skipped, since DoTheSplit settlements
are strictly two-party.

### Currency

DoTheSplit groups use a **single currency**; multi-currency groups are not
supported and are not on the roadmap. The importer pre-selects the first
currency it sees in the CSV's `Currency` column as the new group's default,
and you can change it before clicking *Import*.

When the CSV mixes multiple currencies (which Splitwise allows), the
importer surfaces a warning. Every imported expense and settlement is
stored under the chosen group currency regardless of its original code,
so the numeric values will not be a faithful conversion of the source
file. If you need separate currencies, run separate imports into
separate groups.

### Categories we map

Categories are matched case-insensitively against the seeded DoTheSplit
labels, with a small alias table for Splitwise-specific names
(`TV/Phone/Internet` → `Internet`, `Bus/train` → `Train`, etc.). Anything
we can't map falls back to **Other**.

### Rows we skip

The importer skips rows silently and reports the count back in the preview:

- the trailing `Total balance` summary row,
- blank rows,
- rows where all per-user values are zero,
- rows where every user has the same sign (no creditor or no debtor),
- rows where the per-user values don't sum to zero (within N cents tolerance,
  where N is the number of members),
- rows with zero or negative cost,
- rows with an unparseable date.

### Limits

- File size: 256 KiB.
- Row count: 5000.
- Members: 2 to 32.
- Field length: 256 characters per cell.

These are hard caps enforced server-side; the page also pre-checks file size
and extension before uploading.

### Privacy: unknown emails

The import endpoint never tells you whether an email is registered. If you
type an address that has no DoTheSplit account, the import still succeeds: a
non-loginable placeholder account is created with the display name
`<CSV name> (imported)`, and the imported expenses point at it. When the
real owner registers later they can claim the placeholder (claim flow is a
future task tracked separately). This is a deliberate trade-off to keep the
importer from being used to enumerate the user table.

If you re-import a CSV that names the same email twice in different runs,
you will create two separate groups; the placeholder user is reused if it
already exists.

## Exporting a group

Open a group, go to **Settings**, and click **Export CSV** in the Export
block. The file is `<group-slug>_<YYYY-MM-DD>_export.csv`. Any member can
export.

The header is a strict superset of the Splitwise format:

```
Date,Description,Category,Cost,Currency,Time,Payer,Notes,Created,CreatedBy,<member1>,<member2>,...
```

The first 5 mandatory columns and the trailing per-member columns match
the Splitwise format exactly. The middle columns carry the data the
Splitwise format drops:

| Column      | What it is                                                                          |
|-------------|-------------------------------------------------------------------------------------|
| `Time`      | RFC 3339 time component of `incurred_at` / `settled_at` in UTC (`HH:MM:SSZ`).      |
| `Payer`     | Display name of the payer (or settlement sender). Bypasses sign-based inference.   |
| `Notes`     | Free-form `expense.notes` field. Empty for settlements.                            |
| `Created`   | RFC 3339 timestamp of when the row was first written in DoTheSplit.                |
| `CreatedBy` | Display name of the user who entered the row.                                       |

Per-row member columns hold each member's signed `paid - share` in the
group's default currency (positive = the member is owed for this row,
negative = the member owes). A `Total balance` footer mirrors the
balances endpoint.

Settlements appear as rows with `Category = Payment`; the from-user's
column gets the positive amount and the to-user's the negative.

The file carries display names, never emails. On re-import you map each
CSV name to an email, exactly like the Splitwise import flow.

## DoTheSplit

A DoTheSplit-flavored CSV (the one the export endpoint emits) is the
input to `/import/dothesplit`. The flow is identical to the Splitwise
importer (file picker → name/email mapping → preview → commit), and the
five-column Splitwise format is also accepted (the optional middle
columns degrade gracefully to "no time / no explicit payer / no notes").

What the dothesplit importer reads from the extra columns:

- `Time`: combined with `Date` to populate `incurred_at` /
  `settled_at` to the second. The Splitwise importer rounds to
  midnight; this one does not.
- `Payer`: the named payer wins over the sign-based inference. This
  matters for rows where multiple members have non-zero sign (the
  Splitwise inference picks the largest creditor; the explicit column
  is the source of truth for the original group state).
- `Notes`: round-trips into `expense.notes`. Empty for settlements.
- `Created` / `CreatedBy`: shown in the preview as provenance, not
  stored. The new group has fresh audit columns with the importing
  user as `created_by`.

The legacy Splitwise importer also accepts dothesplit-shaped files
(it skips the optional columns), so you can re-import an exported
DoTheSplit CSV through either route. The dothesplit importer is the
faithful one.

### Round-trip guarantees

Exporting a group, then re-importing the file via `/import/dothesplit`,
produces a fresh group whose:

- balances match the source group's balances exactly,
- expenses preserve description, amount, payer, notes, and `incurred_at`
  to the second,
- settlements preserve from / to / amount / note / `settled_at`.

The new group has fresh `created_at` / `created_by` per row (the
importing user is the new creator), and member emails come from the
mapping you supplied at commit time, not from the file.
