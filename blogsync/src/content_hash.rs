use digest::{Update, Digest};
use sha2::Sha256;
use std::path::Path;
use std::io::Read;

pub const BLOCK_SIZE: usize = 4 * 1024 * 1024;

/// Computes a hash using the same algorithm that the Dropbox API uses for the
/// the "content_hash" metadata field.
///
/// Implements the `digest::Digest` trait, whose `result()` function returns a
/// raw binary representation of the hash.  The "content_hash" field in the
/// Dropbox API is a hexadecimal-encoded version of this value.
///
/// Example:
///
/// ```
/// use dropbox_content_hasher::{DropboxContentHasher, hex};
///
/// let mut hasher = DropboxContentHasher::new();
/// let mut buf: [u8; 4096] = [0; 4096];
/// let mut f = std::fs::File::open("some-file").unwrap();
/// loop {
///     let len = f.read(&mut buf).unwrap();
///     if len == 0 { break; }
///     hasher.input(&buf[..len])
/// }
/// drop(f);
///
/// let hex_hash = format!("{:x}", hasher.result());
/// println!("{}", hex_hash);
/// ```

#[derive(Clone)]
pub struct DropboxContentHasher {
    overall_hasher: Sha256,
    block_hasher: Sha256,
    block_pos: usize,
}

impl DropboxContentHasher {
    pub fn new() -> Self {
        DropboxContentHasher {
            overall_hasher: Sha256::new(),
            block_hasher: Sha256::new(),
            block_pos: 0,
        }
    }

    pub fn update(&mut self, mut input: &[u8]) {
        while !input.is_empty() {
            if self.block_pos == BLOCK_SIZE {
                Update::update(&mut self.overall_hasher, self.block_hasher.finalize_reset().as_slice());
                self.block_pos = 0;
            }

            let space_in_block = BLOCK_SIZE - self.block_pos;
            let (head, rest) = input.split_at(std::cmp::min(input.len(), space_in_block));
            Update::update(&mut self.block_hasher, head);

            self.block_pos += head.len();
            input = rest;
        }
    }

    pub fn finalize(mut self) -> String {
        if self.block_pos > 0 {
            Update::update(&mut self.overall_hasher, self.block_hasher.finalize().as_slice());
        }
        format!("{:x}", self.overall_hasher.finalize())
    }
}

impl Default for DropboxContentHasher {
    fn default() -> Self { Self::new() }
}

pub fn hash_file(file_path: &Path) -> anyhow::Result<String> {
    let mut file = std::fs::File::open(file_path)?;
    let mut hasher = DropboxContentHasher::new();
    let mut buffer = [0u8; 65536];
    
    loop {
        let bytes_read = file.read(&mut buffer)?;
        if bytes_read == 0 {
            break;
        }
        hasher.update(&buffer[..bytes_read]);
    }
    
    Ok(hasher.finalize())
}

pub fn hash_bytes(data: &[u8]) -> String {
    let mut hasher = DropboxContentHasher::new();
    hasher.update(data);
    hasher.finalize()
}

pub fn files_match(file_path: &Path, dropbox_hash: &str) -> anyhow::Result<bool> {
    let local_hash = hash_file(file_path)?;
    Ok(local_hash == dropbox_hash)
}