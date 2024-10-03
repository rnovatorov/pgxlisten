BEGIN;

CREATE UNLOGGED TABLE members (
    group_id TEXT NOT NULL,
    id TEXT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    PRIMARY KEY (group_id, id)
);

CREATE INDEX ON members (group_id, expires_at);

CREATE FUNCTION members_tg_notify_of_insert ()
    RETURNS TRIGGER
    LANGUAGE PLPGSQL
    AS $$
BEGIN
    PERFORM
        pg_notify(format('members.%s', NEW.group_id), format('{"type":"inserted","id":"%s"}', NEW.id));
    RETURN NEW;
END;
$$;

CREATE TRIGGER members_tg_notify_of_insert
    AFTER INSERT ON members
    FOR EACH ROW
    EXECUTE FUNCTION members_tg_notify_of_insert ();

CREATE FUNCTION members_tg_notify_of_delete ()
    RETURNS TRIGGER
    LANGUAGE PLPGSQL
    AS $$
BEGIN
    PERFORM
        pg_notify(format('members.%s', OLD.group_id), format('{"type":"deleted","id":"%s"}', OLD.id));
    RETURN NULL;
END;
$$;

CREATE TRIGGER members_tg_notify_of_delete
    AFTER DELETE ON members
    FOR EACH ROW
    EXECUTE FUNCTION members_tg_notify_of_delete ();

END;
