/*
	ToDo:
		* Add (int)
*/
package repo

import (
	"context"
	"marketplace-yaga/pkg/logger"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
)

func TestAll(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(filerTableTests))
	suite.Run(t, new(filerTests))
	suite.Run(t, new(joinWithDotsTableTests))
}

type testSuite struct {
	ctx                  context.Context
	testRoot             string
	testFilename         string
	testChecksumFilename string
	testChecksum         string
	testSpareFilename    string
	testSpareContent     string
	testSpareRoot        string
	testSpareChecksum    string
	testVersion          string
	testFileContent      []byte
	testFs               afero.Fs
}

func initSuite(t *testing.T) testSuite {
	return testSuite{
		ctx:                  logger.NewContext(context.Background(), zaptest.NewLogger(t)),
		testRoot:             "/my/test/root/versions",
		testFilename:         "my_file.txt",
		testFileContent:      []byte{46, 46, 46, 45, 45, 45, 46, 46, 46},
		testChecksumFilename: "my_file.txt.sha256",
		testChecksum:         "d262964716f4275f54e0368cd94d36b1427cbba19e064b0c29bf8e9475e7ae42",
		testSpareRoot:        "/my/test/root",
		testSpareFilename:    "kenny.spare",
		testSpareContent:     "kenny",
		testSpareChecksum:    "6cadf2f0f34dc55acde751c0f5e4b7cae56694f304c41bbd77ae351421884008",
		testVersion:          "0.0.1",
		testFs:               afero.NewMemMapFs(),
	}
}

// filerTests - is a set of tests, where we assert behaviour of filer
// on init (loading) its repository in different states.
type filerTests struct {
	testSuite
	suite.Suite
}

func (s *filerTests) SetupTest() {
	s.testSuite = initSuite(s.T())
}

// TestInitEmptyCreate - must create root folder.
func (s *filerTests) TestInitEmptyCreate() {
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	t, err := s.testFs.Stat(s.testRoot)
	s.NoError(err)
	s.True(t.IsDir())
}

func (s *filerTests) createRoot() {
	s.Require().NoError(s.testFs.MkdirAll(s.testRoot, defaultPerms))
}

// TestInitOnEmptyVersionDir - must correctly init if repository is empty.
func (s *filerTests) TestInitOnEmptyVersionDir() {
	s.createRoot()

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())
}

func (s *filerTests) createSpareFileAboveRoot() string {
	spareFilepath := filepath.Join(s.testSpareRoot, s.testSpareFilename)
	s.Require().NoError(afero.WriteFile(s.testFs, spareFilepath, s.testFileContent, defaultPerms))

	return spareFilepath
}

// TestInitEmptyDoNotRecreate - must not recreate root folder if one already exist.
func (s *filerTests) TestInitEmptyDoNotRecreate() {
	s.createRoot()
	p := s.createSpareFileAboveRoot()

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	t, err := s.testFs.Stat(p)
	s.NoError(err)
	s.True(!t.IsDir())
}

func (s *filerTests) createEmptyVersionDirectory() string {
	path := filepath.Join(s.testRoot, s.testVersion)
	s.Require().NoError(s.testFs.MkdirAll(path, defaultPerms))

	return path
}

// TestInitOnEmptySemverDir - remove version from repository if one's version directory is empty.
func (s *filerTests) TestInitOnEmptySemverDir() {
	s.createRoot()
	p := s.createEmptyVersionDirectory()

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	// deleted
	_, err = s.testFs.Stat(p)
	s.ErrorIs(err, afero.ErrFileNotFound)
}

func (s *filerTests) createBrokenVersionDirectory() string {
	path := filepath.Join(s.testRoot, "I R NOT SEMVER!11")
	s.Require().NoError(s.testFs.MkdirAll(path, defaultPerms))

	return path
}

// TestInitOnWrongSemverDir - remove version from repository if one is not semver.
func (s *filerTests) TestInitOnWrongSemverDir() {
	s.createRoot()
	p := s.createBrokenVersionDirectory()

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	// deleted
	_, err = s.testFs.Stat(p)
	s.ErrorIs(err, afero.ErrFileNotFound)
}

