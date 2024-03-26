package updater

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"marketplace-yaga/pkg/httpx"
	"marketplace-yaga/pkg/logger"
	"marketplace-yaga/pkg/repo"
	"marketplace-yaga/windows/internal/guest"
	"marketplace-yaga/windows/internal/service"
	mocks2 "marketplace-yaga/windows/internal/updater/mocks"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/go-resty/resty/v2"
	"github.com/gofrs/uuid"
	"github.com/spf13/afero"
	"go.uber.org/zap"
)

type GuestAgent struct {
	ctx             context.Context
	fs              afero.Fs
	fileRepo        repository
	svcMgr          serviceManager
	hclient         httpClient
	agentFilepath   string
	getAgentVersion agentInstalledVersionGetter
}

//go:generate mockery --name Fs --srcpkg "github.com/spf13/afero" --exported --disable-version-string --tags windows

//go:generate mockery --name repository --exported --disable-version-string --tags windows

// to catch interface change
var _ repository = &mocks2.Repository{}

type repository interface {
	Init() error
	Get(version string) string
	Add(p string, lv string) error
	List() []string
}

//go:generate mockery --name serviceManager --exported --disable-version-string --tags windows

var _ serviceManager = &mocks2.ServiceManager{}

type serviceManager interface {
	Init() error
	IsExist(name string) (bool, error)
	Create(path string, name string, displayName string, description string, args ...string) error
	IsStopped(name string) (bool, error)
	Stop(name string) error
	Delete(name string) error
	Start(name string) error
	Close() error
}

//go:generate mockery --name httpClient --exported --disable-version-string --tags windows

var _ httpClient = &mocks2.HttpClient{}

type httpClient interface {
	R() *resty.Request
	Download(filepath, url string) error
	Downloader(w io.Writer, url string) error
	GetClient() *http.Client
}

type agentInstalledVersionGetter func(path string) (string, error)

func getAgentVersion(path string) (string, error) {
	c := exec.Command(path, "version")
	o, err := c.Output()
	if err != nil {
		return "", err
	}

	return strings.Trim(string(o), "\n"), nil
}

var (
	VersionRemoteEndpoint = `https://storage.yandexcloud.net`
	GuestAgentBucket      = `yandexcloud-guestagent`
	GuestAgentLatest      = fmt.Sprintf(`/%v/release/stable`, GuestAgentBucket)
)

const (
	versionLocalRepository = `C:\Program Files\Yandex.Cloud\Guest Agent Updater\Local Repository`
	updaterHTTPAgent       = `Yandex.Cloud.Guest.Agent.Updater`
	checksumSuffix         = "sha256"
)

func New(ctx context.Context) (*GuestAgent, error) {
	if ctx == nil {
		return nil, errors.New("provided nil context")
	}

	err := ctx.Err()
	if errors.Is(err, context.Canceled) {
		return nil, err
	}

	fs := afero.NewOsFs()
	r, err := repo.NewFiler(ctx, versionLocalRepository, guest.AgentExecutable, fs)
	if err != nil {
		return nil, fmt.Errorf("repository creaton failed: %w", err)
	}

	m, err := service.NewManager(ctx)
	if err != nil {
		return nil, fmt.Errorf("service manager creation failed: %w", err)
	}

	h, err := httpx.New(ctx, VersionRemoteEndpoint, updaterHTTPAgent)
	if err != nil {
		return nil, fmt.Errorf("http client creation failed: %w", err)
	}

	u := GuestAgent{
		ctx:             ctx,
		fs:              fs,
		fileRepo:        r,
		svcMgr:          m,
		hclient:         h,
		agentFilepath:   filepath.Join(guest.AgentDir, guest.AgentExecutable),
		getAgentVersion: getAgentVersion,
	}

	return &u, nil
}

func (u *GuestAgent) Init() error {
	if err := u.fileRepo.Init(); err != nil {
		return fmt.Errorf("repository initialization failed: %w", err)
	}

	if err := u.svcMgr.Init(); err != nil {
		return fmt.Errorf("service manager initialization failed: %w", err)
	}

	if err := u.maybeAddInstalledVersion(); err != nil {
		return err
	}

	return nil
}

