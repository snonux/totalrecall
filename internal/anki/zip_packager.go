package anki

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
)

// ZipPackager builds the final .apkg archive from a staging directory.
type ZipPackager struct{}

// NewZipPackager returns a ZipPackager.
func NewZipPackager() *ZipPackager {
	return &ZipPackager{}
}

// CreatePackage zips every file under tempDir into outputPath (Anki package layout).
func (*ZipPackager) CreatePackage(tempDir, outputPath string) error {
	zipFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = zipFile.Close()
	}()

	archive := zip.NewWriter(zipFile)
	defer func() {
		_ = archive.Close()
	}()

	return filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(tempDir, path)
		if err != nil {
			return err
		}

		writer, err := archive.Create(relPath)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			_ = file.Close()
		}()

		_, err = io.Copy(writer, file)
		return err
	})
}
