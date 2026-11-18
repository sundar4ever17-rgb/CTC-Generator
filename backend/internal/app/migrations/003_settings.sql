CREATE TABLE application_settings (
 id integer PRIMARY KEY CHECK(id=1),
 session_idle_minutes integer NOT NULL CHECK(session_idle_minutes BETWEEN 5 AND 60),
 session_max_hours integer NOT NULL CHECK(session_max_hours BETWEEN 1 AND 24),
 updated_at timestamptz NOT NULL DEFAULT now(),
 updated_by text REFERENCES users(id)
);
INSERT INTO application_settings(id,session_idle_minutes,session_max_hours) VALUES(1,30,8);
