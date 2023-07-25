package upload

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func calculateMD5(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func tarGz(src, dst string) error {
	tarFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create tarFile: %w", err)
	}
	defer tarFile.Close()

	gzWriter := gzip.NewWriter(tarFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// TODO(bjb): #121 read .dockerignore if it exists, exclude those files
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk the tempdir path: %w", err)
		}

		// Skip the root directory
		if path == src {
			return nil
		}

		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return fmt.Errorf("failed to read file headers: %w", err)
		}

		// Use relative filepath to ensure the root directory is not included in tarball
		relativePath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to determine relative path: %w", err)
		}

		// clean up the file name to avoid including preceding "./" or "/"
		header.Name = strings.TrimPrefix(relativePath, string(filepath.Separator))

		// Add "/workspace" to the beginning of the header.Name
		header.Name = filepath.Join("workspace", header.Name)

		// Skip if it is not a regular file or a directory
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to prepare a tarfile header: %w", err)
		}

		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file during compression: %w", err)
			}
			defer file.Close()
			if _, err := io.Copy(tarWriter, file); err != nil {
				return fmt.Errorf("failed to copy file contents: %w", err)
			}
		}

		return nil
	})
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func uploadTarball(tarPath, url, encodedMd5Checksum string) error {
	file, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("tar upload: %w", err)
	}
	defer file.Close()

	req, err := http.NewRequest(http.MethodPut, url, file)
	if err != nil {
		return fmt.Errorf("tar upload: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-MD5", encodedMd5Checksum)

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("tar upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}
	fmt.Println("successfully uploaded tarball")
	return nil
}
