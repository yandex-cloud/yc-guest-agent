package managedcertificates

const ManagedCertificatesResponseType = "Certificate"

// response is struct which converted to json and passed to COM port as result of user_handle execution.
type response struct {
	Success      bool     `json:"success"`
	Error        string   `json:"error"`
	Certificates []string `json:"managedcertificates"`
}

// withFiles add parsed users to resulting response.
func (res *response) withCertificates(certs []string) *response {
	res.Certificates = certs

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
