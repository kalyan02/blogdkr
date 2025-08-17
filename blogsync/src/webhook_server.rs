use axum::{
    body::Bytes,
    extract::{Query, State},
    http::{header, HeaderMap, StatusCode},
    response::{Html, Json, Response},
    routing::{get, post},
    Router,
};
use serde::Deserialize;
use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::mpsc;
use tower_http::cors::CorsLayer;
use tracing::{info, warn, error};
use uuid;

use crate::config::Config;
use crate::dropbox_auth::DropboxAuth;
use crate::dropbox_client::DropboxClient;

#[derive(Clone)]
pub struct AppState {
    pub config: Arc<Config>,
    pub sync_sender: mpsc::UnboundedSender<SyncEvent>,
    pub auth: Arc<DropboxAuth>,
}

#[derive(Debug)]
pub enum SyncEvent {
    FilesChanged,
    FilesChangedWithCursor(String),
    ForceSync,
}

#[derive(Debug, Deserialize)]
struct WebhookVerification {
    challenge: String,
}

#[derive(Debug, Deserialize)]
struct WebhookNotification {
    list_folder: Option<WebhookAccount>,
    delta: Option<WebhookDelta>,
}

#[derive(Debug, Deserialize)]
struct WebhookAccount {
    accounts: Vec<String>,
}

#[derive(Debug, Deserialize)]
struct WebhookDelta {
    users: Vec<u64>,
}

pub struct WebhookServer {
    config: Arc<Config>,
    sync_sender: mpsc::UnboundedSender<SyncEvent>,
    auth: Arc<DropboxAuth>,
}

impl WebhookServer {
    pub fn new(
        config: Arc<Config>, 
        sync_sender: mpsc::UnboundedSender<SyncEvent>,
        auth: Arc<DropboxAuth>
    ) -> Self {
        Self {
            config,
            sync_sender,
            auth,
        }
    }

    pub async fn start(&self) -> anyhow::Result<()> {
        let app_state = AppState {
            config: self.config.clone(),
            sync_sender: self.sync_sender.clone(),
            auth: self.auth.clone(),
        };

        // Public server (port 3000) - webhooks and auth callbacks
        let public_app = Router::new()
            .route("/", get(index))
            .route(&self.config.server.webhook_path, get(webhook_verification))
            .route(&self.config.server.webhook_path, post(webhook_notification))
            .route("/auth/callback", get(auth_callback))
            .route("/health", get(health_check))
            .layer(CorsLayer::permissive())
            .with_state(app_state.clone());

        // Admin server (port 3001) - admin endpoints (firewalled)
        let admin_app = Router::new()
            .route("/admin/sync", post(manual_sync))
            .route("/admin/sync_zip", post(sync_zip))
            .route("/admin/status", get(admin_status))
            .route("/admin/auth", get(start_auth))
            .route("/admin/test", get(test_dropbox))
            .route("/admin/webhooks", get(webhook_history))
            .route("/admin/health", get(health_check))
            .layer(CorsLayer::permissive())
            .with_state(app_state);

        let public_addr = format!("{}:{}", self.config.server.host, self.config.server.port);
        let admin_addr = format!("{}:{}", self.config.server.host, self.config.server.admin_port);
        
        info!("Starting public server on {} (webhooks, auth)", public_addr);
        info!("Starting admin server on {} (admin endpoints)", admin_addr);

        let public_listener = tokio::net::TcpListener::bind(&public_addr).await?;
        let admin_listener = tokio::net::TcpListener::bind(&admin_addr).await?;

        // Run both servers concurrently
        tokio::try_join!(
            axum::serve(public_listener, public_app),
            axum::serve(admin_listener, admin_app)
        )?;

        Ok(())
    }
}

