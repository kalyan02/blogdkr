use aes_gcm::{
    aead::{Aead, KeyInit},
    Aes256Gcm, Nonce,
};
use anyhow::{Context, Result};
use rand::RngCore;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::path::PathBuf;

#[derive(Debug, Serialize, Deserialize)]
pub struct TokenData {
    pub access_token: String,
    pub refresh_token: String,
    pub expires_at: i64,
}

pub struct SecureTokenStorage {
    file_path: PathBuf,
    key: [u8; 32],
}

impl SecureTokenStorage {
    pub fn new(file_path: PathBuf, password: &str) -> Self {
        let mut hasher = Sha256::new();
        hasher.update(password.as_bytes());
        hasher.update(b"dropbox_sync_salt");
        let key: [u8; 32] = hasher.finalize().into();

        Self { file_path, key }
    }

    pub fn save_token(&self, token_data: &TokenData) -> Result<()> {
        let json_data = serde_json::to_string(token_data)?;
        let encrypted_data = self.encrypt(&json_data)?;
        
        if let Some(parent) = self.file_path.parent() {
            std::fs::create_dir_all(parent)?;
        }
        
        std::fs::write(&self.file_path, encrypted_data)
            .context("Failed to write token file")?;
        
        Ok(())
    }

    pub fn load_token(&self) -> Result<TokenData> {
        let encrypted_data = std::fs::read(&self.file_path)
            .context("Failed to read token file")?;
        
        let json_data = self.decrypt(&encrypted_data)?;
        let token_data: TokenData = serde_json::from_str(&json_data)
            .context("Failed to parse token data")?;
        
        Ok(token_data)
    }

    pub fn token_exists(&self) -> bool {
        self.file_path.exists()
    }

    fn encrypt(&self, data: &str) -> Result<Vec<u8>> {
        let cipher = Aes256Gcm::new_from_slice(&self.key)?;
        
        let mut nonce_bytes = [0u8; 12];
        rand::thread_rng().fill_bytes(&mut nonce_bytes);
        let nonce = Nonce::from_slice(&nonce_bytes);
        
        let ciphertext = cipher
            .encrypt(nonce, data.as_bytes())
            .map_err(|e| anyhow::anyhow!("Encryption failed: {}", e))?;
        
        let mut result = nonce_bytes.to_vec();
        result.extend_from_slice(&ciphertext);
        
        Ok(result)
    }

    fn decrypt(&self, data: &[u8]) -> Result<String> {
        if data.len() < 12 {
            return Err(anyhow::anyhow!("Invalid encrypted data length"));
        }
        
        let cipher = Aes256Gcm::new_from_slice(&self.key)?;
        
        let (nonce_bytes, ciphertext) = data.split_at(12);
        let nonce = Nonce::from_slice(nonce_bytes);
        
        let plaintext = cipher
            .decrypt(nonce, ciphertext)
            .map_err(|e| anyhow::anyhow!("Decryption failed: {}", e))?;
        
        String::from_utf8(plaintext)
            .context("Decrypted data is not valid UTF-8")
    }

    pub fn get_default_token_path() -> PathBuf {
        let mut path = dirs::home_dir().unwrap_or_else(|| PathBuf::from("."));
        path.push(".dropbox_sync");
        path.push("tokens.enc");
        path
    }
}