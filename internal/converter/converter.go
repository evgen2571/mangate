package converter

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/evgen2571/mangate/internal/config"
	"github.com/evgen2571/mangate/internal/util"
)

type Converter struct {
	cfg config.Config
}

func New(cfg config.Config) *Converter {
	return &Converter{cfg: cfg}
}

func (c *Converter) ConvertChapter(sourceDir, mangaDirName, chapterName string) error {
	if strings.TrimSpace(sourceDir) == "" {
		return fmt.Errorf("convert chapter: source directory cannot be empty")
	}
	if strings.TrimSpace(mangaDirName) == "" {
		return fmt.Errorf("convert chapter: manga directory name cannot be empty")
	}
	if strings.TrimSpace(chapterName) == "" {
		return fmt.Errorf("convert chapter: chapter name cannot be empty")
	}

	files, err := chapterFiles(sourceDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("convert chapter %q: no files found in %q", chapterName, sourceDir)
	}

	switch c.cfg.Download.Type {
	case config.DownloadTypePlain:
		err = c.convertPlainChapter(sourceDir, mangaDirName, chapterName, files)
	case config.DownloadTypeCBZ, config.DownloadTypeZIP:
		err = c.convertArchiveChapter(sourceDir, mangaDirName, chapterName, files)
	default:
		err = fmt.Errorf("convert chapter %q: unsupported download type %q", chapterName, c.cfg.Download.Type)
	}
	if err != nil {
		return err
	}

	if err := os.RemoveAll(sourceDir); err != nil {
		return fmt.Errorf("remove converted source directory %q: %w", sourceDir, err)
	}
	return nil
}

func (c *Converter) convertPlainChapter(sourceDir, mangaDirName, chapterName string, files []fs.DirEntry) error {
	mangaDir := filepath.Join(c.cfg.Download.Dir, mangaDirName)
	chapterDir := filepath.Join(mangaDir, chapterName)
	if err := util.EnsureDir(mangaDir, "manga directory"); err != nil {
		return err
	}

	stagingDir, err := os.MkdirTemp(mangaDir, chapterName+"-staging-*")
	if err != nil {
		return fmt.Errorf("create staging chapter directory for %q: %w", chapterName, err)
	}
	defer func() {
		_ = os.RemoveAll(stagingDir)
	}()

	for _, file := range files {
		sourcePath := filepath.Join(sourceDir, file.Name())
		targetPath := filepath.Join(stagingDir, file.Name())
		if err := copyFileAtomic(sourcePath, targetPath); err != nil {
			return err
		}
	}

	if err := replacePath(stagingDir, chapterDir); err != nil {
		return fmt.Errorf("replace chapter directory %q: %w", chapterDir, err)
	}

	return nil
}

func (c *Converter) convertArchiveChapter(sourceDir, mangaDirName, chapterName string, files []fs.DirEntry) error {
	targetDir := filepath.Join(c.cfg.Download.Dir, mangaDirName)
	if err := util.EnsureDir(targetDir, "manga directory"); err != nil {
		return err
	}

	archivePath := filepath.Join(targetDir, chapterName+"."+c.cfg.Download.Type)
	tempFile, err := os.CreateTemp(targetDir, chapterName+"-*."+c.cfg.Download.Type)
	if err != nil {
		return fmt.Errorf("create temporary archive for %q: %w", chapterName, err)
	}
	tempPath := tempFile.Name()

	zw := zip.NewWriter(tempFile)
	for _, file := range files {
		sourcePath := filepath.Join(sourceDir, file.Name())
		entry, err := zw.Create(file.Name())
		if err != nil {
			_ = zw.Close()
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
			return fmt.Errorf("create archive entry %q: %w", file.Name(), err)
		}

		in, err := os.Open(sourcePath)
		if err != nil {
			_ = zw.Close()
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
			return fmt.Errorf("open source file %q: %w", sourcePath, err)
		}
		if _, err := io.Copy(entry, in); err != nil {
			_ = in.Close()
			_ = zw.Close()
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
			return fmt.Errorf("write archive entry %q: %w", file.Name(), err)
		}
		if err := in.Close(); err != nil {
			_ = zw.Close()
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
			return fmt.Errorf("close source file %q: %w", sourcePath, err)
		}
	}

	if err := zw.Close(); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("close archive %q: %w", tempPath, err)
	}
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close temporary archive %q: %w", tempPath, err)
	}
	if err := replacePath(tempPath, archivePath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace archive %q: %w", archivePath, err)
	}

	return nil
}

func chapterFiles(sourceDir string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("read source directory %q: %w", sourceDir, err)
	}

	files := make([]fs.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, entry)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	return files, nil
}

func copyFileAtomic(sourcePath, targetPath string) error {
	in, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file %q: %w", sourcePath, err)
	}
	defer func() {
		_ = in.Close()
	}()

	out, tempPath, err := createTempFile(targetPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("copy %q to %q: %w", sourcePath, targetPath, err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close copied file %q: %w", targetPath, err)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("move copied file %q to %q: %w", tempPath, targetPath, err)
	}

	return nil
}

func createTempFile(targetPath string) (*os.File, string, error) {
	dir := filepath.Dir(targetPath)
	if err := util.EnsureDir(dir, "output directory"); err != nil {
		return nil, "", err
	}

	tempFile, err := os.CreateTemp(dir, filepath.Base(targetPath)+"-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temporary file for %q: %w", targetPath, err)
	}
	return tempFile, tempFile.Name(), nil
}

func replacePath(sourcePath, targetPath string) error {
	backupPath := targetPath + ".backup"
	if _, err := os.Stat(targetPath); err == nil {
		_ = os.RemoveAll(backupPath)
		if err := os.Rename(targetPath, backupPath); err != nil {
			return fmt.Errorf("move existing path %q to backup: %w", targetPath, err)
		}
		if err := os.Rename(sourcePath, targetPath); err != nil {
			_ = os.Rename(backupPath, targetPath)
			return fmt.Errorf("move new path %q to %q: %w", sourcePath, targetPath, err)
		}
		_ = os.RemoveAll(backupPath)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat existing path %q: %w", targetPath, err)
	}

	if err := os.Rename(sourcePath, targetPath); err != nil {
		return fmt.Errorf("move new path %q to %q: %w", sourcePath, targetPath, err)
	}
	return nil
}