async fn index() -> Html<&'static str> {
    Html(r#"
    <!DOCTYPE html>
    <html>
    <head>
        <title>BlogSync Service</title>
        <style>
            body { 
                font-family: Arial, sans-serif; 
                text-align: center; 
                margin-top: 100px; 
                background: #f5f5f5; 
            }
            .container { 
                max-width: 600px; 
                margin: 0 auto; 
                padding: 40px; 
                background: white; 
                border-radius: 8px; 
                box-shadow: 0 2px 10px rgba(0,0,0,0.1); 
            }
            .status { color: #4CAF50; font-size: 24px; margin-bottom: 20px; }
            .info { color: #666; margin: 10px 0; }
            .endpoint { 
                background: #f0f0f0; 
                padding: 8px 12px; 
                border-radius: 4px; 
                font-family: monospace; 
                margin: 5px 0; 
            }
        </style>
    </head>
    <body>
        <div class="container">
            <div class="status">✅ BlogSync Service Running</div>
            <p class="info">Your Dropbox synchronization service is online and ready!</p>
            
            <h3>Public Endpoints</h3>
            <div class="endpoint">GET /health</div>
            <div class="endpoint">GET/POST /webhook</div>
            <div class="endpoint">GET /auth/callback</div>
            
            <p class="info">
                <strong>Admin endpoints are available on port 3001</strong><br>
                (should be firewalled from public access)<br><br>
                <strong>Admin Endpoints:</strong><br>
                <div class="endpoint">POST /admin/sync</div>
                <div class="endpoint">POST /admin/sync_zip</div>
                <div class="endpoint">GET /admin/status</div>
                <div class="endpoint">GET /admin/auth</div>
                <div class="endpoint">GET /admin/test</div>
            </p>
        </div>
    </body>
    </html>
    "#)
}

async fn webhook_verification(
    Query(params): Query<HashMap<String, String>>,
) -> Result<String, StatusCode> {
    if let Some(challenge) = params.get("challenge") {
        info!("Webhook verification request received");
        Ok(challenge.clone())
    } else {
        warn!("Webhook verification request missing challenge parameter");
        Err(StatusCode::BAD_REQUEST)
    }
}

async fn webhook_notification(
    State(state): State<AppState>,
    headers: axum::http::HeaderMap,
    body: axum::body::Bytes,
) -> Result<String, StatusCode> {
    info!("=== WEBHOOK RECEIVED ===");
    info!("Timestamp: {}", chrono::Utc::now().to_rfc3339());
    
    // Log headers
    for (name, value) in headers.iter() {
        info!("Header {}: {:?}", name, value);
    }
    
    // Log raw body
    let body_str = String::from_utf8_lossy(&body);
    info!("Raw body: {}", body_str);
    
    // Try to parse as JSON if not empty
    if !body.is_empty() {
        match serde_json::from_slice::<WebhookNotification>(&body) {
            Ok(payload) => {
                info!("Parsed JSON payload: {:?}", payload);
            }
            Err(e) => {
                warn!("Failed to parse JSON payload: {}", e);
            }
        }
    }
    
    info!("========================");
    
    if let Err(e) = state.sync_sender.send(SyncEvent::FilesChanged) {
        error!("Failed to send sync event: {}", e);
        return Err(StatusCode::INTERNAL_SERVER_ERROR);
    }

    info!("Sync event sent successfully");
    Ok("OK".to_string())
}

async fn auth_callback(
    Query(params): Query<HashMap<String, String>>,
    State(state): State<AppState>,
) -> Result<Html<String>, StatusCode> {
    info!("OAuth callback received");
    
    if let Some(error) = params.get("error") {
        error!("OAuth error: {}", error);
        return Ok(Html(format!(
            r#"
            <!DOCTYPE html>
            <html>
            <head><title>Authentication Failed</title>
            <style>body{{font-family:Arial;text-align:center;margin-top:100px;}}.error{{color:#f44336;font-size:24px;}}</style>
            </head>
            <body>
                <div class="error">❌ Authentication Failed</div>
                <p>Dropbox Error: {}</p>
                <p>Please try again.</p>
            </body>
            </html>
            "#, error
        )));
    }

    if let Some(code) = params.get("code") {
        info!("Received authorization code: {}, exchanging for token", &code[..10]);
        
        match state.auth.exchange_code_for_token(code).await {
            Ok(()) => {
                info!("Token exchange successful!");
                Ok(Html(format!(
                    r#"
                    <!DOCTYPE html>
                    <html>
                    <head><title>Authentication Successful</title>
                    <style>body{{font-family:Arial;text-align:center;margin-top:100px;}}.success{{color:#4CAF50;font-size:24px;}}</style>
                    </head>
                    <body>
                        <div class="success">✅ Authentication Successful!</div>
                        <p>Your Dropbox account has been successfully linked.</p>
                        <p>You can now close this window and use the BlogSync service.</p>
                        <script>setTimeout(() => window.close(), 3000);</script>
                    </body>
                    </html>
                    "#
                )))
            }
            Err(e) => {
                error!("Token exchange failed: {}", e);
                Ok(Html(format!(
                    r#"
                    <!DOCTYPE html>
                    <html>
                    <head><title>Authentication Failed</title>
                    <style>body{{font-family:Arial;text-align:center;margin-top:100px;}}.error{{color:#f44336;font-size:24px;}}</style>
                    </head>
                    <body>
                        <div class="error">❌ Authentication Failed</div>
                        <p>Error: {}</p>
                        <p>Please try again or check the logs.</p>
                    </body>
                    </html>
                    "#, e
                )))
            }
        }
    } else {
        warn!("OAuth callback missing required parameters");
        Err(StatusCode::BAD_REQUEST)
    }
}

async fn health_check() -> Json<serde_json::Value> {
    Json(serde_json::json!({
        "status": "healthy",
        "timestamp": chrono::Utc::now().to_rfc3339()
    }))
}

async fn manual_sync(
    State(state): State<AppState>,
) -> Result<Response, StatusCode> {
    info!("Manual sync requested");
    
    let json_data = if let Err(e) = state.sync_sender.send(SyncEvent::ForceSync) {
        error!("Failed to send manual sync event: {}", e);
        return Err(StatusCode::INTERNAL_SERVER_ERROR);
    } else {
        serde_json::json!({
            "status": "sync_triggered",
            "message": "Sync process has been triggered",
            "timestamp": chrono::Utc::now().to_rfc3339()
        })
    };

    let pretty_json = serde_json::to_string_pretty(&json_data).unwrap_or_default();
    
    Ok(Response::builder()
        .status(StatusCode::OK)
        .header(header::CONTENT_TYPE, "application/json")
        .body(pretty_json.into())
        .unwrap())
}

async fn sync_zip(
    State(state): State<AppState>,
) -> Result<Response, StatusCode> {
    info!("Zip sync requested");
    
    let json_data = if !state.auth.has_valid_token() {
        serde_json::json!({
            "status": "error",
            "message": "Not authenticated. Run /admin/auth first."
        })
    } else {
        let client = DropboxClient::new(state.auth.clone());
        
        // Try to download as zip
        let temp_zip_path = std::path::Path::new("/tmp/dropbox_sync.zip");

        info!("Requesting download from path: {}", state.config.sync.dropbox_folder);
        info!("Temporary zip path: {:?}", temp_zip_path);
        match client.download_zip(&state.config.sync.dropbox_folder, temp_zip_path).await {
            Ok(()) => {
                // Check if file was created and get its size
                match std::fs::metadata(temp_zip_path) {
                    Ok(metadata) => {
                        info!("Successfully downloaded zip file: {} bytes", metadata.len());
                        let zip_size = metadata.len();
                        
                        // Extract the zip contents to sync folder
                        let sync_folder = std::path::Path::new(&state.config.sync.local_base_path);
                        match extract_zip(temp_zip_path, sync_folder).await {
                            Ok(extracted_count) => {
                                info!("Successfully extracted {} files from zip", extracted_count);
                                
                                // Clean up the temp file
                                // if let Err(e) = std::fs::remove_file(temp_zip_path) {
                                //     warn!("Failed to clean up temp zip file: {}", e);
                                // }
                                
                                serde_json::json!({
                                    "status": "success",
                                    "message": "Successfully downloaded and extracted zip from Dropbox",
                                    "zip_size": zip_size,
                                    "files_extracted": extracted_count,
                                    "extracted_to": state.config.sync.local_base_path,
                                    "timestamp": chrono::Utc::now().to_rfc3339()
                                })
                            }
                            Err(e) => {
                                error!("Failed to extract zip: {}", e);
                                
                                // Clean up the temp file even on extraction error
                                if let Err(cleanup_err) = std::fs::remove_file(temp_zip_path) {
                                    warn!("Failed to clean up temp zip file after extraction error: {}", cleanup_err);
                                }
                                
                                serde_json::json!({
                                    "status": "error",
                                    "message": format!("Downloaded zip but failed to extract: {}", e),
                                    "zip_size": zip_size
                                })
                            }
                        }
                    }
                    Err(e) => {
                        error!("Failed to get zip metadata: {}", e);
                        serde_json::json!({
                            "status": "error",
                            "message": format!("Failed to get zip metadata: {}", e)
                        })
                    }
                }
            }
            Err(e) => {
                error!("Zip download failed: {}", e);
                serde_json::json!({
                    "status": "error",
                    "message": format!("Zip download failed: {}", e)
                })
            }
        }
    };
    
    let pretty_json = serde_json::to_string_pretty(&json_data).unwrap_or_default();
    
    Ok(Response::builder()
        .status(StatusCode::OK)
        .header(header::CONTENT_TYPE, "application/json")
        .body(pretty_json.into())
        .unwrap())
}

async fn extract_zip(zip_path: &std::path::Path, extract_to: &std::path::Path) -> anyhow::Result<usize> {
    use zip::ZipArchive;
    
    let file = std::fs::File::open(zip_path)?;
    let mut archive = ZipArchive::new(file)?;
    
    // Create the extraction directory if it doesn't exist
    std::fs::create_dir_all(extract_to)?;
    
    let mut extracted_count = 0;
    
    for i in 0..archive.len() {
        let mut file = archive.by_index(i)?;
        let outpath = match file.enclosed_name() {
            Some(path) => extract_to.join(path),
            None => continue,
        };
        
        if file.name().ends_with('/') {
            // Directory
            info!("Creating directory: {:?}", outpath);
            std::fs::create_dir_all(&outpath)?;
        } else {
            // File
            info!("Extracting file: {:?}", outpath);
            
            if let Some(parent) = outpath.parent() {
                if !parent.exists() {
                    std::fs::create_dir_all(parent)?;
                }
            }
            
            let mut outfile = std::fs::File::create(&outpath)?;
            std::io::copy(&mut file, &mut outfile)?;
            extracted_count += 1;
        }
    }
    
    Ok(extracted_count)
}

async fn admin_status(
    State(state): State<AppState>,
) -> Response {
    let has_valid_token = state.auth.has_valid_token();
    
    let mut json_data = serde_json::json!({
        "status": "running",
        "timestamp": chrono::Utc::now().to_rfc3339(),
        "authenticated": has_valid_token,
        "config": {
            "public_port": state.config.server.port,
            "admin_port": state.config.server.admin_port,
            "webhook_path": state.config.server.webhook_path,
            "sync_path": state.config.sync.local_base_path,
            "build_command": state.config.build.command
        }
    });

    if has_valid_token {
        let client = DropboxClient::new(state.auth.clone());
        match client.get_current_account().await {
            Ok(user_info) => {
                json_data["dropbox_user"] = serde_json::json!({
                    "display_name": user_info.name.display_name,
                    "email": user_info.email,
                    "account_id": user_info.account_id
                });
            }
            Err(e) => {
                warn!("Failed to get user info: {}", e);
                json_data["dropbox_user"] = serde_json::json!({
                    "error": format!("Failed to get user info: {}", e)
                });
            }
        }
    } else {
        json_data["dropbox_user"] = serde_json::Value::Null;
    }
    
    let pretty_json = serde_json::to_string_pretty(&json_data).unwrap_or_default();
    
    Response::builder()
        .status(StatusCode::OK)
        .header(header::CONTENT_TYPE, "application/json")
        .body(pretty_json.into())
        .unwrap()
}

async fn webhook_history() -> Response {
    let json_data = serde_json::json!({
        "message": "Webhook history not implemented yet - check service logs for webhook activity",
        "tip": "Look for '=== WEBHOOK RECEIVED ===' in logs"
    });
    
    let pretty_json = serde_json::to_string_pretty(&json_data).unwrap_or_default();
    
    Response::builder()
        .status(StatusCode::OK)
        .header(header::CONTENT_TYPE, "application/json")
        .body(pretty_json.into())
        .unwrap()
}

async fn start_auth(
    State(state): State<AppState>,
) -> Result<Response, StatusCode> {
    info!("Auth flow requested via admin endpoint");
    
    let json_data = if state.auth.has_valid_token() {
        serde_json::json!({
            "status": "already_authenticated",
            "message": "Already authenticated with valid token"
        })
    } else {
        match state.auth.get_authorization_url(&uuid::Uuid::new_v4().to_string()) {
            Ok(auth_url) => {
                info!("Generated auth URL: {}", auth_url);
                serde_json::json!({
                    "status": "auth_url_generated",
                    "auth_url": auth_url,
                    "message": "Open this URL in your browser to authenticate",
                    "callback_url": format!("http://{}:{}/auth/callback", 
                        state.config.server.host, state.config.server.port)
                })
            }
            Err(e) => {
                error!("Failed to generate auth URL: {}", e);
                return Err(StatusCode::INTERNAL_SERVER_ERROR);
            }
        }
    };
    
    let pretty_json = serde_json::to_string_pretty(&json_data).unwrap_or_default();
    
    Ok(Response::builder()
        .status(StatusCode::OK)
        .header(header::CONTENT_TYPE, "application/json")
        .body(pretty_json.into())
        .unwrap())
}
async fn test_dropbox(
    State(state): State<AppState>,
) -> Result<Response, StatusCode> {
    info!("Testing Dropbox connection");
    
    let json_data = if !state.auth.has_valid_token() {
        serde_json::json!({
            "status": "error",
            "message": "Not authenticated. Run /admin/auth first."
        })
    } else {
        let client = DropboxClient::new(state.auth.clone());
        match client.list_folder("/", false).await {
            Ok((files, _cursor)) => {
                serde_json::json!({
                    "status": "success",
                    "message": "Successfully connected to Dropbox",
                    "file_count": files.len(),
                    "files": files.iter().take(10).map(|f| &f.path).collect::<Vec<_>>(),
                    "note": if files.len() > 10 { "Showing first 10 files only" } else { "All files shown" }
                })
            }
            Err(e) => {
                error!("Dropbox API error: {}", e);
                serde_json::json!({
                    "status": "error",
                    "message": format!("Dropbox API error: {}", e)
                })
            }
        }
    };
    
    let pretty_json = serde_json::to_string_pretty(&json_data).unwrap_or_default();
    
    Ok(Response::builder()
        .status(StatusCode::OK)
        .header(header::CONTENT_TYPE, "application/json")
        .body(pretty_json.into())
        .unwrap())
}
