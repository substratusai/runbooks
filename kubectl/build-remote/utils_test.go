package main

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateMD5(t *testing.T) {
	content := []byte("Hello, World!")
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}
	expectedChecksum := fmt.Sprintf("%x", md5.Sum(content))
	actualChecksum, err := calculateMD5(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, expectedChecksum, actualChecksum)
}

func TestTarGz(t *testing.T) {
	// Setup a temporary directory with a temporary file
	dir, err := ioutil.TempDir("", "example")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir) // clean up
	tmpfile, err := ioutil.TempFile(dir, "example")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up
	if _, err := tmpfile.Write([]byte("Hello, World!")); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "test.tar.gz")
	err = tarGz(dir, dst)
	require.NoError(t, err)
	_, err = os.Stat(dst)
	require.NoError(t, err)
}

func TestFileExists(t *testing.T) {
	// Creating a temporary file
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up
	require.True(t, fileExists(tmpfile.Name()))
	require.False(t, fileExists(tmpfile.Name()+"not-exist"))
}

// TODO: Implement a mock HTTP server to test `uploadTarball` function.
