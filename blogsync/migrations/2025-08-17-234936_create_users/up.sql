-- Your SQL goes here
create table if not exists "user" (
    id integer primary key autoincrement,
    name text not null,
    email text not null unique,
    last_cursor text null,
    last_cursor_updated_at timestamp null,
    created_at timestamp null,
    updated_at timestamp null
);