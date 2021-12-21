package winapi

import (
	"errors"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// consumer interfaces

// Operates Net* API but hard-coded to use local users on local machine.
type LocalUser struct{}

// Checks if user with provided name exists.
func (u *LocalUser) Exist(username string) (bool, error) {
	return localUserExist(username)
}

// Checks if accountDisable flag set on user.
func (u *LocalUser) IsDisabled(username string) (bool, error) {
	return localUserIsDisabled(username)
}

// Queries for particular user builtin\Administrators group.
func (u *LocalUser) IsAdministrator(username string) (bool, error) {
	return localUserIsAdministrator(username)
}

// Checks if dontExpirePasswd flag set on user.
func (u *LocalUser) IsPasswordNeverExpire(username string) (bool, error) {
	return localUserIsPasswordNeverExpire(username)
}

// Enables user account.
func (u *LocalUser) Enable(username string) error {
	return localUserEnable(username)
}

// Adds user to builtin\Administrators group.
func (u *LocalUser) AddToAdministrators(username string) error {
	return localUserAddToAdministrators(username)
}

// Makes password of user never to expire.
func (u *LocalUser) SetPasswordNeverExpire(username string) error {
	return localUserSetPasswordNeverExpire(username)
}

// Simply changes user pasword.
func (u *LocalUser) SetPassword(username, password string) error {
	return localUserSetPassword(username, password)
}

// Creates user with administrative privileges.
func (u *LocalUser) CreateUser(username, password string) error {
	return localUserCreate(username, password, userPrivUser, userFlagScript|userFlagDontExpirePasswd|userFlagNormalAccount)
}

// bread crumb for data types
// https://docs.microsoft.com/en-us/windows/win32/winprog/windows-data-types

// well-known error numbers
const (
	nErrSuccess      = 0
	nErrUserNotFound = 2221
)

// verbosity for syscalls
const (
	localSystem      = 0
	ignoreParamError = 0
)

// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-samr/10bf6c8e-34af-4cf9-8dff-6b6330922863
const (
	userFlagfAccountDisable  = 0x0002
	userFlagDontExpirePasswd = 0x10000
	userFlagNormalAccount    = 0x0200
	userFlagScript           = 0x0001
)

// https://docs.microsoft.com/en-us/windows/win32/secauthz/privileges
const userPrivUser = 1

const (
	userMaxPrefferedLength = 0xFFFFFFFF
	userLevel0             = 0
	userLevel1             = 1
	userLevel3             = 3
	userLevel1003          = 1003
	userLevel1008          = 1008
)

// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-samr/565a6584-3061-4ede-a531-f5c53826504b
const winBuiltinAdministratorsSid = 26

var (
	ErrEmptyBuff  = errors.New("got zeroed buffer")
	ErrUnrEntLeft = errors.New("got unread entries")
	ErrNotFound   = windows.Errno(nErrUserNotFound)
)

// unfortunately not all NetAPI functions are generated/implemented in windows package
var (
	netapi32DLL                 = windows.NewLazySystemDLL("Netapi32.dll")
	procNetLocalGroupEnum       = netapi32DLL.NewProc("NetLocalGroupEnum")
	procNetUserGetInfo          = netapi32DLL.NewProc("NetUserGetInfo")
	procNetLocalGroupGetMembers = netapi32DLL.NewProc("NetLocalGroupGetMembers")
	procNetUserSetInfo          = netapi32DLL.NewProc("NetUserSetInfo")
	procNetUserAdd              = netapi32DLL.NewProc("NetUserAdd")
	procNetLocalGroupAddMembers = netapi32DLL.NewProc("NetLocalGroupAddMembers")
)

// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/ns-lmaccess-user_info_1
// Used within different functions in a package, so pin it to the top.
type userInfo1 struct {
	name        *uint16 // LPWSTR
	password    *uint16 // LPWSTR
	passwordAge uint32  // DWORD
	priv        uint32  // DWORD
	homeDir     *uint16 // LPWSTR
	comment     *uint16 // LPWSTR
	flags       uint32  // DWORD
	scriptPath  *uint16 // LPWSTR
}

// Seeks Builtin\Administrators local group, enums members compares to provided username.
func localUserIsAdministrator(name string) (bool, error) {
	administratorsGroupName, err := localGroupGetBuiltinAdministrators()
	if err != nil {
		return false, err
	}

	members, err := localGroupGetMembers(administratorsGroupName)
	if err != nil {
		return false, err
	}

	for _, u := range members {
		if name == u {
			return true, nil
		}
	}

	return false, nil
}

// Gets actual Builtin\Administrators group name, some could change it or it may vary depending on locale.
func localGroupGetBuiltinAdministrators() (string, error) {
	groups, err := localGroupEnum()
	if err != nil {
		return "", err
	}

	return localAccountFilter(groups, winBuiltinAdministratorsSid)
}

// Enumerates existing local groups.
func localGroupEnum() (groups []string, err error) {
	var bufptr unsafe.Pointer
	var resumeHandle uintptr
	var entriesRead, entriesTotal uint32

	// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/nf-lmaccess-netlocalgroupenum
	r1, _, _ := procNetLocalGroupEnum.Call(
		uintptr(localSystem),
		uintptr(userLevel0),
		uintptr(unsafe.Pointer(&bufptr)),
		uintptr(userMaxPrefferedLength),
		uintptr(unsafe.Pointer(&entriesRead)),
		uintptr(unsafe.Pointer(&entriesTotal)),
		uintptr(unsafe.Pointer(&resumeHandle)),
	)
	if r1 != nErrSuccess {
		err = windows.Errno(r1)
		return
	}
	if entriesRead != entriesTotal {
		err = ErrUnrEntLeft
		return
	}
	if uintptr(bufptr) == uintptr(0) {
		err = ErrEmptyBuff
		return
	}

	// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/ns-lmaccess-localgroup_info_0
	type localGroupInfo0 struct {
		Name *uint16
	}
	var giSize localGroupInfo0
	bp := bufptr

	for i := uint32(0); i < entriesRead; i++ {
		gi := (*localGroupInfo0)(unsafe.Pointer(bp))
		groups = append(groups, windows.UTF16PtrToString(gi.Name))
		bp = unsafe.Pointer(uintptr(bp) + unsafe.Sizeof(giSize))
	}

	runtime.KeepAlive(bufptr)
	runtime.KeepAlive(resumeHandle)
	runtime.KeepAlive(entriesRead)
	runtime.KeepAlive(entriesTotal)

	return
}

// Filters given slice of users for specific well-known sid aliases.
// For example builtin\administrator wil allways have '*-500' SID.
func localAccountFilter(accounts []string, wksid uint32) (string, error) {
	for _, a := range accounts {
		// https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-lookupaccountnamew
		// https://docs.microsoft.com/en-us/windows/win32/api/winnt/ne-winnt-sid_name_use
		sid, _, _, err := windows.LookupSID("", a)
		if err != nil {
			return "", err
		}
		// https://docs.microsoft.com/en-us/windows/win32/api/winnt/ne-winnt-well_known_sid_type
		if ok := sid.IsWellKnown(windows.WELL_KNOWN_SID_TYPE(wksid)); ok {
			return a, nil
		}

		runtime.KeepAlive(sid)
	}

	return "", ErrNotFound
}

// Enumerates local user's account names of given local group.
func localGroupGetMembers(name string) (users []string, err error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return
	}

	var bufptr unsafe.Pointer
	var resumeHandle uintptr             // LPBYTE
	var entriesRead, entriesTotal uint32 // DWORD

	// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/nf-lmaccess-netlocalgroupgetmembers
	r1, _, _ := procNetLocalGroupGetMembers.Call(
		uintptr(localSystem),
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(userLevel3),
		uintptr(unsafe.Pointer(&bufptr)),
		uintptr(uint32(userMaxPrefferedLength)),
		uintptr(unsafe.Pointer(&entriesRead)),
		uintptr(unsafe.Pointer(&entriesTotal)),
		uintptr(unsafe.Pointer(&resumeHandle)),
	)

	if r1 != nErrSuccess {
		err = windows.Errno(r1)
		return
	} else if uintptr(bufptr) == uintptr(0) {
		err = ErrEmptyBuff
		return
	}

	bp := bufptr
	var uiSize userInfo1
	for i := uint32(0); i < entriesRead; i++ {
		// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/ns-lmaccess-localgroup_members_info_1
		ui := (*userInfo1)(bp)

		// ^^^ syscall will pull domain part with name [user_machine\user_name]
		n := windows.UTF16PtrToString(ui.name)
		if s := strings.Split(n, "\\"); len(s) > 1 {
			n = s[1]
		}

		users = append(users, n)
		bp = unsafe.Pointer(uintptr(bp) + unsafe.Sizeof(uiSize))
	}

	runtime.KeepAlive(namePtr)
	runtime.KeepAlive(bufptr)
	runtime.KeepAlive(resumeHandle)
	runtime.KeepAlive(entriesRead)
	runtime.KeepAlive(entriesTotal)

	return
}

