# Design: Phonebook-only Contacts

Status: draft, for discussion.

## Goal

Keep one CardDAV address book for phone and Thunderbird, but hide some contacts from the CRM:
businesses, hotlines, and people you only need a number for. You must still be able to edit them in
Meerkat.

## Current state

- **Archive** hides a contact from default search, the dashboard, the graph, birthdays and the ICS
  feed. Archived contacts still sync over CardDAV, and you can still edit them (turn on "Show
  archived" in the contact list).
- Archiving deletes the active reminders. The meaning is "people I no longer follow".
- The archive filter is repeated in `contact_controller.go`, `graph_controller.go`,
  `birthday_service.go`, `ics_service.go`. There is no shared query scope.
- **Circles** are a JSON string list on the contact (`contacts.circles`). There is no circle table.
  Circles map to vCard `CATEGORIES` in both directions, so phone groups become circles.
- There is no `KIND` field. A card with only `ORG` gets the company name as first name
  (`fallbackFirstname`).
- There is no bulk selection in the contact list.

## Problem

1. "Archived" and "phonebook only" are different things. With one flag, you cannot tell "old friend"
   from "pizza place". An old friend can come back; a pizza place never becomes CRM data.
2. You must classify each new card from the phone by hand.
3. You can change only one contact at a time.

## Alternatives

### A. Archive only, plus bulk archive

Keep the meaning of archive as "hidden from the CRM". Add bulk selection and "Archive selected".

- \+ Smallest change. No new concept.
- − Problem 1 stays.
- − Archiving deletes reminders; that is wrong for a person you still want a yearly reminder for.

### B. Hidden circles — recommended

A user setting lists circles that are hidden, for example `["Phonebook"]`. Store it like
`User.CustomFieldNames`. A contact in a hidden circle is treated like an archived contact in CRM
views, but keeps its reminders.

Hidden from: default list and search, dashboard, birthdays, ICS feed, graph, relationship and
activity pickers. Visible: the contact list with a "Show hidden" switch (merge with "Show
archived"), the detail page, edits, CardDAV sync.

- \+ Works from the phone and from Thunderbird: put the card in the group "Phonebook", and Meerkat
  hides it after the next sync. No Meerkat-only state.
- \+ Archive keeps its own meaning. A contact can be both.
- \+ Bulk work reuses circles: bulk "add to circle", and filter by circle.
- − Circles have no identity. If you rename the group on the phone, the setting no longer matches.
  Mitigation: settings page shows hidden circles that no contact uses.
- − Every CRM query must also check the hidden circles (`json_each`). Do this together with one
  shared scope for "visible in CRM" (archived OR in a hidden circle). That also removes the repeated
  archive filter.
- − The group "Phonebook" shows on the phone. For most users this is a benefit.

### C. A dedicated flag `phonebook_only`

Same behavior as B, but a boolean column.

- \+ Simple, fast queries. No rename problem.
- − Not visible on the phone, so you cannot set it there. A vCard round trip needs a custom property
  (`X-MEERKAT-HIDDEN`), which most clients drop or do not show.
- − A third state next to archive and circles.

### D. Contact kind from vCard `KIND`

Add `kind` (`individual`, `org`). Organizations get a simple view and are excluded from features for
people (birthdays, graph, stay in touch). Fixes `fallbackFirstname` for company cards.

- \+ Correct data model; `KIND` is standard vCard 4.
- − It answers "what is it", not "do I want to see it". A person you only need in the phonebook
  still needs B or C.
- − Most phone apps do not write `KIND`, so detection needs a guess (`ORG` without `N`/`FN` person
  name).

Good as a later addition to B, not a replacement.

## Automatic classification

Rules run when a card is created by CardDAV sync, the built-in CardDAV server, or vCard import. They
never change existing contacts.

| Rule                                                       | Action                                                                 |
| ---------------------------------------------------------- | ---------------------------------------------------------------------- |
| `KIND:org`, or `ORG` with no given name and no family name | add circle "Phonebook" (B)                                             |
| New card from sync (optional, off by default)              | add circle "Phonebook"; you then move the people you want into the CRM |

The second rule fits users who keep many contacts on the phone and want only a few in the CRM. It
inverts the default: hidden until promoted.

- Show the rules as settings. Default: rule 1 on, rule 2 off.
- Rules add a circle, so you can see and undo the result everywhere.

## Bulk actions

Add multi-select to the contact list (check boxes, "select all on this page" and "select all
matching the filter"). Actions:

- Add to circle / remove from circle
- Archive / unarchive
- Delete (to trash)

API: `POST /contacts/bulk` with `{ids | filter, action, circle?}`. One transaction with a maximum
batch size. The filter variant avoids sending thousands of IDs.

## Recommendation

1. Shared "visible in CRM" query scope (refactor; no behavior change).
2. Hidden circles (B), with the switch in the contact list.
3. Bulk actions.
4. Automatic rule for organizations.
5. Later, if wanted: `KIND` (D) and the "hide new cards" rule.

Each step is useful alone.

## Open questions

1. Must a hidden contact keep its reminders and birthday reminders, or behave like archived
   (reminders deleted)?
2. Must hidden contacts show in the relationship and activity pickers? Example: the plumber in an
   activity "kitchen repair".
3. One merged switch "Show archived and hidden", or two switches?
4. Default name of the hidden circle: "Phonebook", or let the user pick on first use?
5. Is the "hide all new cards" rule wanted, or only the organization rule?
