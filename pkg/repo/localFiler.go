package repo

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"marketplace-yaga/pkg/logger"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/spf13/afero"
	"go.uber.org/zap"
)

type LocalFiler struct {
	ctx      context.Context
	fs       afero.Fs
	root     string
	filename string
	versions []string
}

const checksumPostfix = "sha256"

func NewFiler(ctx context.Context, root, filename string, fs afero.Fs) (*LocalFiler, error) {
	if ctx == nil {
		return nil, ErrNilCtx
	}

	if fs == nil {
		return nil, ErrNilFs
	}

	if root == "" {
		return nil, ErrEmptyRoot
	}

	if filename == "" {
		return nil, ErrEmptyFilename
	}

	l := LocalFiler{
		ctx:      ctx,
		fs:       fs,
		root:     root,
		filename: filename,
	}

	return &l, nil
}

const defaultPerms os.FileMode = 0770

func (l *LocalFiler) Init() error {
	err := l.fs.MkdirAll(l.root, defaultPerms)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "ensure directory exist", zap.String("path", l.root))
		return err
	}

	err = l.load()
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "load repository")
		return err
	}

	return nil
}

const numVersionsToCache = 5

func (l *LocalFiler) load() error {
	entries, err := afero.ReadDir(l.fs, l.root)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "read directory contents", zap.String("path", l.root))
		return err
	}

	l.versions = nil
	for _, f := range entries {
		n := f.Name()

		err = l.validateVersion(n)
		if err != nil {
			logger.ErrorCtx(l.ctx, err, "validate version", zap.String("version", n))
			p := filepath.Join(l.root, n)

			err = l.fs.RemoveAll(p)
			if err != nil {
				logger.ErrorCtx(l.ctx, err, "remove corrupted version",
					zap.String("version", n),
					zap.String("path", p))
				return err
			}
		}

		l.versions = append(l.versions, n)
	}
	l.sortVersions()

	if len(l.versions) > numVersionsToCache {
		oldest := l.versions[len(l.versions)-1]
		err = l.Remove(oldest)
		if err != nil {
			logger.ErrorCtx(l.ctx, err, "remove oldest version",
				zap.String("version", oldest),
				zap.Int("versions to cache", numVersionsToCache))
			return err
		}
	}

	return nil
}

func (l *LocalFiler) sortVersions() {
	logger.DebugCtx(l.ctx, nil, "sort versions", zap.Strings("versions before", l.versions))
	if len(l.versions) > 1 {
		sort.Slice(l.versions, func(i, j int) bool {
			vi, _ := semver.Parse(l.versions[i])
			vj, _ := semver.Parse(l.versions[j])

			return vi.GT(vj)
		})
	}
	logger.DebugCtx(l.ctx, nil, "sort versions", zap.Strings("versions after", l.versions))
}

func (l *LocalFiler) validateVersion(version string) error {
	_, err := semver.Parse(version)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "parse semver", zap.String("version", version))
		return err
	}

	vp := filepath.Join(l.root, version)
	err = l.validateDirectory(vp)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "validate directory", zap.String("path", vp))
		return err
	}

	fp := filepath.Join(vp, l.filename)
	err = l.validateFile(fp)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "validate file", zap.String("path", fp))
		return err
	}

	cp := joinWithDots(fp, checksumPostfix)
	err = l.validateFile(cp)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "validate guest agent executable", zap.String("path", cp))
		return err
	}

	err = l.validateFilehash(fp)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "validate guest agent executable filehash", zap.String("path", fp))
		return err
	}

	return nil
}

func (l *LocalFiler) List() []string {
	return l.versions
}

func (l *LocalFiler) Get(version string) string {
	for _, v := range l.versions {
		if v == version {
			return filepath.Join(l.root, version, l.filename)
		}
	}

	return ""
}