// Checks if we could find local user with given username.
func localUserExist(name string) (bool, error) {
	_, err := localUserGet(name)
	if err != nil {
		if errors.Is(err, windows.Errno(nErrUserNotFound)) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

type user struct {
	Name  string
	Priv  uint32
	Flags uint32
}

// Gets local user account with its flags and privilege flags.
func localUserGet(name string) (u *user, err error) {
	var bufptr unsafe.Pointer
	n := windows.StringToUTF16Ptr(name)

	// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/nf-lmaccess-netusergetinfo
	r1, _, _ := procNetUserGetInfo.Call(
		uintptr(localSystem),
		uintptr(unsafe.Pointer(n)),
		uintptr(userLevel1),
		uintptr(unsafe.Pointer(&bufptr)),
	)
	if r1 != nErrSuccess {
		err = windows.Errno(r1)
		return
	}

	// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/ns-lmaccess-user_info_1
	ui := (*userInfo1)(bufptr)
	u = &user{
		Name:  windows.UTF16PtrToString(ui.name),
		Priv:  ui.priv,
		Flags: ui.flags,
	}

	runtime.KeepAlive(n)
	runtime.KeepAlive(bufptr)

	return
}

// Adds local user to builtin\Administrators group.
func localUserAddToAdministrators(name string) error {
	administratorsGroupName, err := localGroupGetBuiltinAdministrators()
	if err != nil {
		return err
	}

	return localGroupAddMembers(name, administratorsGroupName)
}

// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/nf-lmaccess-netlocalgroupaddmembers
// Adds local users to arbitrary local group.
func localGroupAddMembers(username, groupname string) (err error) {
	groupPtr, err := windows.UTF16PtrFromString(groupname)
	if err != nil {
		return
	}

	sid, _, _, err := windows.LookupSID("", username)
	if err != nil {
		return
	}

	// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/ns-lmaccess-localgroup_members_info_0
	groupInfo := []struct {
		sid *windows.SID
	}{
		{
			sid: sid,
		},
	}

	// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/nf-lmaccess-netlocalgroupaddmembers
	r1, _, _ := procNetLocalGroupAddMembers.Call(
		uintptr(localSystem),
		uintptr(unsafe.Pointer(groupPtr)),
		uintptr(userLevel0),
		uintptr(unsafe.Pointer(&groupInfo[0])),
		uintptr(1), // totalentries
	)

	if r1 != nErrSuccess {
		err = windows.Errno(r1)
	}

	runtime.KeepAlive(groupPtr)
	runtime.KeepAlive(groupInfo)
	runtime.KeepAlive(sid)

	return
}

// Enables local user account.
func localUserEnable(username string) error {
	return localUserRemoveFlag(username, userFlagfAccountDisable)
}

// Removes arbitrary flag to local user.
func localUserRemoveFlag(username string, flag uint32) error {
	f, err := localUserGetFlag(username)
	if err != nil {
		return err
	}

	return localUserSetFlag(username, f&^flag)
}

// Sets password of arbitrary local user to never expire.
func localUserSetPasswordNeverExpire(username string) error {
	return localUserAddFlag(username, userFlagDontExpirePasswd)
}

// Adds arbitrary flag to local user.
func localUserAddFlag(username string, flag uint32) error {
	f, err := localUserGetFlag(username)
	if err != nil {
		return err
	}

	return localUserSetFlag(username, f|flag)
}

// Enables local user account.
func localUserIsDisabled(username string) (bool, error) {
	f, err := localUserGetFlag(username)
	if err != nil {
		return false, err
	}

	return f&userFlagfAccountDisable == userFlagfAccountDisable, nil
}

// Gets flag mask of arbitrary local user account.
func localUserGetFlag(username string) (uint32, error) {
	u, err := localUserGet(username)
	if err != nil {
		return 0, err
	}

	return u.Flags, nil
}

// Checks if password never expires of arbitrary local user.
func localUserIsPasswordNeverExpire(username string) (bool, error) {
	f, err := localUserGetFlag(username)
	if err != nil {
		return false, err
	}

	return f&userFlagDontExpirePasswd == userFlagDontExpirePasswd, nil
}

// Sets exact flag mask on arbitrary local user account.
func localUserSetFlag(username string, flag uint32) (err error) {
	namePtr, err := windows.UTF16PtrFromString(username)
	if err != nil {
		return
	}

	// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/ns-lmaccess-user_info_1008
	buf := struct {
		flags uint32
	}{
		flags: flag,
	}

	// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/nf-lmaccess-netusersetinfo
	r1, _, _ := procNetUserSetInfo.Call(
		uintptr(localSystem),
		uintptr(unsafe.Pointer(&namePtr)),
		uintptr(userLevel1008),
		uintptr(unsafe.Pointer(&buf)),
		uintptr(ignoreParamError),
	)

	if r1 != nErrSuccess {
		err = windows.Errno(r1)
	}

	runtime.KeepAlive(namePtr)
	runtime.KeepAlive(buf)

	return
}

// Sets password to local user account.
func localUserSetPassword(name, password string) (err error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return
	}

	passwordPtr, err := windows.UTF16PtrFromString(password)
	if err != nil {
		return
	}

	// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/ns-lmaccess-user_info_1003
	buf := struct {
		password *uint16 // LPWSTR
	}{
		passwordPtr,
	}

	// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/nf-lmaccess-netusersetinfo
	r1, _, _ := procNetUserSetInfo.Call(
		uintptr(localSystem),
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(userLevel1003),
		uintptr(unsafe.Pointer(&buf)),
		uintptr(ignoreParamError),
	)

	if r1 != nErrSuccess {
		err = windows.Errno(r1)
	}

	runtime.KeepAlive(namePtr)
	runtime.KeepAlive(passwordPtr)
	runtime.KeepAlive(buf)

	return
}

// Creates local user with given privileges and account flags.
func localUserCreate(name, password string, priv, flags uint32) (err error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return
	}

	passwordPtr, err := windows.UTF16PtrFromString(password)
	if err != nil {
		return
	}

	// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/ns-lmaccess-user_info_1
	// filling only needed fields
	buf := userInfo1{
		priv:     priv,
		name:     namePtr,
		password: passwordPtr,
		flags:    flags,
	}

	// https://docs.microsoft.com/en-us/windows/win32/api/lmaccess/nf-lmaccess-netuseradd
	r1, _, _ := procNetUserAdd.Call(
		uintptr(localSystem),
		uintptr(userLevel1),
		uintptr(unsafe.Pointer(&buf)),
		uintptr(ignoreParamError))

	if r1 != nErrSuccess {
		err = windows.Errno(r1)
	}

	runtime.KeepAlive(buf)
	runtime.KeepAlive(namePtr)
	runtime.KeepAlive(passwordPtr)

	return
}
