import requests
import json
import os

def test_dropbox_api():
    # Get token from environment variable
    token = os.getenv('TOKEN')
    if not token:
        print("Error: TOKEN environment variable not set")
        return
    
    headers = {
        'Authorization': f'Bearer {token}',
        'Content-Type': 'application/json'
    }
    
    # First API call - get latest cursor
    print("Step 1: Getting latest cursor...")
    cursor_data = {
        "include_deleted": False,
        "include_has_explicit_shared_members": False,
        "include_media_info": False,
        "include_mounted_folders": True,
        "include_non_downloadable_files": True,
        "path": "/blog",
        "recursive": False
    }
    
    try:
        response1 = requests.post(
            'https://api.dropboxapi.com/2/files/list_folder/get_latest_cursor',
            headers=headers,
            json=cursor_data
        )
        response1.raise_for_status()
        
        cursor_result = response1.json()
        print(f"Response 1: {json.dumps(cursor_result, indent=2)}")
        
        # Extract cursor from response
        cursor = cursor_result.get('cursor')
        if not cursor:
            print("Error: No cursor found in response")
            return
        
        print(f"Extracted cursor: {cursor}")

        # Wait for user input before proceeding
        input("Press Enter to continue to the next step...")
        
        # Second API call - continue with cursor
        print("\nStep 2: Using cursor to continue...")
        continue_data = {
            "cursor": cursor
        }
        
        response2 = requests.post(
            'https://api.dropboxapi.com/2/files/list_folder/continue',
            headers=headers,
            json=continue_data
        )
        response2.raise_for_status()
        
        continue_result = response2.json()
        print(f"Response 2: {json.dumps(continue_result, indent=2)}")
        
        print("\nTest completed successfully!")
        
    except requests.exceptions.RequestException as e:
        print(f"HTTP Error: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"Response status: {e.response.status_code}")
            print(f"Response body: {e.response.text}")
    except json.JSONDecodeError as e:
        print(f"JSON parsing error: {e}")
    except Exception as e:
        print(f"Unexpected error: {e}")

if __name__ == "__main__":
    test_dropbox_api()