func (s *filerTests) createVersionFile() {
	path := filepath.Join(s.testRoot, s.testVersion, s.testFilename)
	s.Require().NoError(afero.WriteFile(s.testFs, path, s.testFileContent, defaultPerms))
}

// TestInitOnIncompleteSemverDir - remove version from repository if one does not contain stored file itself.
func (s *filerTests) TestInitOnIncompleteSemverDir() {
	s.createRoot()
	p := s.createEmptyVersionDirectory()
	s.createVersionFile()

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	// deleted
	_, err = s.testFs.Stat(p)
	s.ErrorIs(err, afero.ErrFileNotFound)
}

func (s *filerTests) createVersionChecksum() {
	path := filepath.Join(s.testRoot, s.testVersion, s.testChecksumFilename)
	s.Require().NoError(afero.WriteFile(s.testFs, path, []byte(s.testChecksum), defaultPerms))
}

// TestInitOnNoChecksum - remove version form repository if there is no checksum file in it.
func (s *filerTests) TestInitOnNoChecksum() {
	s.createRoot()
	p := s.createEmptyVersionDirectory()
	s.createVersionChecksum()

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	// deleted
	_, err = s.testFs.Stat(p)
	s.ErrorIs(err, afero.ErrFileNotFound)
}

func (s *filerTests) createVersionBrokenChecksum() {
	path := filepath.Join(s.testRoot, s.testVersion, s.testChecksumFilename)
	s.Require().NoError(afero.WriteFile(s.testFs, path, s.testFileContent, defaultPerms))
}

// TestInitOnChecksumMismatch - remove version form repository in case of checksum mismatch.
func (s *filerTests) TestInitOnChecksumMismatch() {
	s.createRoot()
	p := s.createEmptyVersionDirectory()
	s.createVersionFile()
	s.createVersionBrokenChecksum()

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	// deleted
	_, err = s.testFs.Stat(p)
	s.ErrorIs(err, afero.ErrFileNotFound)
}

// TestInitOnValidVersion - successfully initialize with existing valid version in folder.
func (s *filerTests) TestInitOnValidVersion() {
	s.createRoot()
	p := s.createEmptyVersionDirectory()
	s.createVersionFile()
	s.createVersionChecksum()

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	// ok
	st, err := s.testFs.Stat(p)
	s.NoError(err)
	s.True(st.IsDir())
}

func (s *filerTests) createDummyVersion(v string) {
	path := filepath.Join(s.testRoot, v, s.testFilename)
	s.Require().NoError(afero.WriteFile(s.testFs, path, s.testFileContent, defaultPerms))

	path = filepath.Join(s.testRoot, v, s.testChecksumFilename)
	s.Require().NoError(afero.WriteFile(s.testFs, path, []byte(s.testChecksum), defaultPerms))
}

// TestVersionRotationOnInit - validate that only allowed latest number of versions caches, oldest removed.
func (s *filerTests) TestVersionRotationOnInit() {
	s.createRoot()
	for i := 0; i < numVersionsToCache+1; i++ {
		s.createDummyVersion("0.0." + strconv.Itoa(i))
	}

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	// ok
	s.Equal(numVersionsToCache, len(f.List()))
}

// TestVersionsSortedOnInit - validates that all versions are sorted on init in descending order.
func (s *filerTests) TestVersionsSortedOnInit() {
	s.createRoot()
	for i := 0; i < numVersionsToCache; i++ {
		s.createDummyVersion("0.0." + strconv.Itoa(i))
	}

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	// ok
	for i := 0; i < len(f.versions)-1; i++ {
		a, _ := semver.Parse(f.versions[i])
		b, _ := semver.Parse(f.versions[i+1])

		s.True(a.GT(b))
	}
}

func (s *filerTests) TestValidateVersion() {
	version := "0.0.1"
	s.createDummyVersion(version)

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	s.NoError(f.validateVersion(version))
}

