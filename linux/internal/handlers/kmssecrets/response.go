package kmssecrets

const KmsSecretsResponseType = "KmsSecrets"

// response is struct which converted to json and passed to COM port as result of user_handle execution.
type response struct {
	Files   []string
	Success bool
	Error   string
}

// withFiles add parsed users to resulting response.
func (res *response) withFiles(files []string) *response {
	res.Files = files

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
