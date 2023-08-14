package cloud

import (
	"fmt"
	"net/url"
	"strings"
)

type BucketURL struct {
	Scheme string
	Bucket string
	Path   string
}

func (v *BucketURL) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return nil
	}

	parsed, err := ParseBucketURL(string(text))
	if err != nil {
		return fmt.Errorf("parsing bucket URL: %s: %w", string(text), err)
	}

	v.Scheme = parsed.Scheme
	v.Bucket = parsed.Bucket
	v.Path = parsed.Path
	return nil
}

func (b BucketURL) String() string {
	return fmt.Sprintf("%s://%s/%s", b.Scheme, b.Bucket, b.Path)
}

func ParseBucketURL(bktURL string) (*BucketURL, error) {
	u, err := url.Parse(bktURL)
	if err != nil {
		return nil, fmt.Errorf("parsing url: %w", err)
	}

	// NOTE: For local Kind clusters where URL is "tar:///bucket", u.Host will be empty.

	return &BucketURL{
		Scheme: u.Scheme,
		Bucket: u.Host,
		Path:   strings.TrimPrefix(u.Path, "/"),
	}, nil
}