func (s *filerTests) createSpareFile() string {
	s.Require().NoError(s.testFs.MkdirAll(s.testSpareRoot, defaultPerms))

	spareFilepath := filepath.Join(s.testSpareRoot, s.testSpareFilename)
	s.Require().NoError(afero.WriteFile(s.testFs, spareFilepath, []byte(s.testSpareContent), defaultPerms))

	return spareFilepath
}

// TestGetFilehash - tests if sha256 of file is correct.
// Note: since afero package has some peculiarities, and this is fine, test is narrow.
// One of'em is https://github.com/spf13/afero/issues/270.
func (s *filerTests) TestGetFilehash() {
	spareFilepath := s.createSpareFile()

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())
	hash, err := f.getFilehash(spareFilepath)

	s.Equal(s.testSpareChecksum, hash)
	s.NoError(err)
}

func (s *filerTests) createSpareChecksumFile() string {
	s.Require().NoError(s.testFs.MkdirAll(s.testSpareRoot, defaultPerms))

	spareChecksumFilepath := filepath.Join(s.testSpareRoot, joinWithDots(s.testSpareFilename, checksumPostfix))
	s.Require().NoError(afero.WriteFile(s.testFs, spareChecksumFilepath, []byte(s.testSpareChecksum), defaultPerms))

	return spareChecksumFilepath
}

func (s *filerTests) TestValidateFilehash() {
	spareFilepath := s.createSpareFile()
	_ = s.createSpareChecksumFile()

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	s.NoError(f.validateFilehash(spareFilepath))
}

func (s *filerTests) TestRemoveExisted() {
	s.createDummyVersion(s.testVersion)

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	s.NoError(f.Remove(s.testVersion))
}

func (s *filerTests) TestRemoveNotExisted() {
	s.createDummyVersion(s.testVersion)

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	s.NoError(f.Remove("6.6.6"))
	s.Contains(f.List(), s.testVersion)
}

func (s *filerTests) TestAddAlreadyExisted() {
	s.createDummyVersion(s.testVersion)

	spareFilepath := s.createSpareFile()
	_ = s.createSpareChecksumFile()

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	s.ErrorIs(f.Add(spareFilepath, s.testVersion), ErrAlreadyAdded)
	s.Len(f.List(), 1)
	s.Contains(f.List(), s.testVersion)
}

func (s *filerTests) TestAddEmptySource() {
	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	s.ErrorIs(f.Add(filepath.Join(s.testSpareRoot, s.testSpareFilename), s.testVersion), ErrNotFound)
	s.Len(f.List(), 0)
}

func (s *filerTests) TestAddNoChecksum() {
	spareFilepath := s.createSpareFile()

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	s.ErrorIs(f.Add(spareFilepath, s.testVersion), ErrNotFound)
	s.Len(f.List(), 0)
}

func (s *filerTests) TestAddNoFile() {
	checksumFilepath := s.createSpareChecksumFile()
	spareFilepath := strings.Trim(checksumFilepath, "."+checksumPostfix)

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	s.ErrorIs(f.Add(spareFilepath, s.testVersion), ErrNotFound)
	s.Len(f.List(), 0)
}

func (s *filerTests) TestAdd() {
	spareFilepath := s.createSpareFile()
	_ = s.createSpareChecksumFile()

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	s.NoError(f.Add(spareFilepath, s.testVersion))
	s.Contains(f.List(), s.testVersion)
}

func (s *filerTests) TestAddAdditional() {
	s.createDummyVersion("6.6.6")
	spareFilepath := s.createSpareFile()
	_ = s.createSpareChecksumFile()

	// test
	f, err := NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)
	s.NoError(err)
	s.NoError(f.Init())

	s.NoError(f.Add(spareFilepath, s.testVersion))
	s.Contains(f.List(), s.testVersion)
	s.Contains(f.List(), "6.6.6")
	s.Len(f.List(), 2)
}

//
// table tests
//

type filerTableTests struct {
	testSuite
	f *LocalFiler
	suite.Suite
}

