use anyhow::{Context, Result};
use reqwest::Client;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::Path;

use crate::dropbox_auth::DropboxAuth;
use std::sync::Arc;

#[derive(Debug, Deserialize)]
struct ListFolderResponse {
    entries: Vec<Metadata>,
    cursor: String,
    has_more: bool,
}

/*
        {
            ".tag": "file",
            "client_modified": "2015-05-12T15:50:38Z",
            "content_hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
            "file_lock_info": {
                "created": "2015-05-12T15:50:38Z",
                "is_lockholder": true,
                "lockholder_name": "Imaginary User"
            },
            "has_explicit_shared_members": false,
            "id": "id:a4ayc_80_OEAAAAAAAAAXw",
            "is_downloadable": true,
            "name": "Prime_Numbers.txt",
            "path_display": "/Homework/math/Prime_Numbers.txt",
            "path_lower": "/homework/math/prime_numbers.txt",
            "property_groups": [
                {
                    "fields": [
                        {
                            "name": "Security Policy",
                            "value": "Confidential"
                        }
                    ],
                    "template_id": "ptid:1a5n2i6d3OYEAAAAAAAAAYa"
                }
            ],
            "rev": "a1c10ce0dd78",
            "server_modified": "2015-05-12T15:50:38Z",
            "sharing_info": {
                "modified_by": "dbid:AAH4f99T0taONIb-OurWxbNQ6ywGRopQngc",
                "parent_shared_folder_id": "84528192421",
                "read_only": true
            },
            "size": 7212
        }
*/

#[derive(Debug, Deserialize)]
struct Metadata {
    #[serde(rename = ".tag")]
    tag: String,
    name: String,
    path_lower: Option<String>,
    path_display: Option<String>,
    id: Option<String>,
    client_modified: Option<String>,
    server_modified: Option<String>,
    size: Option<u64>,
    #[serde(rename = "content_hash")]
    content_hash: Option<String>,
    #[serde(rename = "is_downloadable")]
    is_downloadable: Option<bool>,
}

#[derive(Debug, Serialize)]
struct ListFolderRequest {
    path: String,
    recursive: bool,
    include_media_info: bool,
    include_deleted: bool,
    include_has_explicit_shared_members: bool,
}

#[derive(Debug, Serialize)]
struct DownloadRequest {
    path: String,
}

#[derive(Debug, Serialize)]
struct ListFolderContinueRequest {
    cursor: String,
}

#[derive(Debug, Serialize)]
struct DownloadZipRequest {
    path: String,
}

#[derive(Debug, Deserialize)]
pub struct UserInfo {
    pub name: UserName,
    pub email: String,
    pub account_id: String,
}

#[derive(Debug, Deserialize)]
pub struct UserName {
    pub given_name: String,
    pub surname: String,
    pub familiar_name: String,
    pub display_name: String,
}

pub struct DropboxClient {
    client: Client,
    auth: Arc<DropboxAuth>,
}

impl DropboxClient {
    pub fn new(auth: Arc<DropboxAuth>) -> Self {
        Self {
            client: Client::new(),
            auth,
        }
    }

    pub async fn list_folder(&self, folder_path: &str, recursive: bool) -> Result<(Vec<FileInfo>, String)> {
        let access_token = self.auth.get_valid_access_token().await?;
        
        let request = ListFolderRequest {
            path: if folder_path == "/" { "".to_string() } else { folder_path.to_string() },
            recursive,
            include_media_info: false,
            include_deleted: false,
            include_has_explicit_shared_members: false,
        };

        let response = self
            .client
            .post("https://api.dropboxapi.com/2/files/list_folder")
            .header("Authorization", format!("Bearer {}", access_token))
            .header("Content-Type", "application/json")
            .json(&request)
            .send()
            .await
            .context("Failed to list folder")?;

        if !response.status().is_success() {
            let error_text = response.text().await.unwrap_or_default();
            return Err(anyhow::anyhow!("Dropbox API error: {}", error_text));
        }

        let mut list_response: ListFolderResponse = response
            .json()
            .await
            .context("Failed to parse list folder response")?;

        let mut all_files = Vec::new();
        
        // Process initial entries
        for entry in list_response.entries {
            if entry.tag == "file" {
                all_files.push(FileInfo {
                    name: entry.name,
                    path: entry.path_display.unwrap_or_default(),
                    size: entry.size.unwrap_or(0),
                    modified: entry.server_modified.unwrap_or_default(),
                    id: entry.id,
                    content_hash: entry.content_hash,
                    is_downloadable: entry.is_downloadable,
                });
            }
        }

        // Continue fetching if there are more entries
        while list_response.has_more {
            let continue_response = self.list_folder_continue(&list_response.cursor).await?;
            list_response = continue_response;
            
            for entry in &list_response.entries {
                if entry.tag == "file" {
                    all_files.push(FileInfo {
                        name: entry.name.clone(),
                        path: entry.path_display.clone().unwrap_or_default(),
                        size: entry.size.unwrap_or(0),
                        modified: entry.server_modified.clone().unwrap_or_default(),
                        id: entry.id.clone(),
                        content_hash: entry.content_hash.clone(),
                        is_downloadable: entry.is_downloadable,
                    });
                }
            }
        }

        Ok((all_files, list_response.cursor))
    }

