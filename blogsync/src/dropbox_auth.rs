use anyhow::{Context, Result};
use reqwest::Client;
use serde::{Deserialize, Serialize};
use std::time::{SystemTime, UNIX_EPOCH};
use url::Url;

use crate::config::DropboxConfig;
use crate::token_storage::{SecureTokenStorage, TokenData};

#[derive(Debug, Serialize, Deserialize)]
struct TokenResponse {
    access_token: String,
    token_type: String,
    expires_in: Option<u64>,
    refresh_token: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
struct RefreshTokenResponse {
    access_token: String,
    token_type: String,
    expires_in: Option<u64>,
}

pub struct DropboxAuth {
    config: DropboxConfig,
    client: Client,
    token_storage: SecureTokenStorage,
}

impl DropboxAuth {
    pub fn new(config: DropboxConfig, token_storage: SecureTokenStorage) -> Self {
        Self {
            config,
            client: Client::new(),
            token_storage,
        }
    }

    pub fn get_authorization_url(&self, state: &str) -> Result<String> {
        let mut url = Url::parse("https://www.dropbox.com/oauth2/authorize")?;
        
        url.query_pairs_mut()
            .append_pair("response_type", "code")
            .append_pair("client_id", &self.config.app_key)
            .append_pair("redirect_uri", &self.config.redirect_uri)
            .append_pair("state", state)
            .append_pair("token_access_type", "offline");
        
        Ok(url.to_string())
    }

    pub async fn exchange_code_for_token(&self, code: &str) -> Result<()> {
        let params = [
            ("code", code),
            ("grant_type", "authorization_code"),
            ("client_id", &self.config.app_key),
            ("client_secret", &self.config.app_secret),
            ("redirect_uri", &self.config.redirect_uri),
        ];

        let response = self
            .client
            .post("https://api.dropbox.com/oauth2/token")
            .form(&params)
            .send()
            .await
            .context("Failed to exchange code for token")?;

        let token_response: TokenResponse = response
            .json()
            .await
            .context("Failed to parse token response")?;

        let expires_at = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs() as i64
            + token_response.expires_in.unwrap_or(14400) as i64;

        let token_data = TokenData {
            access_token: token_response.access_token,
            refresh_token: token_response.refresh_token.unwrap_or_default(),
            expires_at,
        };

        self.token_storage.save_token(&token_data)?;
        Ok(())
    }

    pub async fn get_valid_access_token(&self) -> Result<String> {
        if !self.token_storage.token_exists() {
            return Err(anyhow::anyhow!("No stored token found. Please authenticate first."));
        }

        let token_data = self.token_storage.load_token()?;
        
        let current_time = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs() as i64;

        if current_time < token_data.expires_at - 300 {
            return Ok(token_data.access_token);
        }

        if token_data.refresh_token.is_empty() {
            return Err(anyhow::anyhow!("Token expired and no refresh token available"));
        }

        self.refresh_access_token(&token_data.refresh_token).await
    }

    async fn refresh_access_token(&self, refresh_token: &str) -> Result<String> {
        let params = [
            ("grant_type", "refresh_token"),
            ("refresh_token", refresh_token),
            ("client_id", &self.config.app_key),
            ("client_secret", &self.config.app_secret),
        ];

        let response = self
            .client
            .post("https://api.dropbox.com/oauth2/token")
            .form(&params)
            .send()
            .await
            .context("Failed to refresh token")?;

        let refresh_response: RefreshTokenResponse = response
            .json()
            .await
            .context("Failed to parse refresh token response")?;

        let expires_at = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs() as i64
            + refresh_response.expires_in.unwrap_or(14400) as i64;

        let token_data = TokenData {
            access_token: refresh_response.access_token.clone(),
            refresh_token: refresh_token.to_string(),
            expires_at,
        };

        self.token_storage.save_token(&token_data)?;
        Ok(refresh_response.access_token)
    }

    pub fn has_valid_token(&self) -> bool {
        if !self.token_storage.token_exists() {
            return false;
        }

        if let Ok(token_data) = self.token_storage.load_token() {
            let current_time = SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap()
                .as_secs() as i64;
            
            return current_time < token_data.expires_at;
        }

        false
    }
}