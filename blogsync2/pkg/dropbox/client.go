package dropbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Client struct {
	auth   *Auth
	client *http.Client
}

type FileInfo struct {
	Name         string    `json:"name"`
	Path         string    `json:"path_display"`
	Size         uint64    `json:"size"`
	Modified     time.Time `json:"server_modified"`
	ID           string    `json:"id,omitempty"`
	ContentHash  string    `json:"content_hash,omitempty"`
	IsDownloadable bool    `json:"is_downloadable,omitempty"`
}

type ListFolderRequest struct {
	Path                           string `json:"path"`
	Recursive                      bool   `json:"recursive"`
	IncludeMediaInfo              bool   `json:"include_media_info"`
	IncludeDeleted                bool   `json:"include_deleted"`
	IncludeHasExplicitSharedMembers bool   `json:"include_has_explicit_shared_members"`
}

type ListFolderContinueRequest struct {
	Cursor string `json:"cursor"`
}

type ListFolderResponse struct {
	Entries []struct {
		Tag            string    `json:".tag"`
		Name           string    `json:"name"`
		PathLower      string    `json:"path_lower,omitempty"`
		PathDisplay    string    `json:"path_display,omitempty"`
		ID             string    `json:"id,omitempty"`
		ClientModified time.Time `json:"client_modified,omitempty"`
		ServerModified time.Time `json:"server_modified,omitempty"`
		Size           uint64    `json:"size,omitempty"`
		ContentHash    string    `json:"content_hash,omitempty"`
		IsDownloadable bool      `json:"is_downloadable,omitempty"`
	} `json:"entries"`
	Cursor  string `json:"cursor"`
	HasMore bool   `json:"has_more"`
}

type DownloadRequest struct {
	Path string `json:"path"`
}

type DownloadZipRequest struct {
	Path string `json:"path"`
}

type UserInfo struct {
	Name struct {
		GivenName    string `json:"given_name"`
		Surname      string `json:"surname"`
		FamiliarName string `json:"familiar_name"`
		DisplayName  string `json:"display_name"`
	} `json:"name"`
	Email     string `json:"email"`
	AccountID string `json:"account_id"`
}

func NewClient(auth *Auth) *Client {
	return &Client{
		auth:   auth,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) ListFolder(folderPath string, recursive bool) ([]FileInfo, string, error) {
	accessToken, err := c.auth.GetValidAccessToken()
	if err != nil {
		return nil, "", err
	}

	path := folderPath
	if path == "/" {
		path = ""
	}

	request := ListFolderRequest{
		Path:                           path,
		Recursive:                      recursive,
		IncludeMediaInfo:              false,
		IncludeDeleted:                false,
		IncludeHasExplicitSharedMembers: false,
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.dropboxapi.com/2/files/list_folder", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to list folder: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("dropbox API error: %s", string(body))
	}

	var listResp ListFolderResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, "", fmt.Errorf("failed to decode response: %w", err)
	}

	var allFiles []FileInfo

	// Process initial entries
	for _, entry := range listResp.Entries {
		if entry.Tag == "file" {
			allFiles = append(allFiles, FileInfo{
				Name:         entry.Name,
				Path:         entry.PathDisplay,
				Size:         entry.Size,
				Modified:     entry.ServerModified,
				ID:           entry.ID,
				ContentHash:  entry.ContentHash,
				IsDownloadable: entry.IsDownloadable,
			})
		}
	}

	// Continue fetching if there are more entries
	for listResp.HasMore {
		var err error
		listResp, err = c.listFolderContinue(listResp.Cursor)
		if err != nil {
			return nil, "", fmt.Errorf("failed to continue listing: %w", err)
		}

		for _, entry := range listResp.Entries {
			if entry.Tag == "file" {
				allFiles = append(allFiles, FileInfo{
					Name:         entry.Name,
					Path:         entry.PathDisplay,
					Size:         entry.Size,
					Modified:     entry.ServerModified,
					ID:           entry.ID,
					ContentHash:  entry.ContentHash,
					IsDownloadable: entry.IsDownloadable,
				})
			}
		}
	}

	return allFiles, listResp.Cursor, nil
}

func (c *Client) listFolderContinue(cursor string) (ListFolderResponse, error) {
	accessToken, err := c.auth.GetValidAccessToken()
	if err != nil {
		return ListFolderResponse{}, err
	}

	request := ListFolderContinueRequest{
		Cursor: cursor,
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return ListFolderResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.dropboxapi.com/2/files/list_folder/continue", bytes.NewBuffer(reqBody))
	if err != nil {
		return ListFolderResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return ListFolderResponse{}, fmt.Errorf("failed to continue listing folder: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ListFolderResponse{}, fmt.Errorf("dropbox API error: %s", string(body))
	}

	var listResp ListFolderResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return ListFolderResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return listResp, nil
}

func (c *Client) GetChangesFromCursor(cursor string) ([]FileInfo, error) {
	continueResp, err := c.listFolderContinue(cursor)
	if err != nil {
		return nil, err
	}

	var allFiles []FileInfo
	currentResp := continueResp

	for {
		for _, entry := range currentResp.Entries {
			if entry.Tag == "file" {
				allFiles = append(allFiles, FileInfo{
					Name:         entry.Name,
					Path:         entry.PathDisplay,
					Size:         entry.Size,
					Modified:     entry.ServerModified,
					ID:           entry.ID,
					ContentHash:  entry.ContentHash,
					IsDownloadable: entry.IsDownloadable,
				})
			}
		}

		if !currentResp.HasMore {
			break
		}

		currentResp, err = c.listFolderContinue(currentResp.Cursor)
		if err != nil {
			return nil, err
		}
	}

	return allFiles, nil
}

func (c *Client) DownloadFile(dropboxPath, localPath string) error {
	accessToken, err := c.auth.GetValidAccessToken()
	if err != nil {
		return err
	}

	downloadReq := DownloadRequest{
		Path: dropboxPath,
	}

	reqHeader, err := json.Marshal(downloadReq)
	if err != nil {
		return fmt.Errorf("failed to marshal download request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://content.dropboxapi.com/2/files/download", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Dropbox-API-Arg", string(reqHeader))

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dropbox download error: %s", string(body))
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to file
	outFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (c *Client) DownloadZip(folderPath, localZipPath string) error {
	accessToken, err := c.auth.GetValidAccessToken()
	if err != nil {
		return err
	}

	path := folderPath
	if path == "/" || path == "" {
		path = ""
	}

	downloadReq := DownloadZipRequest{
		Path: path,
	}

	reqHeader, err := json.Marshal(downloadReq)
	if err != nil {
		return fmt.Errorf("failed to marshal download zip request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://content.dropboxapi.com/2/files/download_zip", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Dropbox-API-Arg", string(reqHeader))

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download zip: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dropbox download zip error: %s", string(body))
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(localZipPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to file
	outFile, err := os.Create(localZipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write zip file: %w", err)
	}

	return nil
}

func (c *Client) GetCurrentAccount() (*UserInfo, error) {
	accessToken, err := c.auth.GetValidAccessToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://api.dropboxapi.com/2/users/get_current_account", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get current account: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("dropbox API error: %s", string(body))
	}

	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &userInfo, nil
}