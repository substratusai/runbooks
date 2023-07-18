package cloud

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

func parseBucketURL(bucketURL string) (string, string, error) {
	u, err := url.Parse(bucketURL)
	if err != nil {
		return "", "", fmt.Errorf("parsing bucket url: %w", err)
	}

	bucket := u.Host
	subpath := strings.TrimPrefix(filepath.Dir(u.Path), "/")

	return bucket, subpath, nil
}