func (s *filerTableTests) SetupTest() {
	s.testSuite = initSuite(s.T())

	var err error
	s.f, err = NewFiler(s.ctx, s.testRoot, s.testFilename, s.testFs)

	s.Require().NoError(err)
	s.Require().NoError(s.f.Init())
}

func (s *filerTableTests) TestNew() {
	tests := []struct {
		ctx      context.Context
		root     string
		filename string
		fs       afero.Fs
		err      error
	}{
		{nil, s.testRoot, s.testFilename, s.testFs, ErrNilCtx},
		{context.Background(), "", s.testFilename, s.testFs, ErrEmptyRoot},
		{context.Background(), s.testRoot, "", s.testFs, ErrEmptyFilename},
		{context.Background(), s.testRoot, s.testFilename, nil, ErrNilFs},
		{context.Background(), s.testRoot, s.testFilename, s.testFs, nil},
	}

	for _, t := range tests {
		_, err := NewFiler(t.ctx, t.root, t.filename, t.fs)

		if t.err != nil {
			s.ErrorIs(err, t.err)
		} else {
			s.NoError(err)
		}
	}
}

func (s *filerTableTests) TestSort() {
	tests := []struct {
		versions []string
		want     []string
	}{
		{versions: []string{"1.0.0", "0.1.0", "0.0.1"}, want: []string{"1.0.0", "0.1.0", "0.0.1"}},
		{versions: []string{"1.0.0", "0.0.1", "0.1.0"}, want: []string{"1.0.0", "0.1.0", "0.0.1"}},
		{versions: []string{"0.1.0", "0.0.1", "1.0.0"}, want: []string{"1.0.0", "0.1.0", "0.0.1"}},
		{versions: []string{"0.1.0", "1.0.0", "0.0.1"}, want: []string{"1.0.0", "0.1.0", "0.0.1"}},
		{versions: []string{"0.0.1", "1.0.0", "0.1.0"}, want: []string{"1.0.0", "0.1.0", "0.0.1"}},
		{versions: []string{"0.0.1", "0.1.0", "1.0.0"}, want: []string{"1.0.0", "0.1.0", "0.0.1"}},
		{versions: []string{"1.0.0", "10.0.0", "2.0.0"}, want: []string{"10.0.0", "2.0.0", "1.0.0"}},
		{versions: []string{"1.0.0", "0.0.1"}, want: []string{"1.0.0", "0.0.1"}},
		{versions: []string{"0.0.1", "1.0.0"}, want: []string{"1.0.0", "0.0.1"}},
		{versions: []string{"1.0.0"}, want: []string{"1.0.0"}},
		{versions: []string{}, want: []string{}},
	}

	for _, t := range tests {
		s.f.versions = t.versions
		s.f.sortVersions()
		s.Equal(t.want, s.f.versions)
	}
}

func (s *filerTableTests) TestGet() {
	tests := []struct {
		versions []string
		get      string
		want     string
	}{
		{versions: []string{}, get: "", want: ""},
		{versions: []string{"0.0.1"}, get: "0.0.1", want: filepath.Join(s.f.root, "0.0.1", s.f.filename)},
		{versions: []string{"0.0.1"}, get: "100500.0.0", want: ""},
		{versions: []string{"0.0.1"}, get: "", want: ""},
		{versions: []string{"1.0.0", "0.1.0", "0.0.1"}, get: "100500.0.0", want: ""},
		{versions: []string{"1.0.0", "0.1.0", "0.0.1"}, get: "1.0.0", want: filepath.Join(s.f.root, "1.0.0", s.f.filename)},
		{versions: []string{"1.0.0", "0.1.0", "0.0.1"}, get: "0.1.0", want: filepath.Join(s.f.root, "0.1.0", s.f.filename)},
		{versions: []string{"1.0.0", "0.1.0", "0.0.1"}, get: "0.0.1", want: filepath.Join(s.f.root, "0.0.1", s.f.filename)},
		{versions: []string{"1.0.0", "0.1.0", "0.0.1"}, get: "", want: ""},
	}

	for _, t := range tests {
		s.f.versions = t.versions
		s.Equal(t.want, s.f.Get(t.get))
	}
}

