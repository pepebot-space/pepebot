// Pepebot - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package providers

import (
	"mime"
	"path/filepath"
	"strings"
)

// FileType represents the category of a file
type FileType string

const (
	FileTypeText     FileType = "text"
	FileTypeImage    FileType = "image"
	FileTypeDocument FileType = "document"
	FileTypeAudio    FileType = "audio"
	FileTypeVideo    FileType = "video"
	FileTypeUnknown  FileType = "file"
)

// DetectFileType detects the file type from URL or file path
func DetectFileType(urlOrPath string) (FileType, string) {
	// Extract extension from URL or path
	ext := strings.ToLower(filepath.Ext(urlOrPath))
	if ext == "" {
		// Try to extract from URL query parameters
		if idx := strings.Index(urlOrPath, "?"); idx > 0 {
			ext = strings.ToLower(filepath.Ext(urlOrPath[:idx]))
		}
	}

	// Get MIME type from extension
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		// Fallback to common MIME types
		mimeType = getMimeTypeByExtension(ext)
	}

	// Determine file type category
	fileType := categorizeByMimeType(mimeType, ext)
	return fileType, mimeType
}

// getMimeTypeByExtension provides fallback MIME types for common extensions
func getMimeTypeByExtension(ext string) string {
	mimeTypes := map[string]string{
		// Images
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".bmp":  "image/bmp",
		".ico":  "image/x-icon",

		// Documents
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".txt":  "text/plain",
		".csv":  "text/csv",
		".rtf":  "application/rtf",
		".odt":  "application/vnd.oasis.opendocument.text",
		".ods":  "application/vnd.oasis.opendocument.spreadsheet",
		".odp":  "application/vnd.oasis.opendocument.presentation",

		// Audio
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".ogg":  "audio/ogg",
		".m4a":  "audio/m4a",
		".flac": "audio/flac",
		".aac":  "audio/aac",
		".wma":  "audio/x-ms-wma",
		".opus": "audio/opus",

		// Video
		".mp4":  "video/mp4",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".wmv":  "video/x-ms-wmv",
		".flv":  "video/x-flv",
		".webm": "video/webm",
		".mkv":  "video/x-matroska",
		".m4v":  "video/x-m4v",
		".3gp":  "video/3gpp",

		// Archives
		".zip": "application/zip",
		".rar": "application/x-rar-compressed",
		".7z":  "application/x-7z-compressed",
		".tar": "application/x-tar",
		".gz":  "application/gzip",

		// Code/Text
		".json": "application/json",
		".xml":  "application/xml",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".md":   "text/markdown",
	}

	if mimeType, ok := mimeTypes[ext]; ok {
		return mimeType
	}

	return "application/octet-stream"
}

// categorizeByMimeType categorizes file by MIME type prefix
func categorizeByMimeType(mimeType, ext string) FileType {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return FileTypeImage
	case strings.HasPrefix(mimeType, "audio/"):
		return FileTypeAudio
	case strings.HasPrefix(mimeType, "video/"):
		return FileTypeVideo
	case mimeType == "application/pdf":
		return FileTypeDocument
	case strings.Contains(mimeType, "word"):
		return FileTypeDocument
	case strings.Contains(mimeType, "excel"):
		return FileTypeDocument
	case strings.Contains(mimeType, "powerpoint"):
		return FileTypeDocument
	case strings.Contains(mimeType, "opendocument"):
		return FileTypeDocument
	case strings.HasPrefix(mimeType, "text/"):
		return FileTypeDocument
	default:
		return FileTypeUnknown
	}
}

// GetFileName extracts filename from URL or path
func GetFileName(urlOrPath string) string {
	// Remove query parameters
	if idx := strings.Index(urlOrPath, "?"); idx > 0 {
		urlOrPath = urlOrPath[:idx]
	}
	return filepath.Base(urlOrPath)
}
