-- Trigger function that sends NOTIFY when blocklists change
CREATE OR REPLACE FUNCTION notify_config_change() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('config_changed', TG_TABLE_NAME);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Notify on blocklist changes
CREATE TRIGGER blocklists_changed
    AFTER INSERT OR UPDATE OR DELETE ON blocklists
    FOR EACH STATEMENT EXECUTE FUNCTION notify_config_change();

CREATE TRIGGER blocklist_entries_changed
    AFTER INSERT OR UPDATE OR DELETE ON blocklist_entries
    FOR EACH STATEMENT EXECUTE FUNCTION notify_config_change();

-- Notify on policy changes
CREATE TRIGGER policies_changed
    AFTER INSERT OR UPDATE OR DELETE ON policies
    FOR EACH STATEMENT EXECUTE FUNCTION notify_config_change();

-- Notify on IP range changes
CREATE TRIGGER ip_ranges_changed
    AFTER INSERT OR UPDATE OR DELETE ON ip_ranges
    FOR EACH STATEMENT EXECUTE FUNCTION notify_config_change();