// Check - is somewhat of dryrun, it reports what actions Updater will commit.
func (u *GuestAgent) Check() (s State, err error) {
	s = Noop

	err = u.ctx.Err()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check context",
			zap.String("state before", s.String()))
		return Unknown, err
	}

	iv, err := u.getInstalledVersion()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check installed version",
			zap.String("version", iv),
			zap.String("state before", s.String()))
		return Unknown, fmt.Errorf("failed to check installed version (%w)", err)
	}

	rv := u.getRepoLatest()
	logger.DebugCtx(u.ctx, nil, "check repo version",
		zap.String("version", rv))

	lv, err := u.getLatest()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check latest version",
			zap.String("version", lv),
			zap.String("state before", s.String()))
		return Unknown, fmt.Errorf("failed to check latest version (%w)", err)
	}

	if lv != "" {
		latestInRepo := u.fileRepo.Get(lv) != ""
		logger.DebugCtx(u.ctx, nil, "check latest version in repo",
			zap.Bool("latest in repo", latestInRepo),
			zap.String("latest", lv),
			zap.String("state before", s.String()))
		if !latestInRepo {
			s += Download // Unknown is 0, Download is 1 -> Download
		}
	}

	isInstalled := iv != ""
	logger.DebugCtx(u.ctx, nil, "check is guest agent installed",
		zap.Bool("installed", isInstalled),
		zap.String("state before", s.String()))
	if !isInstalled {
		s += Install

		return
	}

	siv, err := semver.New(iv)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "parse semver of installed version",
			zap.String("version", iv),
			zap.String("state before", s.String()))
	}

	slv, _ := semver.New(lv)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "parse semver of latest version",
			zap.String("version", lv),
			zap.String("state before", s.String()))
	}

	srv, _ := semver.New(rv)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "parse semver of repo version",
			zap.String("version", rv),
			zap.String("state before", s.String()))
	}

	needUpdate := (siv != nil && slv != nil && siv.LT(*slv)) || (siv != nil && srv != nil && siv.LT(*srv))
	logger.DebugCtx(u.ctx, nil, "check is guest agent need update",
		zap.Bool("need update", needUpdate),
		zap.String("state before", s.String()))
	if needUpdate {
		s += Update
	}

	return
}

// Update - install or update existing guest agent, download if version is absent in local repo.
func (u *GuestAgent) Update() error {
	err := u.ensureLatestAdded()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "ensure latest version added into repo")
		return err
	}

	repoVersion := u.getRepoLatest()
	logger.DebugCtx(u.ctx, nil, "get latest version from repo",
		zap.String("version", repoVersion))
	if repoVersion == "" {
		return nil
	}

	// check installed version if not exist or rv greater - proceed
	instVersion, err := u.getInstalledVersion()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check installed version",
			zap.String("version", instVersion))
		return fmt.Errorf("failed to check installed version (%w)", err)
	}

	if instVersion != "" {
		installed, err := semver.New(instVersion)
		if err != nil {
			logger.ErrorCtx(u.ctx, err, "parse semver of installed version",
				zap.String("version", instVersion))
			return fmt.Errorf("failed to parse (%v) installed version: %w", instVersion, err)
		}

		maybeLatest, err := semver.New(repoVersion)
		if err != nil {
			logger.ErrorCtx(u.ctx, err, "parse semver of latest repo version",
				zap.String("version", repoVersion))
			return fmt.Errorf("failed to parse (%v) latest repo version: %w", repoVersion, err)
		}

		alreadyLatest := installed.GE(*maybeLatest)
		logger.DebugCtx(u.ctx, nil, "check if update needed",
			zap.Bool("alreadyLatest", alreadyLatest))
		if alreadyLatest {
			return nil
		}
	}

	err = u.ctx.Err()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check context")
		return err
	}

	err = u.install(repoVersion)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "install version",
			zap.String("version", repoVersion))
	}
	if err != nil {
		prevRepoVersion := u.getRepoPrevious(repoVersion)
		logger.DebugCtx(u.ctx, nil, "get previous latest version from repo",
			zap.String("version", prevRepoVersion))

		if prevRepoVersion != "" {
			err = u.install(prevRepoVersion)
			if err != nil {
				logger.ErrorCtx(u.ctx, err, "rollback version",
					zap.String("version", prevRepoVersion))
				return fmt.Errorf("failed to rollback to version (%v) from (%v): %w",
					prevRepoVersion, repoVersion, err)
			}

			return nil
		}

		return fmt.Errorf("failed to install version (%v): %w", repoVersion, err)
	}

	return nil
}