func (s *filerTableTests) TestList() {
	tests := []struct {
		versions []string
		want     []string
	}{
		{versions: []string{}, want: []string{}},
		{versions: []string{"0.0.1"}, want: []string{"0.0.1"}},
		{versions: []string{"1.0.0", "0.1.0"}, want: []string{"1.0.0", "0.1.0"}},
		{versions: []string{"1.0.0", "0.1.0", "0.0.1"}, want: []string{"1.0.0", "0.1.0", "0.0.1"}},
		{versions: []string{"1.0.0", "0.100500.0", "0.0.1"}, want: []string{"1.0.0", "0.100500.0", "0.0.1"}},
	}

	for _, t := range tests {
		s.f.versions = t.versions
		s.Equal(t.want, s.f.List())
	}
}

func (s *filerTableTests) createSpareFile() string {
	s.Require().NoError(s.f.fs.MkdirAll(s.testSpareRoot, defaultPerms))

	spareFilepath := filepath.Join(s.testSpareRoot, s.testSpareFilename)
	s.Require().NoError(afero.WriteFile(s.f.fs, spareFilepath, []byte(s.testSpareContent), defaultPerms))

	return spareFilepath
}

func (s *filerTableTests) TestValidateExist() {
	spareFilepath := s.createSpareFile()

	tests := []struct {
		path string
		err  error
	}{
		{"", ErrNotFound},
		{"/knowhere/my_file.txt", ErrNotFound},
		{"/knowhere/my_directory", ErrNotFound},
		{"/knowhere/", ErrNotFound},
		{spareFilepath, nil},
		{s.testSpareRoot, nil},
	}

	for _, t := range tests {
		err := s.f.validateExist(t.path)
		if t.err != nil {
			s.ErrorIs(err, t.err)
		}
	}
}

func (s *filerTableTests) TestValidateDirectory() {
	spareFilepath := s.createSpareFile()

	tests := []struct {
		path string
		err  error
	}{
		{"", ErrNotFound},
		{"/knowhere/my_file.txt", ErrNotFound},
		{"/knowhere/my_directory", ErrNotFound},
		{"/knowhere/", ErrNotFound},
		{spareFilepath, ErrNotDir},
		{s.testSpareRoot, nil},
	}

	for _, t := range tests {
		err := s.f.validateDirectory(t.path)
		if t.err != nil {
			s.ErrorIs(err, t.err)
		}
	}
}
func (s *filerTableTests) TestValidateFile() {
	spareFilepath := s.createSpareFile()

	tests := []struct {
		path string
		err  error
	}{
		{"", ErrNotFound},
		{"/knowhere/my_file.txt", ErrNotFound},
		{"/knowhere/my_directory", ErrNotFound},
		{"/knowhere/", ErrNotFound},
		{s.testSpareRoot, ErrNotFile},
		{spareFilepath, nil},
	}

	for _, t := range tests {
		err := s.f.validateFile(t.path)
		if t.err != nil {
			s.ErrorIs(err, t.err)
		}
	}
}

func (s *filerTableTests) TestCopy() {
	spareFilepath := s.createSpareFile()

	tests := []struct {
		dst string
		src string
		err error
	}{
		{"/knowhere/diary.txt", "/middle/of/knowhere/diary.txt", afero.ErrFileNotFound},
		{filepath.Join(s.testRoot, s.testFilename), "/middle/of/knowhere/diary.txt", afero.ErrFileNotFound},
		{"/knowhere/diary.txt", spareFilepath, nil},
		{"/knowhere/diary.txt", spareFilepath, nil},
	}

	for _, t := range tests {
		err := s.f.copy(t.dst, t.src)
		if t.err != nil {
			s.ErrorIs(err, t.err)
		}
	}
}

//
// helpers
//

type joinWithDotsTableTests struct{ suite.Suite }

func (s *joinWithDotsTableTests) TestJoinWithDots() {
	s.Equal("one.by.one", joinWithDots("one", "by", "one"))
	s.Equal("..", joinWithDots("", "", ""))
	s.Equal("", joinWithDots(""))
}
