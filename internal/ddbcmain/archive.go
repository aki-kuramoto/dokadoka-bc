package ddbcmain

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ArchiveEntry struct {
	SrcPath  string
	DestName string
}

// MakeArchive creates an archive ("tar.gz" or "zip") containing the specified entries.
// It will write the output to outputPath. The existing outputPath will be overwritten.
func MakeArchive(outputPath string, format string, entries []ArchiveEntry) error {
	switch format {
	case "tar.gz":
		return makeTarGz(outputPath, entries)
	case "zip":
		return makeZip(outputPath, entries)
	default:
		return fmt.Errorf("unsupported archive format: %s", format)
	}
}

func makeTarGz(outputPath string, entries []ArchiveEntry) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", outputPath, err)
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, entry := range entries {
		err := appendToTar(tw, entry.SrcPath, entry.DestName)
		if err != nil {
			return err
		}
	}
	return nil
}

func appendToTar(tw *tar.Writer, srcPath, destName string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", srcPath, err)
	}

	if info.Mode().IsDir() {
		return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			relPath, err := filepath.Rel(srcPath, path)
			if err != nil {
				return err
			}

			arcPath := filepath.ToSlash(destName)
			if relPath != "." {
				arcPath = filepath.ToSlash(filepath.Join(destName, relPath))
			}

			return writeTarEntry(tw, path, arcPath, info)
		})
	}

	// File
	return writeTarEntry(tw, srcPath, filepath.ToSlash(destName), info)
}

func writeTarEntry(tw *tar.Writer, srcPath, arcPath string, info os.FileInfo) error {
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}
	header.Name = arcPath

	// Force mode just in case (e.g. built on Windows) for execution files if needed?
	// But `info` handles basic executable flags if built properly locally.
	// We'll trust os.FileInfo unless we need forced overrides later.
	// For now let's just write the header.
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return nil
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(tw, f)
	return err
}

func makeZip(outputPath string, entries []ArchiveEntry) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", outputPath, err)
	}
	defer outFile.Close()

	zw := zip.NewWriter(outFile)
	defer zw.Close()

	for _, entry := range entries {
		err := appendToZip(zw, entry.SrcPath, entry.DestName)
		if err != nil {
			return err
		}
	}
	return nil
}

func appendToZip(zw *zip.Writer, srcPath, destName string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", srcPath, err)
	}

	if info.Mode().IsDir() {
		return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			relPath, err := filepath.Rel(srcPath, path)
			if err != nil {
				return err
			}

			arcPath := filepath.ToSlash(destName)
			if relPath != "." {
				arcPath = filepath.ToSlash(filepath.Join(destName, relPath))
			}

			return writeZipEntry(zw, path, arcPath, info)
		})
	}

	// File
	return writeZipEntry(zw, srcPath, filepath.ToSlash(destName), info)
}

func writeZipEntry(zw *zip.Writer, srcPath, arcPath string, info os.FileInfo) error {
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = arcPath

	// Ensure Windows builds keep zip execution permissions logic.
	// Setting the higher 16 bits of CreatorVersion defines it as a Unix permission set,
	// allowing `unzip` to restore the execution flags gracefully.
	header.SetMode(info.Mode())
	
	if info.Mode().IsDir() && !strings.HasSuffix(arcPath, "/") {
		header.Name += "/"
	}

	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return nil
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(writer, f)
	return err
}
