import { describe, expect, it } from "vitest";
import { firstCsvCurrency, groupNameFromFilename, memberNamesFromCsv } from "./useImport";

describe("groupNameFromFilename", () => {
  it("strips the DoTheSplit export suffix and date", () => {
    expect(groupNameFromFilename("prost_2026-06-10_export.csv")).toBe("prost");
  });

  it("turns separators into spaces", () => {
    expect(groupNameFromFilename("trip_to_tokyo_2024-01-02_export.csv")).toBe("trip to tokyo");
  });

  it("handles an arbitrary Splitwise file name", () => {
    expect(groupNameFromFilename("my-splitwise-export.csv")).toBe("my splitwise export");
  });

  it("keeps a plain stem when there is no suffix", () => {
    expect(groupNameFromFilename("Groceries.csv")).toBe("Groceries");
  });
});

describe("memberNamesFromCsv", () => {
  it("returns the user columns of a plain Splitwise header", () => {
    const csv = "Date,Description,Category,Cost,Currency,Alice,Bob\n";
    expect(memberNamesFromCsv(csv)).toEqual(["Alice", "Bob"]);
  });

  it("skips the DoTheSplit optional metadata columns", () => {
    // The server strips Time/Payer/Notes/Created/CreatedBy before counting
    // members; the client must agree or the member count mismatches and the
    // import fails. This is the regression the import fix addresses.
    const csv =
      "Date,Description,Category,Cost,Currency,Time,Payer,Notes,Created,CreatedBy,Alice,Bob\n";
    expect(memberNamesFromCsv(csv)).toEqual(["Alice", "Bob"]);
  });

  it("matches the optional columns case-insensitively", () => {
    const csv = "Date,Description,Category,Cost,Currency,time,NOTES,Alice,Bob\n";
    expect(memberNamesFromCsv(csv)).toEqual(["Alice", "Bob"]);
  });

  it("treats a member literally named like a metadata column after the run ends", () => {
    // Only a *contiguous* leading run of optional columns is skipped; a member
    // column that happens to share a name is preserved once real members begin.
    const csv = "Date,Description,Category,Cost,Currency,Time,Alice,Notes\n";
    expect(memberNamesFromCsv(csv)).toEqual(["Alice", "Notes"]);
  });

  it("returns [] when the fixed prefix does not match", () => {
    expect(memberNamesFromCsv("Foo,Bar,Baz,Qux,Quux,Alice,Bob\n")).toEqual([]);
  });

  it("returns [] when there are fewer than two member columns", () => {
    expect(memberNamesFromCsv("Date,Description,Category,Cost,Currency,Alice\n")).toEqual([]);
  });
});

describe("firstCsvCurrency", () => {
  it("returns the first valid ISO code from the Currency column", () => {
    const csv = "Date,Description,Category,Cost,Currency,Alice,Bob\n2024-01-01,Lunch,Food,10,USD,10,-10\n";
    expect(firstCsvCurrency(csv)).toBe("USD");
  });

  it("defaults to EUR when no valid code is present", () => {
    expect(firstCsvCurrency("Date,Description,Category,Cost,Currency,Alice,Bob\n")).toBe("EUR");
  });
});
