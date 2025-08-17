use anyhow::{Context, Result};
use std::path::Path;
use std::process::Command;
use tokio::sync::mpsc;
use tracing::{info, warn, error, debug};

use crate::config::{Config, CopyRule};
use crate::dropbox_client::{DropboxClient, FileInfo};
use crate::webhook_server::SyncEvent;
use crate::content_hash;

pub struct SyncManager {
    config: Config,
    dropbox_client: DropboxClient,
    last_cursor: Option<String>,
}

impl SyncManager {
    pub fn new(config: Config, dropbox_client: DropboxClient) -> Self {
        let last_cursor = Self::load_cursor(&config.sync.local_base_path).ok();
        Self {
            config,
            dropbox_client,
            last_cursor,
        }
    }

    fn cursor_file_path(base_path: &str) -> std::path::PathBuf {
        Path::new(base_path).join(".blogsync_cursor")
    }

    fn load_cursor(base_path: &str) -> Result<String> {
        let cursor_file = Self::cursor_file_path(base_path);
        std::fs::read_to_string(cursor_file).context("Failed to read cursor file")
    }

    fn save_cursor(&self, cursor: &str) -> Result<()> {
        let cursor_file = Self::cursor_file_path(&self.config.sync.local_base_path);
        std::fs::write(cursor_file, cursor).context("Failed to write cursor file")
    }