// install - perform clean install of agent, removes one if it is already installed.
func (u *GuestAgent) install(v string) error {
	p := u.fileRepo.Get(v)
	logger.DebugCtx(u.ctx, nil, "get version filepath",
		zap.String("filepath", p))
	if p == "" {
		return fmt.Errorf("no version (%v) found in repository", v)
	}

	err := u.Remove()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "ensure guest agent removed")
		return err
	}

	err = u.ensureDirectory()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "ensure guest agent directory exist")
		return err
	}

	err = u.ensureCopy(u.agentFilepath, p)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "ensure guest agent copied",
			zap.String("filepath", p))
		return err
	}

	err = u.ensureService()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "ensure guest agent service created")
		return err
	}

	err = u.Start()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "start service")
	}

	return err
}

func (u *GuestAgent) getInstalledVersion() (string, error) {
	err := u.ctx.Err()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check context")
		return "", err
	}

	e, err := afero.Exists(u.fs, u.agentFilepath)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check agent binary existance",
			zap.String("filepath", u.agentFilepath),
			zap.Bool("exist", e))
		return "", err
	}
	if !e {
		return "", nil
	}

	v, err := u.getAgentVersion(u.agentFilepath)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check agent version",
			zap.String("version", v))
		return "", err
	}

	return v, nil
}

func (u *GuestAgent) getRepoPrevious(to string) string {
	versions := u.fileRepo.List()
	logger.DebugCtx(u.ctx, nil, "get versions from repo",
		zap.Strings("versions", versions))

	if len(versions) > 1 {
		sort.Slice(versions, func(i, j int) bool {
			vi, _ := semver.Parse(versions[i])
			vj, _ := semver.Parse(versions[j])

			return vi.LT(vj)
		})

		for i, v := range versions {
			if v == to && i > 0 {
				return versions[i-1]
			}
		}
	}

	return ""
}

func (u *GuestAgent) getRepoLatest() string {
	versions := u.fileRepo.List()
	logger.DebugCtx(u.ctx, nil, "get versions from repo",
		zap.Strings("versions", versions))

	if len(versions) == 0 {
		return ""
	}

	if len(versions) > 1 {
		sort.Slice(versions, func(i, j int) bool {
			vi, _ := semver.Parse(versions[i])
			vj, _ := semver.Parse(versions[j])

			return vi.GT(vj)
		})
	}

	return versions[0]
}

func (u *GuestAgent) getLatest() (string, error) {
	err := u.ctx.Err()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check context")
		return "", err
	}

	r, err := u.hclient.R().Get(GuestAgentLatest)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "get latest version",
			zap.String("url", GuestAgentLatest))
		return "", err
	}
	if r.IsError() {
		return "", nil
	}
	v := strings.Trim(string(r.Body()), "\n")

	_, err = semver.Parse(v)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "validate version numbers",
			zap.String("version", v))
		return "", nil
	}

	return v, nil
}

func (u *GuestAgent) downloadVersion(v string) (path string, err error) {
	err = u.ctx.Err()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check context")
		return
	}

	tmp, err := getTempFilepath()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "get random filepath",
			zap.String("filepath", tmp))
		return
	}

	tmpAgent := joinWithDots(tmp, "exe")
	urlAgent := fmt.Sprintf(`%v/release/%v/%v/%v/%v`,
		GuestAgentBucket, v, runtime.GOOS, runtime.GOARCH, guest.AgentExecutable)
	a, err := u.fs.Create(tmpAgent)
	if err != nil {
		return
	}
	defer func() { _ = a.Close() }()

	err = u.hclient.Downloader(a, urlAgent)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "download guest agent",
			zap.String("filepath", tmpAgent),
			zap.String("url", urlAgent))
		return
	}
	defer func() {
		if err != nil {
			_ = u.fs.RemoveAll(tmpAgent)
		}
	}()

	tmpChecksum := joinWithDots(tmpAgent, checksumSuffix)
	urlChecksum := fmt.Sprintf(`%v/release/%v/%v/%v/%v`,
		GuestAgentBucket, v, runtime.GOOS, runtime.GOARCH, joinWithDots(guest.AgentExecutable, checksumSuffix))
	c, err := u.fs.Create(tmpChecksum)
	if err != nil {
		return
	}
	defer func() { _ = c.Close() }()

	err = u.hclient.Downloader(c, urlChecksum)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "download checksum",
			zap.String("filepath", tmpChecksum),
			zap.String("url", urlChecksum))
		return
	}
	defer func() {
		if err != nil {
			_ = u.fs.RemoveAll(tmpChecksum)
		}
	}()

	path = tmpAgent

	return
}

