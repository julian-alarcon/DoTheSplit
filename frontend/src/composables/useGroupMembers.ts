// Memoized lookup maps for a group's members. The same memberByID/nameByID
// pair was hand-rolled in several views (dashboard, settle, expense detail,
// activity); this keeps one definition and lets callers pass the existing
// `members` computed directly via MaybeRefOrGetter.
import { computed, toValue, type MaybeRefOrGetter } from "vue";
import type { components } from "@/lib/api/schema";

type GroupMember = components["schemas"]["GroupMember"];

export function useGroupMembers(members: MaybeRefOrGetter<GroupMember[]>) {
  const memberByID = computed(
    () => new Map(toValue(members).map((m) => [m.user_id, m])),
  );
  const nameByID = computed(
    () => new Map(toValue(members).map((m) => [m.user_id, m.display_name])),
  );
  const nameOf = (id: string) => nameByID.value.get(id) ?? "?";
  return { memberByID, nameByID, nameOf };
}
