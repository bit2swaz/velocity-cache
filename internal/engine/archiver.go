package engine

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func compress(outputs []string, targetZip string, packagePath string) (err error) {
	if len(outputs) == 0 {
		return errors.New("compress: no outputs provided")
	}

	// If a packagePath is provided, change into it for the duration of the operation.
	originalWd := ""
	if strings.TrimSpace(packagePath) != "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("compress: getwd: %w", err)
		}
		if err := os.Chdir(packagePath); err != nil {
			return fmt.Errorf("compress: chdir to %s: %w", packagePath, err)
		}
		originalWd = wd
		defer func() {
			if originalWd != "" {
				_ = os.Chdir(originalWd)
			}
		}()
	}

	absTarget, err := filepath.Abs(targetZip)
	if err != nil {
		return fmt.Errorf("compress: resolve target path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(absTarget), 0o755); err != nil {
		return fmt.Errorf("compress: ensure target directory: %w", err)
	}

	archiveFile, err := os.Create(absTarget)
	if err != nil {
		return fmt.Errorf("compress: create archive: %w", err)
	}
	defer func() {
		closeErr := archiveFile.Close()
		if err == nil && closeErr != nil {
			err = fmt.Errorf("compress: close archive file: %w", closeErr)
		}
	}()

	writer := zip.NewWriter(archiveFile)
	defer func() {
		closeErr := writer.Close()
		if err == nil && closeErr != nil {
			err = fmt.Errorf("compress: finalize archive: %w", closeErr)
		}
	}()

	seenBases := make(map[string]struct{}, len(outputs))

	for _, output := range outputs {
		cleaned := filepath.Clean(output)
		info, statErr := os.Stat(cleaned)
		if statErr != nil {
			return fmt.Errorf("compress: stat %s: %w", cleaned, statErr)
		}
		if !info.IsDir() {
			return fmt.Errorf("compress: %s is not a directory", cleaned)
		}

		base := filepath.Base(cleaned)
		if base == "." || base == string(filepath.Separator) {
			return fmt.Errorf("compress: invalid directory name %s", cleaned)
		}
		if _, ok := seenBases[base]; ok {
			return fmt.Errorf("compress: duplicate directory name %s", base)
		}
		seenBases[base] = struct{}{}

		walkErr := filepath.WalkDir(cleaned, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			absPath, absErr := filepath.Abs(path)
			if absErr != nil {
				return absErr
			}
			if absPath == absTarget {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}

			rel, relErr := filepath.Rel(cleaned, path)
			if relErr != nil {
				return relErr
			}

			archiveName := base
			if rel != "." {
				archiveName = filepath.Join(base, rel)
			}
			archiveName = filepath.ToSlash(archiveName)

			entryInfo, infoErr := d.Info()
			if infoErr != nil {
				return infoErr
			}

			if entryInfo.IsDir() {
				if !strings.HasSuffix(archiveName, "/") {
					archiveName += "/"
				}

				header, headerErr := zip.FileInfoHeader(entryInfo)
				if headerErr != nil {
					return headerErr
				}
				header.Name = archiveName
				_, createErr := writer.CreateHeader(header)
				return createErr
			}

			header, headerErr := zip.FileInfoHeader(entryInfo)
			if headerErr != nil {
				return headerErr
			}
			header.Name = archiveName
			header.Method = zip.Deflate

			archiveEntry, createErr := writer.CreateHeader(header)
			if createErr != nil {
				return createErr
			}

			file, openErr := os.Open(path)
			if openErr != nil {
				return openErr
			}

			_, copyErr := io.Copy(archiveEntry, file)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}

			return nil
		})
		if walkErr != nil {
			return walkErr
		}
	}

	return nil
}

