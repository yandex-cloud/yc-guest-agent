package sshkeys

import "marketplace-yaga/linux/internal/usermanager"

const UserUpdateSshKeysResponseType = "UserUpdateSshKeys"

// response is struct which converted to json and passed to COM port as result of user_handle execution.
type response struct {
	Users   []usermanager.User `json:"users"`
	Success bool               `json:"success"`
	Error   string             `json:"error"`
}

// withUsers add parsed users to resulting response.
func (res *response) withUsers(users []usermanager.User) *response {
	res.Users = users

	return res
}

// withSuccess changes Success field of resulting response to true.
func (res *response) withSuccess() *response {
	res.Success = true

	return res
}

// withError add error string to resulting response.
func (res *response) withError(e error) *response {
	res.Error = e.Error()

	return res
}