    pub async fn start_sync_loop(&mut self, mut sync_receiver: mpsc::UnboundedReceiver<SyncEvent>) {
        info!("Starting sync manager loop");
        
        while let Some(event) = sync_receiver.recv().await {
            match event {
                SyncEvent::FilesChanged => {
                    info!("Files changed event received, starting incremental sync");
                    if let Err(e) = self.incremental_sync().await {
                        error!("Incremental sync failed: {}", e);
                    }
                }
                SyncEvent::FilesChangedWithCursor(cursor) => {
                    info!("Files changed event with cursor received, starting incremental sync");
                    if let Err(e) = self.incremental_sync_with_cursor(&cursor).await {
                        error!("Incremental sync with cursor failed: {}", e);
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

    async fn sync_files(&mut self) -> Result<()> {
        info!("Starting file synchronization");
        
        let base_path = Path::new(&self.config.sync.local_base_path);
        std::fs::create_dir_all(base_path)
            .context("Failed to create local base directory")?;

        let (files, new_cursor) = self
            .dropbox_client
            .list_folder(&self.config.sync.dropbox_folder, true)
            .await
            .context("Failed to list Dropbox folder")?;

        info!("Found {} files in Dropbox", files.len());

        //print the file and metadata
        for file in &files {
            debug!("File: {}, Size: {}, Modified: {}", 
                   file.path, file.size, file.modified);
        }


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

        if let Err(e) = self.save_cursor(&new_cursor) {
            warn!("Failed to save cursor: {}", e);
        } else {
            self.last_cursor = Some(new_cursor.clone());
            debug!("Saved cursor: {}", new_cursor);
        }

        info!("Full sync process completed successfully");
        Ok(())
    }

    async fn incremental_sync(&mut self) -> Result<()> {
        if let Some(cursor) = self.last_cursor.clone() {
            info!("Starting incremental sync from cursor");
            
            let changed_files = self
                .dropbox_client
                .get_changes_from_cursor(&cursor)
                .await
                .context("Failed to get changes from cursor")?;

            if changed_files.is_empty() {
                info!("No files changed since last sync");
                return Ok(());
            }

            info!("Found {} changed files", changed_files.len());
            
            let base_path = Path::new(&self.config.sync.local_base_path);
            
            for file in &changed_files {
                let relative_path = file.path.strip_prefix(&self.config.sync.dropbox_folder)
                    .unwrap_or(&file.path)
                    .trim_start_matches('/');
                
                let local_path = base_path.join(relative_path);
                
                debug!("Syncing changed file: {} -> {:?}", file.path, local_path);
                
                if let Err(e) = self.sync_single_file(file, &local_path).await {
                    warn!("Failed to sync changed file {}: {}", file.path, e);
                    continue;
                }
            }

            if let Err(e) = self.run_build_command().await {
                error!("Build command failed: {}", e);
                return Err(e);
            }

            if let Err(e) = self.apply_copy_rules().await {
                error!("Copy rules failed: {}", e);
                return Err(e);
            }

            info!("Incremental sync completed successfully");
        } else {
            warn!("No cursor available, falling back to full sync");
            self.sync_files().await?;
        }
        
        Ok(())
    }

    async fn incremental_sync_with_cursor(&self, cursor: &str) -> Result<()> {
        info!("Starting incremental sync with provided cursor");
        
        let changed_files = self
            .dropbox_client
            .get_changes_from_cursor(cursor)
            .await
            .context("Failed to get changes from cursor")?;

        if changed_files.is_empty() {
            info!("No files changed since provided cursor");
            return Ok(());
        }

        info!("Found {} changed files", changed_files.len());
        
        let base_path = Path::new(&self.config.sync.local_base_path);
        
        for file in &changed_files {
            let relative_path = file.path.strip_prefix(&self.config.sync.dropbox_folder)
                .unwrap_or(&file.path)
                .trim_start_matches('/');
            
            let local_path = base_path.join(relative_path);
            
            debug!("Syncing changed file: {} -> {:?}", file.path, local_path);
            
            if let Err(e) = self.sync_single_file(file, &local_path).await {
                warn!("Failed to sync changed file {}: {}", file.path, e);
                continue;
            }
        }

        if let Err(e) = self.run_build_command().await {
            error!("Build command failed: {}", e);
            return Err(e);
        }

        if let Err(e) = self.apply_copy_rules().await {
            error!("Copy rules failed: {}", e);
            return Err(e);
        }

        info!("Incremental sync with cursor completed successfully");
        Ok(())
    }

    async fn sync_single_file(&self, file_info: &FileInfo, local_path: &Path) -> Result<()> {
        if local_path.exists() {
            if let Some(dropbox_hash) = &file_info.content_hash {
                match content_hash::files_match(local_path, dropbox_hash) {
                    Ok(true) => {
                        debug!("File {} already up to date (hash match)", file_info.path);
                        return Ok(());
                    }
                    Ok(false) => {
                        debug!("File {} has different content hash, updating", file_info.path);
                    }
                    Err(e) => {
                        warn!("Failed to check content hash for {}: {}, falling back to size check", file_info.path, e);
                        let metadata = std::fs::metadata(local_path)?;
                        let local_size = metadata.len();
                        
                        if local_size == file_info.size {
                            debug!("File {} size matches, assuming up to date", file_info.path);
                            return Ok(());
                        }
                    }
                }
            } else {
                let metadata = std::fs::metadata(local_path)?;
                let local_size = metadata.len();
                
                if local_size == file_info.size {
                    debug!("File {} size matches and no hash available, assuming up to date", file_info.path);
                    return Ok(());
                }
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

        let working_dir = if working_dir.is_relative() {
            std::env::current_dir()
                .context("Failed to get current working directory")?
                .join(working_dir).canonicalize()
                .context("Failed to resolve working directory")?
        } else {
            working_dir.to_path_buf()
        };

        // print where we are
        let output = Command::new("sh")
            .args(["-c", "zola"])
            .current_dir(&working_dir)
            .output()
            .context("Failed to execute pwd")?;
        let pwd = String::from_utf8_lossy(&output.stdout);
        info!("Expected PWD directory: {}", pwd.trim());

        let parts: Vec<&str> = self.config.build.command.split_whitespace().collect();
        if parts.is_empty() {
            return Err(anyhow::anyhow!("Empty build command"));
        }

        // print command and args
        info!("Executing build command: {:?}", parts);
        // print working directory
        info!("Build working directory: {:?}", working_dir);
        // print pwd
        {
            let current_dir = std::env::current_dir()
                .context("Failed to get current working directory")?;
            info!("Current working directory: {:?}", current_dir);
        }
        // if parts.len() < 2 {
        //     return Err(anyhow::anyhow!("Invalid build command format"));
        // }

        // set command and args
        // ["-c", parts[0], ..parts[1..]]
        // prepend parts[0] with "-c"
        let mut command = vec!["-c"];
        command.extend_from_slice(&parts[0..]);

        info!("Running build command: {:?}", command);

        let output = Command::new("sh")
            .args(&command)
            .current_dir(working_dir)
            .output()
            .context("Failed to execute build command")?;

        let stdout = String::from_utf8_lossy(&output.stdout);
        info!("Build command completed successfully");
        info!("Build output: {}", stdout);

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            return Err(anyhow::anyhow!(
                "Build command failed with exit code {}: {}",
                output.status.code().unwrap_or(-1),
                stderr
            ));
        }

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