func extract(sourceZip string, outputs []string, packagePath string) (err error) {
	if len(outputs) == 0 {
		return errors.New("extract: no outputs provided")
	}

	originalWd := ""
	if strings.TrimSpace(packagePath) != "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("extract: getwd: %w", err)
		}
		if err := os.Chdir(packagePath); err != nil {
			return fmt.Errorf("extract: chdir to %s: %w", packagePath, err)
		}
		originalWd = wd
		defer func() {
			if originalWd != "" {
				_ = os.Chdir(originalWd)
			}
		}()
	}

	// sourceZip is expected to be an absolute path (temporary file). Opening by absolute
	// path is safe even after chdir, but we still compute a cleaned path first.

	reader, err := zip.OpenReader(filepath.Clean(sourceZip))
	if err != nil {
		return fmt.Errorf("extract: open archive: %w", err)
	}
	defer func() {
		closeErr := reader.Close()
		if err == nil && closeErr != nil {
			err = fmt.Errorf("extract: close archive: %w", closeErr)
		}
	}()

	outputMap := make(map[string]string, len(outputs))

	for _, output := range outputs {
		cleaned := filepath.Clean(output)
		base := filepath.Base(cleaned)
		if base == "." || base == string(filepath.Separator) {
			return fmt.Errorf("extract: invalid directory name %s", cleaned)
		}
		if _, exists := outputMap[base]; exists {
			return fmt.Errorf("extract: duplicate directory name %s", base)
		}

		if err := os.RemoveAll(cleaned); err != nil {
			return fmt.Errorf("extract: clean %s: %w", cleaned, err)
		}
		if err := os.MkdirAll(cleaned, 0o755); err != nil {
			return fmt.Errorf("extract: ensure %s: %w", cleaned, err)
		}

		outputMap[base] = cleaned
	}

	for _, file := range reader.File {
		name := strings.ReplaceAll(file.Name, "\\", "/")
		if name == "" {
			continue
		}

		clean := path.Clean(name)
		if clean == "." {
			continue
		}
		if strings.HasPrefix(clean, "../") || clean == ".." || strings.HasPrefix(clean, "/") {
			return fmt.Errorf("extract: invalid path %s", file.Name)
		}

		parts := strings.SplitN(clean, "/", 2)
		top := parts[0]
		targetRoot, ok := outputMap[top]
		if !ok {
			return fmt.Errorf("extract: unexpected archive root %s", file.Name)
		}

		rel := ""
		if len(parts) == 2 {
			rel = parts[1]
		}

		targetPath := targetRoot
		if rel != "" {
			targetPath = filepath.Join(targetRoot, filepath.FromSlash(rel))
		}

		mode := file.Mode()
		if mode&os.ModeSymlink != 0 {
			if rel == "" {
				return fmt.Errorf("extract: invalid symlink %s", file.Name)
			}

			rc, openErr := file.Open()
			if openErr != nil {
				return fmt.Errorf("extract: open symlink %s: %w", file.Name, openErr)
			}

			linkTarget, readErr := io.ReadAll(rc)
			rc.Close()
			if readErr != nil {
				return fmt.Errorf("extract: read symlink %s: %w", file.Name, readErr)
			}

			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("extract: prepare symlink %s: %w", targetPath, err)
			}
			if err := os.Symlink(string(linkTarget), targetPath); err != nil {
				return fmt.Errorf("extract: create symlink %s: %w", targetPath, err)
			}
			continue
		}

		if mode.IsDir() || strings.HasSuffix(file.Name, "/") {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("extract: create directory %s: %w", targetPath, err)
			}
			if chmodErr := os.Chmod(targetPath, mode.Perm()); chmodErr != nil && !errors.Is(chmodErr, os.ErrPermission) {
				return fmt.Errorf("extract: chmod %s: %w", targetPath, chmodErr)
			}
			continue
		}

		if rel == "" {
			return fmt.Errorf("extract: unexpected file at root %s", file.Name)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("extract: prepare file %s: %w", targetPath, err)
		}

		rc, openErr := file.Open()
		if openErr != nil {
			return fmt.Errorf("extract: open file %s: %w", file.Name, openErr)
		}

		outFile, createErr := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
		if createErr != nil {
			rc.Close()
			return fmt.Errorf("extract: create file %s: %w", targetPath, createErr)
		}

		if _, copyErr := io.Copy(outFile, rc); copyErr != nil {
			rc.Close()
			outFile.Close()
			return fmt.Errorf("extract: write file %s: %w", targetPath, copyErr)
		}

		if closeErr := rc.Close(); closeErr != nil {
			outFile.Close()
			return fmt.Errorf("extract: close reader %s: %w", targetPath, closeErr)
		}
		if closeErr := outFile.Close(); closeErr != nil {
			return fmt.Errorf("extract: close file %s: %w", targetPath, closeErr)
		}
	}

	return nil
}

// compress/extract public wrappers accept packagePath and forward to internal functions.
func Compress(outputs []string, targetZip string, packagePath string) error {
	return compress(outputs, targetZip, packagePath)
}

func Extract(sourceZip string, outputs []string, packagePath string) error {
	return extract(sourceZip, outputs, packagePath)
}
