package stat

type ConfigUserAuthenticator struct {
	Authenticator
}

func (a *ConfigUserAuthenticator) CheckHash(hash string) bool {
	return true
}

func (a *ConfigUserAuthenticator) Close() error {
	return nil
}

type MixedAuthenticator struct {
	configAuth Authenticator
	Authenticator
}

func (a *MixedAuthenticator) CheckHash(hash string) bool {
	return true
}

func (a *MixedAuthenticator) Close() error {
	return nil
}

func NewMixedAuthenticator() (Authenticator, error) {
	a := &MixedAuthenticator{
		configAuth: &ConfigUserAuthenticator{
		},
	}
	return a, nil
}