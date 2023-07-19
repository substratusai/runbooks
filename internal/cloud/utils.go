package cloud

import (
	"fmt"
	"net/url"
	"strings"
)

func parseArtifactBucketURL(bucketURL string) (string, string, error) {
	u, err := url.Parse(bucketURL)
	if err != nil {
		return "", "", fmt.Errorf("parsing bucket url: %w", err)
	}

	bucket := u.Host
	subpath := strings.TrimPrefix(u.Path, "/")

	if bucket == "" {
		return "", "", fmt.Errorf("invalid artifact url: empty bucket: %s", bucketURL)
	}
	if subpath == "" {
		return "", "", fmt.Errorf("invalid artifact url: empty subpath: %s", bucketURL)
	}

	return bucket, subpath, nil
}
