mod config;
mod token_storage;
mod dropbox_auth;
mod dropbox_client;
mod webhook_server;
mod sync;
mod content_hash;

use anyhow::Result;
use clap::{Parser, Subcommand};
use std::sync::Arc;
use tokio::sync::mpsc;
use tracing::{info, error};

use config::Config;
use token_storage::SecureTokenStorage;
use dropbox_auth::DropboxAuth;
use dropbox_client::DropboxClient;
use webhook_server::WebhookServer;
use sync::SyncManager;

#[derive(Parser)]
#[command(name = "dropbox-sync")]
#[command(about = "A Dropbox file synchronization tool with webhook support")]
struct Cli {
    #[command(subcommand)]
    command: Commands,
    
    #[arg(short, long, default_value = "config.toml")]
    config: String,
    
    #[arg(short, long, help = "Password for token encryption")]
    password: Option<String>,
}

#[derive(Subcommand)]
enum Commands {
    #[command(about = "Start the webhook server and sync daemon")]
    Start,
    
    #[command(about = "Generate a default configuration file")]
    InitConfig,
    
    #[command(about = "Print the current access token for API testing")]
    Token,
}

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt::init();
    
    let cli = Cli::parse();
    
    match cli.command {
        Commands::InitConfig => {
            generate_default_config(&cli.config)?;
            println!("Generated default configuration at: {}", cli.config);
            Ok(())
        }
        Commands::Start => {
            start_server(&cli).await
        }
        Commands::Token => {
            print_token(&cli).await
        }
    }
}

fn generate_default_config(config_path: &str) -> Result<()> {
    let default_config = Config::default();
    default_config.save_to_file(config_path)?;
    Ok(())
}


async fn start_server(cli: &Cli) -> Result<()> {
    let config = Arc::new(load_config(&cli.config)?);
    let password = get_password(cli)?;
    let token_storage = SecureTokenStorage::new(
        SecureTokenStorage::get_default_token_path(),
        &password,
    );
    
    let auth = DropboxAuth::new(config.dropbox.clone(), token_storage);
    let auth_arc = Arc::new(auth);
    let dropbox_client = DropboxClient::new(auth_arc.clone());
    let mut sync_manager = SyncManager::new((*config).clone(), dropbox_client);
    
    let (sync_sender, sync_receiver) = mpsc::unbounded_channel();
    
    let webhook_server = WebhookServer::new(config.clone(), sync_sender, auth_arc.clone());
    
    let sync_handle = {
        tokio::spawn(async move {
            sync_manager.start_sync_loop(sync_receiver).await;
        })
    };
    
    let server_handle = tokio::spawn(async move {
        if let Err(e) = webhook_server.start().await {
            error!("Webhook server error: {}", e);
        }
    });
    
    info!("BlogSync service started");
    info!("Public server: http://{}:{} (webhooks, auth)", 
          config.server.host, config.server.port);
    info!("Admin server: http://{}:{} (admin endpoints)", 
          config.server.host, config.server.admin_port);
    info!("Webhook endpoint: http://{}:{}{}", 
          config.server.host, config.server.port, config.server.webhook_path);
    info!("Auth callback: http://{}:{}/auth/callback", 
          config.server.host, config.server.port);
    
    tokio::select! {
        _ = sync_handle => {
            error!("Sync manager stopped unexpectedly");
        }
        _ = server_handle => {
            error!("Webhook server stopped unexpectedly");
        }
        _ = tokio::signal::ctrl_c() => {
            info!("Shutdown signal received");
        }
    }
    
    Ok(())
}

fn load_config(config_path: &str) -> Result<Config> {
    if !std::path::Path::new(config_path).exists() {
        return Err(anyhow::anyhow!(
            "Configuration file not found: {}. Run 'dropbox-sync init-config' to generate one.",
            config_path
        ));
    }
    
    Config::load_from_file(config_path)
}

async fn print_token(cli: &Cli) -> Result<()> {
    let config = load_config(&cli.config)?;
    let password = get_password(cli)?;
    let token_storage = SecureTokenStorage::new(
        SecureTokenStorage::get_default_token_path(),
        &password,
    );
    
    let auth = DropboxAuth::new(config.dropbox, token_storage);
    
    match auth.get_valid_access_token().await {
        Ok(token) => {
            println!("{}", token);
            Ok(())
        }
        Err(e) => {
            eprintln!("Failed to get access token: {}", e);
            eprintln!("You may need to authenticate first by running the server and visiting /admin/auth");
            std::process::exit(1);
        }
    }
}

fn get_password(cli: &Cli) -> Result<String> {
    if let Some(password) = &cli.password {
        return Ok(password.clone());
    }
    
    if let Ok(password) = std::env::var("DROPBOX_SYNC_PASSWORD") {
        return Ok(password);
    }
    
    println!("Enter password for token encryption:");
    let password = rpassword::read_password()?;
    Ok(password)
}