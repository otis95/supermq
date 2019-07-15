package auth

type iotopoAuthenticator struct {
	Username string
	Password string
}
var iotopo iotopoAuthenticator
func (this *iotopoAuthenticator) Authenticate(id string, cred interface{}) error {
	if id == this.Username && cred == this.Password {
		return nil
	}

	return ErrAuthFailure
}

func RegisterBasicAuth(username, password string) {
	Register("basic", &iotopoAuthenticator{
		Username: username,
		Password: password,
	})
}
