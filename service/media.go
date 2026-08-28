package service

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"

	_ "golang.org/x/image/webp"
)

var imageTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

// StoreImage persists one verified raster image under the configured data
// volume. Callers choose only a fixed, code-owned area name; user filenames
// never become part of a filesystem path.
func StoreImage(reader io.Reader, area string) (string, error) {
	if area != "branding" && area != "avatars" {
		return "", errors.New("invalid media area")
	}
	limit := Config.Media.MaxImageBytes
	if limit == 0 {
		limit = 1 << 20
	}
	content, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}
	if int64(len(content)) == 0 || int64(len(content)) > limit {
		return "", errors.New("image is empty or exceeds the configured size limit")
	}
	contentType := http.DetectContentType(content)
	extension, ok := imageTypes[contentType]
	if !ok {
		return "", errors.New("only PNG, JPEG, and WebP images are accepted")
	}
	dimensions, _, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || dimensions.Width <= 0 || dimensions.Height <= 0 || dimensions.Width > 8192 || dimensions.Height > 8192 || int64(dimensions.Width)*int64(dimensions.Height) > 24_000_000 {
		return "", errors.New("image dimensions are invalid or exceed the safe limit")
	}
	randomName := make([]byte, 16)
	if _, err := rand.Read(randomName); err != nil {
		return "", fmt.Errorf("generate media name: %w", err)
	}
	directory := filepath.Join(Config.Media.Directory, area)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create media directory: %w", err)
	}
	directoryInfo, err := os.Lstat(directory)
	if err != nil || !directoryInfo.IsDir() || directoryInfo.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("media area must be a real directory")
	}
	name := hex.EncodeToString(randomName) + extension
	path := filepath.Join(directory, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("create media file: %w", err)
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write media file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close media file: %w", err)
	}
	return "/media/" + area + "/" + name, nil
}
