use sea_orm::{Database, Schema, StatementBuilder};
use sea_orm::entity::prelude::*;
use sea_orm::sea_query::SqliteQueryBuilder;
use tracing::info;

// User model
pub mod users {
    use super::*;

    #[derive(Clone, Debug, PartialEq, Eq, DeriveEntityModel)]
    #[sea_orm(table_name = "users")]
    pub struct Model {
        #[sea_orm(primary_key)]
        pub id: i32,
        pub username: String,
        pub email: String,
        pub refresh_token: String,
        pub last_cursor: Option<String>,
        pub last_cursor_updated_at: Option<DateTimeUtc>,
        pub created_at: DateTimeUtc,
    }

    #[derive(Copy, Clone, Debug, EnumIter, DeriveRelation)]
    pub enum Relation {}
    impl ActiveModelBehavior for ActiveModel {}
}

// Files model
pub mod files {
    use super::*;

    #[derive(Clone, Debug, PartialEq, Eq, DeriveEntityModel)]
    #[sea_orm(table_name = "files")]
    pub struct Model {
        #[sea_orm(primary_key)]
        pub id: i32,
        pub user_id: i32,
        pub file_id_dbx: String,
        pub file_name: String,
        pub file_path: String,
        pub file_hash_dbx: String,
        pub file_hash_sha256: String,
        pub file_size: i64,
        pub deleted: bool,
        pub created_at: DateTimeUtc,
        pub updated_at: DateTimeUtc,
        pub deleted_at: Option<DateTimeUtc>,
    }

    #[derive(Copy, Clone, Debug, EnumIter, DeriveRelation)]
    pub enum Relation {
        User,
    }

    impl ActiveModelBehavior for ActiveModel {}
}



// sqlite connect
pub async fn sqlite3_connect(name: &str) -> Result<DatabaseConnection, DbErr> {
    let db: DatabaseConnection = Database::connect(format!("sqlite://{}", name))?.await;
    Ok(db)
}