// ensureLatestAdded - check if latest version is in repo, add it if not.
func (u *GuestAgent) ensureLatestAdded() error {
	err := u.ctx.Err()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check context")
		return err
	}

	lv, err := u.getLatest()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "get latest version",
			zap.String("version", lv))
		return err
	}

	alreadyAdded := u.fileRepo.Get(lv) != ""
	logger.DebugCtx(u.ctx, nil, "check if latest already in repo",
		zap.Bool("alreadyAdded", alreadyAdded))
	if alreadyAdded {
		return nil
	}

	p, err := u.downloadVersion(lv)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "download version",
			zap.String("version", lv))
		return err
	}

	err = u.fileRepo.Add(p, lv)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "add version to repo",
			zap.String("filepath", p),
			zap.String("version", lv))
		return err
	}

	err = u.fs.Remove(p)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "delete downloaded temporary version",
			zap.String("filepath", p))
		return err
	}

	c := joinWithDots(p, checksumSuffix)
	err = u.fs.Remove(c)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "delete downloaded temporary chechsum",
			zap.String("filepath", c))
		return err
	}

	return nil
}

const defaultPerms os.FileMode = 0770

// maybeAddInstalledVersion - checks if running version in repo, add it if not.
func (u *GuestAgent) maybeAddInstalledVersion() (err error) {
	err = u.ctx.Err()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check context")
		return
	}

	// get installed version
	v, err := u.getInstalledVersion()
	if err != nil || v == "" {
		logger.ErrorCtx(u.ctx, err, "check agent version",
			zap.String("version", v))
		return
	}

	// getRepoVersion
	rv := u.fileRepo.Get(v)
	logger.DebugCtx(u.ctx, nil, "check agent version at repository",
		zap.String("version", v))
	if rv != "" { // found
		return
	}

	// get hash
	f, err := u.fs.Open(u.agentFilepath)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "get file content",
			zap.String("filepath", u.agentFilepath))
		return
	}
	defer func() {
		fErr := f.Close()
		if err == nil {
			err = fErr
		}
	}()

	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "calculate hash")
		return
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))

	// create hashfile
	cp := joinWithDots(u.agentFilepath, checksumSuffix)
	err = afero.WriteFile(u.fs, cp, []byte(hash), defaultPerms)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "write filehash",
			zap.String("filepath", cp),
			zap.String("hash", hash))
		return
	}

	// add
	err = u.fileRepo.Add(u.agentFilepath, v)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "add file to repository",
			zap.String("filepath", u.agentFilepath),
			zap.String("version", v))
	}

	return
}

// ensureService - checks if guest agent service exist, create it if not.
func (u *GuestAgent) ensureService() error {
	e, err := u.svcMgr.IsExist(guest.ServiceName)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check service exist",
			zap.String("service name", guest.ServiceName),
			zap.Bool("exist", e))
		return err
	}
	if e {
		return nil
	}

	err = u.svcMgr.Create(u.agentFilepath, guest.ServiceName, guest.ServiceName, guest.ServiceDescription, guest.ServiceArgs)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "create service",
			zap.String("path", u.agentFilepath),
			zap.String("display name", guest.ServiceName),
			zap.String("description", guest.ServiceDescription),
			zap.String("name", guest.ServiceName),
			zap.String("args", guest.ServiceArgs))
		return err
	}

	return nil
}

// ensureNoService - removes guest agent service, stopps it if one is running and then performs deletion.
func (u *GuestAgent) ensureNoService() error {
	e, err := u.svcMgr.IsExist(guest.ServiceName)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check service exist",
			zap.String("service name", guest.ServiceName),
			zap.Bool("exist", e))
		return err
	}
	if !e {
		return nil
	}

	stopped, err := u.svcMgr.IsStopped(guest.ServiceName)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check service is stopped",
			zap.String("service name", guest.ServiceName),
			zap.Bool("is stopped", stopped))
		return err
	}
	if !stopped {
		err = u.svcMgr.Stop(guest.ServiceName)
		if err != nil {
			logger.ErrorCtx(u.ctx, err, "stop service",
				zap.String("service name", guest.ServiceName))
			return err
		}
	}

	err = u.svcMgr.Delete(guest.ServiceName)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "delete service",
			zap.String("service name", guest.ServiceName))
		return err
	}

	return nil
}