    pub async fn list_folder_continue(&self, cursor: &str) -> Result<ListFolderResponse> {
        let access_token = self.auth.get_valid_access_token().await?;
        
        let request = ListFolderContinueRequest {
            cursor: cursor.to_string(),
        };

        let response = self
            .client
            .post("https://api.dropboxapi.com/2/files/list_folder/continue")
            .header("Authorization", format!("Bearer {}", access_token))
            .header("Content-Type", "application/json")
            .json(&request)
            .send()
            .await
            .context("Failed to continue listing folder")?;

        if !response.status().is_success() {
            let error_text = response.text().await.unwrap_or_default();
            return Err(anyhow::anyhow!("Dropbox API error: {}", error_text));
        }

        response
            .json()
            .await
            .context("Failed to parse list folder continue response")
    }

    pub async fn get_changes_from_cursor(&self, cursor: &str) -> Result<Vec<FileInfo>> {
        let continue_response = self.list_folder_continue(cursor).await?;
        let mut all_files = Vec::new();
        let mut current_response = continue_response;
        
        loop {
            for entry in &current_response.entries {
                if entry.tag == "file" {
                    all_files.push(FileInfo {
                        name: entry.name.clone(),
                        path: entry.path_display.clone().unwrap_or_default(),
                        size: entry.size.unwrap_or(0),
                        modified: entry.server_modified.clone().unwrap_or_default(),
                        id: entry.id.clone(),
                        content_hash: entry.content_hash.clone(),
                        is_downloadable: entry.is_downloadable,
                    });
                }
            }
            
            if !current_response.has_more {
                break;
            }
            
            current_response = self.list_folder_continue(&current_response.cursor).await?;
        }
        
        Ok(all_files)
    }

    pub async fn download_file(&self, dropbox_path: &str, local_path: &Path) -> Result<()> {
        let access_token = self.auth.get_valid_access_token().await?;
        
        let download_request = DownloadRequest {
            path: dropbox_path.to_string(),
        };

        let response = self
            .client
            .post("https://content.dropboxapi.com/2/files/download")
            .header("Authorization", format!("Bearer {}", access_token))
            .header("Dropbox-API-Arg", serde_json::to_string(&download_request)?)
            .send()
            .await
            .context("Failed to download file")?;

        if !response.status().is_success() {
            let error_text = response.text().await.unwrap_or_default();
            return Err(anyhow::anyhow!("Dropbox download error: {}", error_text));
        }

        let bytes = response.bytes().await?;
        
        if let Some(parent) = local_path.parent() {
            std::fs::create_dir_all(parent)?;
        }
        
        std::fs::write(local_path, bytes)?;
        Ok(())
    }

    pub async fn download_zip(&self, folder_path: &str, local_zip_path: &Path) -> Result<()> {
        let access_token = self.auth.get_valid_access_token().await?;
        
        let download_request = DownloadZipRequest {
            path: if folder_path == "/" || folder_path.is_empty() { 
                "".to_string() 
            } else { 
                folder_path.to_string() 
            },
        };

        let response = self
            .client
            .post("https://content.dropboxapi.com/2/files/download_zip")
            .header("Authorization", format!("Bearer {}", access_token))
            .header("Dropbox-API-Arg", serde_json::to_string(&download_request)?)
            .send()
            .await
            .context("Failed to download zip")?;

        if !response.status().is_success() {
            let error_text = response.text().await.unwrap_or_default();
            return Err(anyhow::anyhow!("Dropbox download zip error: {}", error_text));
        }

        let bytes = response.bytes().await?;
        
        if let Some(parent) = local_zip_path.parent() {
            std::fs::create_dir_all(parent)?;
        }
        
        std::fs::write(local_zip_path, bytes)?;
        Ok(())
    }

    pub async fn setup_webhook(&self, webhook_url: &str) -> Result<()> {
        let access_token = self.auth.get_valid_access_token().await?;
        
        let mut params = HashMap::new();
        params.insert("url", webhook_url);

        let response = self
            .client
            .post("https://api.dropboxapi.com/2/files/list_folder/get_latest_cursor")
            .header("Authorization", format!("Bearer {}", access_token))
            .header("Content-Type", "application/json")
            .json(&serde_json::json!({
                "path": "",
                "recursive": true
            }))
            .send()
            .await
            .context("Failed to get initial cursor for webhook")?;

        if !response.status().is_success() {
            let error_text = response.text().await.unwrap_or_default();
            return Err(anyhow::anyhow!("Failed to setup webhook: {}", error_text));
        }

        Ok(())
    }

    pub async fn get_current_account(&self) -> Result<UserInfo> {
        let access_token = self.auth.get_valid_access_token().await?;
        
        let response = self
            .client
            .post("https://api.dropboxapi.com/2/users/get_current_account")
            .header("Authorization", format!("Bearer {}", access_token))
            .send()
            .await
            .context("Failed to get current account")?;

        if !response.status().is_success() {
            let error_text = response.text().await.unwrap_or_default();
            return Err(anyhow::anyhow!("Dropbox API error: {}", error_text));
        }

        let user_info: UserInfo = response
            .json()
            .await
            .context("Failed to parse current account response")?;

        Ok(user_info)
    }
}

#[derive(Debug, Clone)]
pub struct FileInfo {
    pub name: String,
    pub path: String,
    pub size: u64,
    pub modified: String,
    // id
    pub id: Option<String>,
    // content_hash
    pub content_hash: Option<String>,
    // is_downloadable
    pub is_downloadable: Option<bool>,
}