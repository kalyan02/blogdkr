// @generated automatically by Diesel CLI.

diesel::table! {
    user (id) {
        id -> Nullable<Integer>,
        name -> Text,
        email -> Text,
        refresh_token -> Nullable<Text>,
        last_cursor -> Nullable<Text>,
        last_cursor_updated_at -> Nullable<Timestamp>,
        created_at -> Nullable<Timestamp>,
        updated_at -> Nullable<Timestamp>,
    }
}
