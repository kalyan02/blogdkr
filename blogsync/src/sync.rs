use anyhow::{Context, Result};
use std::path::Path;
use std::process::Command;
use tokio::sync::mpsc;
use tracing::{info, warn, error, debug};

use crate::config::{Config, CopyRule};
use crate::dropbox_client::{DropboxClient, FileInfo};
use crate::webhook_server::SyncEvent;

pub struct SyncManager {
    config: Config,
    dropbox_client: DropboxClient,
}

impl SyncManager {
    pub fn new(config: Config, dropbox_client: DropboxClient) -> Self {
        Self {
            config,
            dropbox_client,
        }
    }

    pub async fn start_sync_loop(&self, mut sync_receiver: mpsc::UnboundedReceiver<SyncEvent>) {
        info!("Starting sync manager loop");
        
        while let Some(event) = sync_receiver.recv().await {
            match event {
                SyncEvent::FilesChanged => {
                    info!("Files changed event received, starting sync");
                    if let Err(e) = self.sync_files().await {
                        error!("Sync failed: {}", e);
                    }
                }
                SyncEvent::ForceSync => {
                    info!("Force sync event received, starting full sync");
                    if let Err(e) = self.sync_files().await {
                        error!("Force sync failed: {}", e);
                    }
                }
            }
        }
    }

    async fn sync_files(&self) -> Result<()> {
        info!("Starting file synchronization");
        
        let base_path = Path::new(&self.config.sync.local_base_path);
        std::fs::create_dir_all(base_path)
            .context("Failed to create local base directory")?;

        let files = self
            .dropbox_client
            .list_folder(&self.config.sync.dropbox_folder, true)
            .await
            .context("Failed to list Dropbox folder")?;

        info!("Found {} files in Dropbox", files.len());

        // Build a set of all files that should exist locally
        let mut expected_files = std::collections::HashSet::new();
        
        // Download/update files from Dropbox
        for file in &files {
            let relative_path = file.path.strip_prefix(&self.config.sync.dropbox_folder)
                .unwrap_or(&file.path)
                .trim_start_matches('/');
            
            let local_path = base_path.join(relative_path);
            expected_files.insert(local_path.clone());
            
            debug!("Syncing file: {} -> {:?}", file.path, local_path);
            
            if let Err(e) = self.sync_single_file(&file, &local_path).await {
                warn!("Failed to sync file {}: {}", file.path, e);
                continue;
            }
        }

        // Remove local files that no longer exist in Dropbox
        if let Err(e) = self.remove_deleted_files(base_path, &expected_files).await {
            warn!("Failed to remove deleted files: {}", e);
        }

        info!("File synchronization completed");

        if let Err(e) = self.run_build_command().await {
            error!("Build command failed: {}", e);
            return Err(e);
        }

        if let Err(e) = self.apply_copy_rules().await {
            error!("Copy rules failed: {}", e);
            return Err(e);
        }

        info!("Full sync process completed successfully");
        Ok(())
    }

    async fn sync_single_file(&self, file_info: &FileInfo, local_path: &Path) -> Result<()> {
        if local_path.exists() {
            let metadata = std::fs::metadata(local_path)?;
            let local_size = metadata.len();
            
            if local_size == file_info.size {
                debug!("File {} already up to date", file_info.path);
                return Ok(());
            }
        }

        debug!("Downloading file: {}", file_info.path);
        self.dropbox_client
            .download_file(&file_info.path, local_path)
            .await
            .context("Failed to download file")?;

        info!("Downloaded: {}", file_info.path);
        Ok(())
    }

    async fn run_build_command(&self) -> Result<()> {
        info!("Running build command: {}", self.config.build.command);
        
        let working_dir = Path::new(&self.config.build.working_directory);
        if !working_dir.exists() {
            warn!("Build working directory does not exist: {:?}", working_dir);
            return Ok(());
        }

        let parts: Vec<&str> = self.config.build.command.split_whitespace().collect();
        if parts.is_empty() {
            return Err(anyhow::anyhow!("Empty build command"));
        }

        let output = Command::new(parts[0])
            .args(&parts[1..])
            .current_dir(working_dir)
            .output()
            .context("Failed to execute build command")?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(anyhow::anyhow!(
                "Build command failed with exit code {}: {}",
                output.status.code().unwrap_or(-1),
                stderr
            ));
        }

        let stdout = String::from_utf8_lossy(&output.stdout);
        info!("Build command completed successfully");
        debug!("Build output: {}", stdout);

