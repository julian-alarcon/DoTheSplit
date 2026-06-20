// Shared, cached date formatters. Several views render a "MMM / DD" day badge
// next to transactions; constructing Intl.DateTimeFormat per view (or worse,
// per render) re-parses locale data needlessly. `undefined` locale follows the
// runtime, matching moneyFormatter in currencies.ts.

export const monthShortFmt = new Intl.DateTimeFormat(undefined, {
  month: "short",
});

export const dayTwoDigitFmt = new Intl.DateTimeFormat(undefined, {
  day: "2-digit",
});
