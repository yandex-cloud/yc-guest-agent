package users

const UserChangeResponseType = "UserChangeResponse"

// response is struct which converted to json and passed to COM port as result of user_handle execution.
type response struct {
	Modulus           string
	Exponent          string
	Username          string
	EncryptedPassword string
	Success           bool
	Error             string
}

// withRequest add request fields to resulting response.
func (res *response) withRequest(req request) *response {
	res.Modulus = req.Modulus
	res.Exponent = req.Exponent
	res.Username = req.Username

	return res
}

// withEncryptedPassword add EncryptedPassword field to resulting response.
func (res *response) withEncryptedPassword(p string) *response {
	res.EncryptedPassword = p

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