        Ok(())
    }

    async fn apply_copy_rules(&self) -> Result<()> {
        info!("Applying copy rules");
        
        for rule in &self.config.copy_rules {
            if let Err(e) = self.apply_copy_rule(rule).await {
                warn!("Failed to apply copy rule {:?}: {}", rule, e);
            }
        }

        info!("Copy rules applied");
        Ok(())
    }

    async fn apply_copy_rule(&self, rule: &CopyRule) -> Result<()> {
        debug!("Applying copy rule: {:?}", rule);
        
        let dest_path = Path::new(&rule.destination);
        std::fs::create_dir_all(dest_path)
            .context("Failed to create destination directory")?;

        let recursive = rule.recursive.unwrap_or(false);
        
        let pattern_entries = glob::glob(&rule.source_pattern)
            .context("Failed to parse glob pattern")?;

        for entry in pattern_entries {
            let source_path = entry.context("Invalid glob entry")?;
            
            if source_path.is_file() {
                let file_name = source_path
                    .file_name()
                    .context("Failed to get file name")?;
                let dest_file = dest_path.join(file_name);
                
                std::fs::copy(&source_path, &dest_file)
                    .context("Failed to copy file")?;
                
                debug!("Copied: {:?} -> {:?}", source_path, dest_file);
            } else if source_path.is_dir() && recursive {
                self.copy_directory_recursive(&source_path, dest_path)?;
            }
        }

        Ok(())
    }

    fn copy_directory_recursive(&self, source: &Path, dest: &Path) -> Result<()> {
        if !source.is_dir() {
            return Err(anyhow::anyhow!("Source is not a directory"));
        }

        let entries = std::fs::read_dir(source)
            .context("Failed to read source directory")?;

        for entry in entries {
            let entry = entry.context("Failed to read directory entry")?;
            let source_path = entry.path();
            let file_name = entry.file_name();
            let dest_path = dest.join(file_name);

            if source_path.is_dir() {
                std::fs::create_dir_all(&dest_path)
                    .context("Failed to create destination subdirectory")?;
                self.copy_directory_recursive(&source_path, &dest_path)?;
            } else {
                std::fs::copy(&source_path, &dest_path)
                    .context("Failed to copy file")?;
            }
        }

        Ok(())
    }

    async fn remove_deleted_files(&self, base_path: &Path, expected_files: &std::collections::HashSet<std::path::PathBuf>) -> Result<()> {
        info!("Checking for deleted files to remove");
        
        let mut files_to_remove = Vec::new();
        self.collect_local_files(base_path, &mut files_to_remove)?;
        
        let mut removed_count = 0;
        for local_file in files_to_remove {
            if !expected_files.contains(&local_file) {
                info!("Removing deleted file: {:?}", local_file);
                if let Err(e) = std::fs::remove_file(&local_file) {
                    warn!("Failed to remove file {:?}: {}", local_file, e);
                } else {
                    removed_count += 1;
                }
            }
        }
        
        // Remove empty directories
        self.remove_empty_directories(base_path)?;
        
        if removed_count > 0 {
            info!("Removed {} deleted files", removed_count);
        }
        
        Ok(())
    }

    fn collect_local_files(&self, dir: &Path, files: &mut Vec<std::path::PathBuf>) -> Result<()> {
        if !dir.is_dir() {
            return Ok(());
        }

        let entries = std::fs::read_dir(dir)
            .context("Failed to read directory")?;

        for entry in entries {
            let entry = entry.context("Failed to read directory entry")?;
            let path = entry.path();
            
            if path.is_file() {
                files.push(path);
            } else if path.is_dir() {
                self.collect_local_files(&path, files)?;
            }
        }

        Ok(())
    }

    fn remove_empty_directories(&self, base_path: &Path) -> Result<()> {
        let mut dirs_to_check = Vec::new();
        self.collect_directories(base_path, &mut dirs_to_check)?;
        
        // Sort directories by depth (deepest first) to remove leaf directories first
        dirs_to_check.sort_by(|a, b| b.components().count().cmp(&a.components().count()));
        
        for dir in dirs_to_check {
            if dir != base_path {
                if let Ok(entries) = std::fs::read_dir(&dir) {
                    if entries.count() == 0 {
                        if let Err(e) = std::fs::remove_dir(&dir) {
                            debug!("Failed to remove empty directory {:?}: {}", dir, e);
                        } else {
                            debug!("Removed empty directory: {:?}", dir);
                        }
                    }
                }
            }
        }
        
        Ok(())
    }

    fn collect_directories(&self, dir: &Path, dirs: &mut Vec<std::path::PathBuf>) -> Result<()> {
        if !dir.is_dir() {
            return Ok(());
        }

        let entries = std::fs::read_dir(dir)
            .context("Failed to read directory")?;

        for entry in entries {
            let entry = entry.context("Failed to read directory entry")?;
            let path = entry.path();
            
            if path.is_dir() {
                dirs.push(path.clone());
                self.collect_directories(&path, dirs)?;
            }
        }

        Ok(())
    }
}