// ensureDirectory - creates guest agent directory on path if one does not exist.
func (u *GuestAgent) ensureDirectory() error {
	e, err := afero.Exists(u.fs, guest.AgentDir)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check guest agent directory exist",
			zap.String("directory path", guest.AgentDir),
			zap.Bool("exist", e))
		return err
	}
	if e {
		d, err := afero.IsDir(u.fs, guest.AgentDir)
		if err != nil {
			return err
		}
		if d {
			return nil
		}

		return errors.New("not a directory")
	}

	err = u.fs.MkdirAll(guest.AgentDir, defaultPerms)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "create guest agent directory",
			zap.String("directory path", guest.AgentDir))
		return err
	}

	return nil
}

// ensureNoDirectory - removes guest agent directory.
func (u *GuestAgent) ensureNoDirectory() error {
	err := u.fs.RemoveAll(guest.AgentDir)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "remove guest agent directory",
			zap.String("directory path", guest.AgentDir))
		return err
	}

	return nil
}

func (u *GuestAgent) ensureCopy(dst, src string) (err error) {
	s, err := u.fs.Open(src)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "open file",
			zap.String("filepath", src))
		return
	}
	defer func() {
		errClose := s.Close()
		if err == nil {
			logger.ErrorCtx(u.ctx, errClose, "close source file",
				zap.String("filepath", src))
			err = errClose
		}
	}()

	d, err := u.fs.Create(dst)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "open file",
			zap.String("filepath", dst))
		return
	}
	defer func() {
		errClose := d.Close()
		if err == nil {
			logger.ErrorCtx(u.ctx, errClose, "close destination file",
				zap.String("filepath", dst))
			err = errClose
		}
	}()

	_, err = io.Copy(d, s)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "copy file",
			zap.String("source filepath", src),
			zap.String("destination filepath", dst))
	}

	return
}

// Remove - removes guest agent if one is installed.
func (u *GuestAgent) Remove() (err error) {
	err = u.ctx.Err()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check context")
		return
	}

	err = u.ensureNoService()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "ensure no service")
		return
	}

	// we use for tests https://github.com/spf13/afero/blob/master/memmap.go#L282
	// RemoveAll '/foo/bar' will also delete '/foo/bar baz'
	// therefore guest agent updater dir in tests coz its path is prefix to guest agent dir
	exist, err := afero.DirExists(u.fs, guest.AgentDir)
	if err != nil || !exist {
		logger.ErrorCtx(u.ctx, err, "check directory exist",
			zap.String("path", guest.AgentDir),
			zap.Bool("exist", exist))
		return
	}

	f, err := u.fs.Open(guest.AgentDir)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "open directory",
			zap.String("path", guest.AgentDir))
		return
	}
	defer func() {
		fErr := f.Close()
		if err == nil {
			err = fErr
		}
	}()

	names, err := f.Readdirnames(0)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "read agent directory entries",
			zap.Strings("names", names))
		return
	}

	for _, name := range names {
		err = u.fs.RemoveAll(filepath.Join(guest.AgentDir, name))
		if err != nil {
			logger.ErrorCtx(u.ctx, err, "ensure no subdirectory entities",
				zap.String("path", filepath.Join(guest.AgentDir, name)))
			return
		}
	}

	err = u.fs.Remove(guest.AgentDir)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "ensure no directory",
			zap.String("directory path", guest.AgentDir))
	}

	return
}

func (u *GuestAgent) Start() error {
	err := u.svcMgr.Start(guest.ServiceName)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "start service",
			zap.String("service name", guest.ServiceName))
	}

	return err
}

func (u *GuestAgent) Stop() error {
	err := u.ctx.Err()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "check context")
		return err
	}

	err = u.svcMgr.Stop(guest.ServiceName)
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "stop service",
			zap.String("service name", guest.ServiceName))
	}

	return err
}

func (u *GuestAgent) Close() error {
	err := u.svcMgr.Close()
	if err != nil {
		logger.ErrorCtx(u.ctx, err, "close service manager")
	}

	return err
}

func joinWithDots(strs ...string) string {
	var s []string
	s = append(s, strs...)

	return strings.Join(s, ".")
}

func getTempFilepath() (string, error) {
	u, err := uuid.NewV4()
	if err != nil {
		return "", err
	}

	return filepath.Join(os.TempDir(), u.String()), nil
}
