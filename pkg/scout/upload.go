package scout

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// UploadSink identifies a cloud storage destination.
type UploadSink string

const (
	SinkGoogleDrive UploadSink = "gdrive"
	SinkOneDrive    UploadSink = "onedrive"
)

// UploadConfig holds credentials and destination for cloud upload.
type UploadConfig struct {
	Sink       UploadSink `json:"sink"`
	Token      *oauth2.Token `json:"token,omitempty"`
	FolderID   string     `json:"folder_id,omitempty"`   // GDrive folder ID or OneDrive folder path
	FolderPath string     `json:"folder_path,omitempty"` // OneDrive folder path
}

// UploadResult describes the outcome of a cloud upload.
type UploadResult struct {
	Sink     UploadSink `json:"sink"`
	FileID   string     `json:"file_id,omitempty"`
	FileName string     `json:"file_name"`
	URL      string     `json:"url,omitempty"`
	Size     int64      `json:"size"`
}

// Uploader uploads files to cloud storage.
type Uploader struct {
	config *UploadConfig
	client *http.Client
}

// NewUploader creates a cloud uploader with the given config.
// The OAuth2 token must already be obtained (see UploadOAuthConfig for helpers).
func NewUploader(cfg *UploadConfig) *Uploader {
	client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(cfg.Token))

	return &Uploader{
		config: cfg,
		client: client,
	}
}

// Upload sends data to the configured cloud sink.
func (u *Uploader) Upload(ctx context.Context, filename string, data []byte) (*UploadResult, error) {
	switch u.config.Sink {
	case SinkGoogleDrive:
		return u.uploadGDrive(ctx, filename, data)
	case SinkOneDrive:
		return u.uploadOneDrive(ctx, filename, data)
	default:
		return nil, fmt.Errorf("scout: upload: unknown sink: %s", u.config.Sink)
	}
}

// UploadFile reads a file from disk and uploads it.
func (u *Uploader) UploadFile(ctx context.Context, path string) (*UploadResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scout: upload: read file: %w", err)
	}

	return u.Upload(ctx, filepath.Base(path), data)
}

// uploadGDrive uploads to Google Drive using the simple upload API.
func (u *Uploader) uploadGDrive(ctx context.Context, filename string, data []byte) (*UploadResult, error) {
	// Use multipart upload to set metadata + content in one request.
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Metadata part.
	metaHeader := fmt.Sprintf(`{"name": %q`, filename)
	if u.config.FolderID != "" {
		metaHeader += fmt.Sprintf(`, "parents": [%q]`, u.config.FolderID)
	}

	metaHeader += "}"

	metaPart, err := writer.CreatePart(map[string][]string{
		"Content-Type": {"application/json; charset=UTF-8"},
	})
	if err != nil {
		return nil, fmt.Errorf("scout: gdrive: create meta part: %w", err)
	}

	if _, err := metaPart.Write([]byte(metaHeader)); err != nil {
		return nil, fmt.Errorf("scout: gdrive: write meta: %w", err)
	}

	// Content part.
	contentType := detectContentType(filename)

	filePart, err := writer.CreatePart(map[string][]string{
		"Content-Type": {contentType},
	})
	if err != nil {
		return nil, fmt.Errorf("scout: gdrive: create file part: %w", err)
	}

	if _, err := filePart.Write(data); err != nil {
		return nil, fmt.Errorf("scout: gdrive: write data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("scout: gdrive: close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://www.googleapis.com/upload/drive/v3/files?uploadType=multipart",
		&body)
	if err != nil {
		return nil, fmt.Errorf("scout: gdrive: create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scout: gdrive: upload: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("scout: gdrive: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var gResp struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&gResp); err != nil {
		return nil, fmt.Errorf("scout: gdrive: decode response: %w", err)
	}

	return &UploadResult{
		Sink:     SinkGoogleDrive,
		FileID:   gResp.ID,
		FileName: gResp.Name,
		URL:      fmt.Sprintf("https://drive.google.com/file/d/%s", gResp.ID),
		Size:     int64(len(data)),
	}, nil
}

// uploadOneDrive uploads to OneDrive using the simple upload API.
func (u *Uploader) uploadOneDrive(ctx context.Context, filename string, data []byte) (*UploadResult, error) {
	folderPath := u.config.FolderPath
	if folderPath == "" {
		folderPath = "root"
	}

	// PUT to /me/drive/items/root:/{path}/{filename}:/content
	uploadURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/%s:/%s:/content",
		folderPath, filename)

	if folderPath == "root" {
		uploadURL = fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s:/content", filename)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("scout: onedrive: create request: %w", err)
	}

	req.Header.Set("Content-Type", detectContentType(filename))

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scout: onedrive: upload: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("scout: onedrive: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var odResp struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		WebURL  string `json:"webUrl"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&odResp); err != nil {
		return nil, fmt.Errorf("scout: onedrive: decode response: %w", err)
	}

	return &UploadResult{
		Sink:     SinkOneDrive,
		FileID:   odResp.ID,
		FileName: odResp.Name,
		URL:      odResp.WebURL,
		Size:     int64(len(data)),
	}, nil
}

// UploadOAuthConfig returns OAuth2 configs for supported sinks.
func UploadOAuthConfig(sink UploadSink, clientID, clientSecret, redirectURL string) *oauth2.Config {
	switch sink {
	case SinkGoogleDrive:
		return &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"https://www.googleapis.com/auth/drive.file"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
				TokenURL: "https://oauth2.googleapis.com/token",
			},
		}
	case SinkOneDrive:
		return &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"Files.ReadWrite", "offline_access"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
				TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
			},
		}
	default:
		return nil
	}
}

// SaveUploadConfig persists upload config to ~/.scout/upload.json.
func SaveUploadConfig(cfg *UploadConfig) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("scout: upload config: %w", err)
	}

	dir := filepath.Join(home, ".scout")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("scout: upload config: mkdir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: upload config: marshal: %w", err)
	}

	return os.WriteFile(filepath.Join(dir, "upload.json"), data, 0o600)
}

// LoadUploadConfig reads upload config from ~/.scout/upload.json.
func LoadUploadConfig() (*UploadConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("scout: upload config: %w", err)
	}

	data, err := os.ReadFile(filepath.Join(home, ".scout", "upload.json"))
	if err != nil {
		return nil, err
	}

	var cfg UploadConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("scout: upload config: parse: %w", err)
	}

	return &cfg, nil
}

func detectContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		return "application/json"
	case ".har":
		return "application/json"
	case ".html", ".htm":
		return "text/html"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".pdf":
		return "application/pdf"
	case ".md":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

// TokenExpiry returns the token expiry time, or zero if not set.
func (u *Uploader) TokenExpiry() time.Time {
	if u.config.Token == nil {
		return time.Time{}
	}

	return u.config.Token.Expiry
}
