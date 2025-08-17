use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Config {
    pub dropbox: DropboxConfig,
    pub server: ServerConfig,
    pub sync: SyncConfig,
    pub build: BuildConfig,
    pub copy_rules: Vec<CopyRule>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct DropboxConfig {
    pub app_key: String,
    pub app_secret: String,
    pub redirect_uri: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ServerConfig {
    pub host: String,
    pub port: u16,
    pub admin_port: u16,
    pub webhook_path: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct SyncConfig {
    pub local_base_path: String,
    pub dropbox_folder: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct BuildConfig {
    pub command: String,
    pub working_directory: String,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct CopyRule {
    pub source_pattern: String,
    pub destination: String,
    pub recursive: Option<bool>,
}

impl Config {
    pub fn load_from_file(path: &str) -> anyhow::Result<Self> {
        let content = std::fs::read_to_string(path)?;
        let config: Config = toml::from_str(&content)?;
        Ok(config)
    }

    pub fn save_to_file(&self, path: &str) -> anyhow::Result<()> {
        let content = toml::to_string_pretty(self)?;
        std::fs::write(path, content)?;
        Ok(())
    }
}

impl Default for Config {
    fn default() -> Self {
        Self {
            dropbox: DropboxConfig {
                app_key: "your_app_key".to_string(),
                app_secret: "your_app_secret".to_string(),
                redirect_uri: "http://localhost:3000/auth/callback".to_string(),
            },
            server: ServerConfig {
                host: "0.0.0.0".to_string(),
                port: 3000,
                admin_port: 3001,
                webhook_path: "/webhook".to_string(),
            },
            sync: SyncConfig {
                local_base_path: "./sync".to_string(),
                dropbox_folder: "/".to_string(),
            },
            build: BuildConfig {
                command: "zola build".to_string(),
                working_directory: "./sync".to_string(),
            },
            copy_rules: vec![
                CopyRule {
                    source_pattern: "./sync/public/**/*".to_string(),
                    destination: "./output".to_string(),
                    recursive: Some(true),
                },
            ],
        }
    }
}