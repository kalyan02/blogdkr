use std::fs::File;
use std::io::Read;
use dropbox_sync::content_hash;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let file_path = "./milky-way-nasa.jpg";
    
    println!("Testing hasher implementation with milky-way-nasa.jpg");
    println!("Expected hash: 485291fa0ee50c016982abbfa943957bcd231aae0492ccbaa22c58e3997b35e0");
    
    // Test our implementation
    let computed_hash = content_hash::hash_file(std::path::Path::new(file_path))?;
    println!("Computed hash: {}", computed_hash);
    
    // Check if they match
    let expected = "485291fa0ee50c016982abbfa943957bcd231aae0492ccbaa22c58e3997b35e0";
    if computed_hash == expected {
        println!("✅ SUCCESS: Hashes match!");
    } else {
        println!("❌ FAILURE: Hashes do not match!");
    }
    
    // Let's also verify the file size
    let mut file = File::open(file_path)?;
    let mut buffer = Vec::new();
    file.read_to_end(&mut buffer)?;
    println!("File size: {} bytes (expected: 9,711,423)", buffer.len());
    
    // Let's manually verify the block structure
    println!("\nManual verification:");
    let expected_size = 9_711_423;
    if buffer.len() == expected_size {
        println!("✅ File size matches documentation");
    } else {
        println!("❌ File size mismatch: {} vs {}", buffer.len(), expected_size);
    }
    
    // Calculate block info
    let block_size = 4 * 1024 * 1024; // 4MB
    let num_full_blocks = buffer.len() / block_size;
    let remainder = buffer.len() % block_size;
    
    println!("Block structure:");
    println!("  Block size: {} bytes", block_size);
    println!("  Full blocks: {}", num_full_blocks);
    if remainder > 0 {
        println!("  Last block size: {} bytes", remainder);
    }
    
    Ok(())
}