// Add - copies provided file alongside with it hash-file.
func (l *LocalFiler) Add(path, version string) error {
	v := l.Get(version)
	logger.DebugCtx(l.ctx, nil, "get version", zap.String("version", version))
	if v != "" {
		return ErrAlreadyAdded
	}

	err := l.validateFile(path)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "validate file", zap.String("path", path))
		return err
	}

	checksumPath := joinWithDots(path, checksumPostfix)
	err = l.validateFile(checksumPath)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "validate file", zap.String("path", checksumPath))
		return err
	}

	err = l.validateFilehash(path)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "validate filehash", zap.String("path", path))
		return err
	}

	// create version dir before copy
	vd := filepath.Join(l.root, version)
	err = l.fs.MkdirAll(vd, defaultPerms)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "create guest agent directory",
			zap.String("directory path", vd))
		return err
	}

	vp := filepath.Join(vd, l.filename)
	err = l.copy(vp, path)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "copy file", zap.String("from", path), zap.String("to", vp))
		return err
	}

	c := joinWithDots(l.filename, checksumPostfix)
	cp := filepath.Join(l.root, version, c)
	err = l.copy(cp, checksumPath)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "copy file", zap.String("from", checksumPath), zap.String("to", cp))
	}

	l.versions = append(l.versions, version)
	l.sortVersions()

	return nil
}

func (l *LocalFiler) Remove(version string) error {
	v := l.Get(version)
	logger.DebugCtx(l.ctx, nil, "get version", zap.String("version", version))
	if v == "" {
		return nil
	}

	vp := filepath.Join(l.root, version)
	err := l.fs.RemoveAll(vp)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "remove version", zap.String("path", vp))
		return err
	}

	err = l.load()
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "reload filerepo", zap.Strings("versions", l.versions))
		return err
	}

	return nil
}

func (l *LocalFiler) validateExist(path string) error {
	if path == "" {
		return ErrNotFound
	}

	e, err := afero.Exists(l.fs, path)
	if err != nil {
		return err
	}
	if !e {
		return ErrNotFound
	}

	return nil
}

func (l *LocalFiler) validateDirectory(path string) error {
	err := l.validateExist(path)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "exists", zap.String("path", path))
		return err
	}

	d, err := afero.IsDir(l.fs, path)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "check if it is a directory", zap.String("path", path))
		return err
	}
	if !d {
		return ErrNotDir
	}

	return nil
}

func (l *LocalFiler) validateFile(path string) error {
	err := l.validateExist(path)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "exists", zap.String("path", path))
		return err
	}

	d, err := afero.IsDir(l.fs, path)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "check if it is a file", zap.String("path", path))
		return err
	}
	if d {
		return ErrNotFile
	}

	return nil
}

func (l *LocalFiler) validateFilehash(path string) error {
	hash, err := l.getFilehash(path)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "get filehash", zap.String("path", path))
		return err
	}

	p := joinWithDots(path, checksumPostfix)
	b, err := afero.ReadFile(l.fs, p)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "read file", zap.String("path", p))
		return err
	}
	checksum := strings.Trim(string(b), "\n")

	if hash != checksum {
		return fmt.Errorf("checksum mismatch, want: %v, got: %v", hash, checksum)
	}

	return nil
}

func (l *LocalFiler) getFilehash(path string) (hash string, err error) {
	f, err := l.fs.Open(path)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "open file", zap.String("path", path))
		return
	}
	defer func() {
		fErr := f.Close()
		if err == nil {
			logger.ErrorCtx(l.ctx, fErr, "close file", zap.String("path", path))
			err = fErr
		}
	}()

	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "copy to hash-func", zap.String("path", path))
		return
	}
	hash = fmt.Sprintf("%x", h.Sum(nil))

	return
}

func (l *LocalFiler) copy(dst, src string) (err error) {
	s, err := l.fs.Open(src)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "open file", zap.String("path", src))
		return
	}
	defer func() {
		errClose := s.Close()
		if err == nil {
			logger.ErrorCtx(l.ctx, errClose, "close file", zap.String("path", src))
			err = errClose
		}
	}()

	d, err := l.fs.Create(dst)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "create file", zap.String("path", dst))
		return
	}
	defer func() {
		errClose := d.Close()
		if err == nil {
			logger.ErrorCtx(l.ctx, errClose, "close file", zap.String("path", dst))
			err = errClose
		}
	}()

	_, err = io.Copy(d, s)
	if err != nil {
		logger.ErrorCtx(l.ctx, err, "copy file", zap.String("from", s.Name()), zap.String("to", d.Name()))
	}

	return
}

func joinWithDots(strs ...string) string {
	var s []string
	s = append(s, strs...)

	return strings.Join(s, ".")
}
