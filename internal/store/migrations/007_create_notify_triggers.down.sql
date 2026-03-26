DROP TRIGGER IF EXISTS blocklists_changed ON blocklists;
DROP TRIGGER IF EXISTS blocklist_entries_changed ON blocklist_entries;
DROP TRIGGER IF EXISTS policies_changed ON policies;
DROP TRIGGER IF EXISTS ip_ranges_changed ON ip_ranges;
DROP FUNCTION IF EXISTS notify_config_change();