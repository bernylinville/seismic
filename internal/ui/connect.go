package ui

import (
	"seismic/internal/conn"
	"seismic/internal/es"
)

// connectForm is the state of the add-connection form on the welcome page.
type connectForm struct {
	testing bool
	tested  bool
	testOK  bool
	testMsg string

	Name            string
	URI             string
	AuthType        string // es.AuthType 值；空为 none
	Username        string
	Password        string
	APIKey          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

var authChoices = []struct {
	value es.AuthType
	label string
}{
	{es.AuthNone, "无认证"},
	{es.AuthBasic, "Basic"},
	{es.AuthAPIKey, "API Key"},
	{es.AuthAWSIAM, "AWS IAM"},
}

func (f *connectForm) auth() es.Auth {
	switch es.AuthType(f.AuthType) {
	case es.AuthBasic:
		return es.Auth{Type: es.AuthBasic, Username: f.Username, Password: f.Password}
	case es.AuthAPIKey:
		return es.Auth{Type: es.AuthAPIKey, Password: f.APIKey}
	case es.AuthAWSIAM:
		return es.Auth{Type: es.AuthAWSIAM, AccessKeyID: f.AccessKeyID,
			SecretAccessKey: f.SecretAccessKey, SessionToken: f.SessionToken, Region: f.Region}
	default:
		return es.Auth{Type: es.AuthNone}
	}
}

func authLabel(v string) string {
	for _, ch := range authChoices {
		if string(ch.value) == v {
			return ch.label
		}
	}
	return authChoices[0].label
}

// testConnection pings the cluster with the form's values.
func (a *App) testConnection() {
	f := &a.connect
	f.testing = true
	f.tested = false
	a.invalidate()
	go func() {
		_, info, err := conn.TestConnection(a.backgroundCtx(), f.URI, f.auth())
		a.update(func() {
			f.testing = false
			f.tested = true
			if err != nil {
				f.testOK = false
				f.testMsg = err.Error()
				return
			}
			f.testOK = true
			f.testMsg = info.ClusterName + " · " + info.Distribution + " " + info.Version
		})
	}()
}

// SetConnectForm fills the add-connection form (tests and deep links).
func (a *App) SetConnectForm(name, uri, authType string) {
	f := &a.connect
	f.Name, f.URI, f.AuthType = name, uri, authType
}

// ConnectTestMessage reports the last test/connect error (tests).
func (a *App) ConnectTestMessage() string { return a.connect.testMsg }
