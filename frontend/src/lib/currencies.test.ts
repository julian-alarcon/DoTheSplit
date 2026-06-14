import { describe, expect, it } from "vitest";
import { COMMON_CURRENCIES, currencyFlag, formatMoney, moneyFormatter } from "./currencies";

describe("currencies", () => {
  it("leads the common list with EUR (the app default)", () => {
    expect(COMMON_CURRENCIES[0]).toBe("EUR");
  });

  it("formats integer cents with the narrow symbol", () => {
    // narrowSymbol is load-bearing (CLAUDE.md): "$5.00", not "US$5.00".
    const out = formatMoney(500, "USD");
    expect(out).toContain("5");
    expect(out).not.toContain("US$");
  });

  it("defaults an empty currency code to EUR", () => {
    const fmt = moneyFormatter("");
    expect(fmt.resolvedOptions().currency).toBe("EUR");
  });

  it("maps EUR and ILS to their override flags", () => {
    expect(currencyFlag("EUR")).toBe("🇪🇺");
    expect(currencyFlag("ILS")).toBe("🇵🇸");
  });

  it("derives a flag from the first two letters for national currencies", () => {
    expect(currencyFlag("USD")).toBe("🇺🇸");
    expect(currencyFlag("GBP")).toBe("🇬🇧");
  });
});
