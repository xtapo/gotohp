package backend

import (
	"path/filepath"
	"strings"
)

// Supported format categories for Google Photos
var (
	rawExtensions = map[string]bool{
		"cr2": true, "cr3": true, "nef": true, "arw": true, "orf": true,
		"raf": true, "rw2": true, "pef": true, "sr2": true, "dng": true,
	}

	heicExtensions = map[string]bool{
		"heic": true, "heif": true,
	}

	gifExtensions = map[string]bool{
		"gif": true,
	}

	standardPhotoExtensions = map[string]bool{
		"avif": true, "bmp": true, "ico": true,
		"jpg": true, "jpeg": true, "png": true,
		"tif": true, "tiff": true, "webp": true,
	}

	videoExtensions = map[string]bool{
		"3gp": true, "3g2": true, "asf": true, "avi": true, "divx": true,
		"m2t": true, "m2ts": true, "m4v": true, "mkv": true, "mmv": true,
		"mod": true, "mov": true, "mp4": true, "mpg": true, "mpeg": true,
		"mts": true, "tod": true, "wmv": true, "ts": true, "webm": true,
	}
)

func cleanExt(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if strings.HasPrefix(ext, ".") {
		return ext[1:]
	}
	return ext
}

// IsRawFormat checks if the file has a professional camera RAW extension.
func IsRawFormat(path string) bool {
	return rawExtensions[cleanExt(path)]
}

// IsHeicFormat checks if the file has an Apple HEIC/HEIF extension.
func IsHeicFormat(path string) bool {
	return heicExtensions[cleanExt(path)]
}

// IsGifFormat checks if the file has a GIF extension.
func IsGifFormat(path string) bool {
	return gifExtensions[cleanExt(path)]
}

// IsVideoFormat checks if the file is a recognized video format.
func IsVideoFormat(path string) bool {
	return videoExtensions[cleanExt(path)]
}

// IsPhotoFormat checks if the file is a photo format (standard, RAW, HEIC, or GIF).
func IsPhotoFormat(path string) bool {
	ext := cleanExt(path)
	return standardPhotoExtensions[ext] || rawExtensions[ext] || heicExtensions[ext] || gifExtensions[ext]
}

// MatchesUploadFilters checks whether a file matches the format and size criteria in UploadOptions.
// It returns true if the file should be included, or false with a descriptive reason if excluded.
func MatchesUploadFilters(path string, size int64, opts UploadOptions) (bool, string) {
	ext := cleanExt(path)
	if ext == "" {
		return false, "empty file extension"
	}

	// 1. Google Photos format check (if unsupported filter is active)
	if !opts.DisableUnsupportedFilesFilter && !isSupportedByGooglePhotos(path) {
		return false, "unsupported file format"
	}

	// 2. Minimum file size check (e.g. discard thumbnail cache or small icons)
	if opts.MinFileSizeKB > 0 {
		minBytes := int64(opts.MinFileSizeKB) * 1024
		if size >= 0 && size < minBytes {
			return false, "file size is smaller than minimum threshold"
		}
	}

	// 3. Media type and format specific filtering
	if IsVideoFormat(path) {
		if !opts.FilterIncludeVideos {
			return false, "video uploads disabled by filter"
		}
		// Maximum video size check (e.g. skip videos larger than 2 GB)
		if opts.MaxVideoSizeMB > 0 {
			maxBytes := int64(opts.MaxVideoSizeMB) * 1024 * 1024
			if size >= 0 && size > maxBytes {
				return false, "video exceeds maximum size threshold"
			}
		}
		return true, ""
	}

	if IsPhotoFormat(path) {
		if !opts.FilterIncludePhotos {
			return false, "photo uploads disabled by filter"
		}
		if IsRawFormat(path) && !opts.FilterIncludeRaw {
			return false, "RAW format excluded by filter"
		}
		if IsHeicFormat(path) && !opts.FilterIncludeHeic {
			return false, "HEIC format excluded by filter"
		}
		if IsGifFormat(path) && !opts.FilterIncludeGif {
			return false, "GIF format excluded by filter"
		}
		return true, ""
	}

	// If unsupported filter is disabled and extension isn't in our standard list, allow it
	return true, ""
}
