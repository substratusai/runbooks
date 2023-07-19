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

func (v *BucketURL) EnvDecode(val string) error {
	parsed, err := ParseBucketURL(val)
	if err != nil {
		return err
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
		return nil, fmt.Errorf("parsing bucket url: %w", err)
	}

	bucket := u.Host
	subpath := strings.TrimPrefix(u.Path, "/")

	if bucket == "" {
		return nil, fmt.Errorf("invalid artifact url: empty bucket: %s", bktURL)
	}

	return &BucketURL{
		Scheme: u.Scheme,
		Bucket: bucket,
		Path:   subpath,
	}, nil
}
