package backend

import (
	"testing"
)

func TestFormatClassifications(t *testing.T) {
	// RAW
	for _, f := range []string{"test.CR2", "test.cr3", "photo.NEF", "test.ARW", "test.dng"} {
		if !IsRawFormat(f) {
			t.Errorf("expected %s to be RAW format", f)
		}
		if !IsPhotoFormat(f) {
			t.Errorf("expected RAW %s to also be considered a photo", f)
		}
	}

	// HEIC
	for _, f := range []string{"photo.heic", "photo.HEIF"} {
		if !IsHeicFormat(f) {
			t.Errorf("expected %s to be HEIC format", f)
		}
		if !IsPhotoFormat(f) {
			t.Errorf("expected %s to also be photo format", f)
		}
	}

	// GIF
	if !IsGifFormat("anim.gif") || !IsPhotoFormat("anim.gif") {
		t.Errorf("expected anim.gif to be GIF and Photo format")
	}

	// Standard Photos
	for _, f := range []string{"test.jpg", "test.jpeg", "test.png", "test.webp"} {
		if !IsPhotoFormat(f) {
			t.Errorf("expected %s to be photo format", f)
		}
		if IsRawFormat(f) || IsHeicFormat(f) || IsGifFormat(f) {
			t.Errorf("expected %s to NOT be RAW/HEIC/GIF", f)
		}
	}

	// Videos
	for _, f := range []string{"movie.mp4", "clip.mov", "video.mkv", "v.avi"} {
		if !IsVideoFormat(f) {
			t.Errorf("expected %s to be video format", f)
		}
		if IsPhotoFormat(f) {
			t.Errorf("expected %s to NOT be photo format", f)
		}
	}
}

func TestMatchesUploadFilters(t *testing.T) {
	baseOpts := DefaultPreferences.UploadOptions().normalized()

	// 1. Default should accept typical files
	if match, _ := MatchesUploadFilters("photo.jpg", 1024*100, baseOpts); !match {
		t.Errorf("expected photo.jpg to match under default options")
	}
	if match, _ := MatchesUploadFilters("video.mp4", 1024*1024*10, baseOpts); !match {
		t.Errorf("expected video.mp4 to match under default options")
	}

	// 2. Photos only
	photosOnlyOpts := baseOpts
	photosOnlyOpts.FilterIncludePhotos = true
	photosOnlyOpts.FilterIncludeVideos = false
	if match, _ := MatchesUploadFilters("photo.jpg", 1000, photosOnlyOpts); !match {
		t.Errorf("photos-only should allow photo.jpg")
	}
	if match, _ := MatchesUploadFilters("video.mp4", 1000, photosOnlyOpts); match {
		t.Errorf("photos-only should reject video.mp4")
	}

	// 3. Videos only
	videosOnlyOpts := baseOpts
	videosOnlyOpts.FilterIncludePhotos = false
	videosOnlyOpts.FilterIncludeVideos = true
	if match, _ := MatchesUploadFilters("photo.jpg", 1000, videosOnlyOpts); match {
		t.Errorf("videos-only should reject photo.jpg")
	}
	if match, _ := MatchesUploadFilters("video.mp4", 1000, videosOnlyOpts); !match {
		t.Errorf("videos-only should allow video.mp4")
	}

	// 4. Disable RAW
	noRawOpts := baseOpts
	noRawOpts.FilterIncludeRaw = false
	if match, _ := MatchesUploadFilters("shot.cr2", 1000, noRawOpts); match {
		t.Errorf("expected shot.cr2 to be rejected when FilterIncludeRaw is false")
	}
	if match, _ := MatchesUploadFilters("shot.jpg", 1000, noRawOpts); !match {
		t.Errorf("expected shot.jpg to still be accepted when FilterIncludeRaw is false")
	}

	// 5. Disable HEIC
	noHeicOpts := baseOpts
	noHeicOpts.FilterIncludeHeic = false
	if match, _ := MatchesUploadFilters("image.heic", 1000, noHeicOpts); match {
		t.Errorf("expected image.heic to be rejected when FilterIncludeHeic is false")
	}

	// 6. Disable GIF
	noGifOpts := baseOpts
	noGifOpts.FilterIncludeGif = false
	if match, _ := MatchesUploadFilters("banner.gif", 1000, noGifOpts); match {
		t.Errorf("expected banner.gif to be rejected when FilterIncludeGif is false")
	}

	// 7. Minimum file size filter
	minSizeOpts := baseOpts
	minSizeOpts.MinFileSizeKB = 50 // 50 KB
	if match, _ := MatchesUploadFilters("thumb.jpg", 30*1024, minSizeOpts); match {
		t.Errorf("expected 30 KB file to be rejected with MinFileSizeKB=50")
	}
	if match, _ := MatchesUploadFilters("thumb.jpg", 60*1024, minSizeOpts); !match {
		t.Errorf("expected 60 KB file to be accepted with MinFileSizeKB=50")
	}

	// 8. Maximum video size filter
	maxVideoOpts := baseOpts
	maxVideoOpts.MaxVideoSizeMB = 100 // 100 MB
	if match, _ := MatchesUploadFilters("movie.mp4", 150*1024*1024, maxVideoOpts); match {
		t.Errorf("expected 150 MB video to be rejected with MaxVideoSizeMB=100")
	}
	if match, _ := MatchesUploadFilters("movie.mp4", 80*1024*1024, maxVideoOpts); !match {
		t.Errorf("expected 80 MB video to be accepted with MaxVideoSizeMB=100")
	}
	// Max video size does not limit photos
	if match, _ := MatchesUploadFilters("huge_pano.jpg", 150*1024*1024, maxVideoOpts); !match {
		t.Errorf("expected huge photo to still be accepted with MaxVideoSizeMB=100")
	}
}
