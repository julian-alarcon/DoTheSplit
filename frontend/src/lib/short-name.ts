// Compact a display name for tight UI rows (transaction lists, balances,
// search results). Names are usually "First Last": keep the first name and
// abbreviate the last to an initial ("Diana Yermanos" -> "Diana Y."). Use
// that form only when it fits under 10 chars; otherwise fall back to the
// first 9 chars of the first name plus an ellipsis ("Maximiliano" -> "Maximilia...").

export function shortName(name: string | undefined | null): string {
  const s = (name ?? "?").trim();
  if (!s) return "?";
  const parts = s.split(/\s+/);
  const first = parts[0];
  if (parts.length >= 2) {
    const candidate = `${first} ${parts[1][0]}.`;
    if (candidate.length < 10) return candidate;
  }
  return first.length > 9 ? first.slice(0, 9) + "..." : first;
}
