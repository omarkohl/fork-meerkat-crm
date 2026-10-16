---
title: User Settings
nav_order: 5
has_children: false
---

# User Settings

## Language

Currently you can choose between **English** and **Deutsch** (German) for the application interface. The language change takes effect immediately. Your language preference is also used in the backend for email notifications (e.g., reminder emails). Backend messages (especially errors) always stay in English.

## Date Format

You can choose between the European (DD.MM.YYYY), US (MM/DD/YYYY), and ISO (YYYY-MM-DD) date format. This affects all date displays and also determines the expected input format when entering dates like birthdays.

## Appearance

Choose your preference between light mode and dark mode. This setting is stored locally in your browser and is not synced across devices.

## Custom Field Names

Define custom fields that appear on all your contacts for tracking information that doesn't fit into the standard contact fields. You can change the order of the custom fields, though they will always be displayed after the standard fields.

## Calendar Feed (ICS)

Under **Settings > Integrations** you can publish your birthdays and reminders as a calendar feed and subscribe to it from any calendar app (Google Calendar, Apple Calendar, Thunderbird, Outlook, ...). Updates in Meerkat show up the next time your calendar app refreshes the subscription, typically within a few hours.

1. Click **Generate feed URL** and copy the URL. It is shown only once, because Meerkat stores just a hash of its secret token.
2. Add the URL to your calendar app as a subscription ("add calendar from URL").

The feed contains the birthdays of your contacts (repeating every year, archived contacts are left out) and your open reminders (recurring reminders repeat with the same interval). It does not contain notes, activities or other contact details.

Anyone who has the URL can read the feed, without logging in. Keep it private. Meerkat does not write the URL to its own logs, but reverse proxies and load balancers in front of it (e.g. nginx access logs) record the full request path, token included, so protect those logs too. **Regenerate URL** replaces the secret and immediately stops the old URL from working (update your calendar apps with the new one); **Revoke feed** switches the feed off. The instance must be reachable from your calendar app, since the app fetches the feed itself.

Reminders are exported with their next date. A reminder set to recur from completion moves forward when you complete it, but the calendar repeats it at the fixed interval from that date, so the two can drift apart after a late completion. Monthly, quarterly and six-monthly reminders that start on the 29th to 31st fall on the last day of shorter months. A birthday on 29 February falls on 28 February in non-leap years. Dates are taken in UTC.

## Trash

Deleted contacts move to the **Trash** (sidebar, under Network), where you can restore them or delete them permanently. They are removed for good after `TRASH_RETENTION_DAYS` days (default 30). See [Contacts](data-model.html).

## Backups (administrators)

Administrators find **Settings > Backups**, where the automatic backups can be listed, created, downloaded and deleted. See [Backups](deployment.html#backups).
