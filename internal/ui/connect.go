package ui

import (
	"context"

	"seismic/internal/es"
)

// connectForm is the state of the add-connection sheet on the welcome page.
type connectForm struct {
	open    bool
	testing bool
	tested  bool
	testOK  bool
	testMsg string

	Name            string
	URI             string
	AuthType        string // es.AuthType 值
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
	{es.AuthBasic, "Basic 用户名/密码"},
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

// testConnection pings the cluster with the form's values.
func (a *App) testConnection() {
	f := &a.connect
	f.testing = true
	f.tested = false
	a.invalidate()
	go func() {
		client, err := es.New(f.URI, f.auth())
		var info map[string]any
		if err == nil {
			var resp *es.Response
			resp, err = client.Request(context.Background(), "GET", "", nil)
			if err == nil {
				err = resp.JSON(&info)
			}
		}
		a.update(func() {
			f.testing = false
			f.tested = true
			if err != nil {
				f.testOK = false
				f.testMsg = err.Error()
				return
			}
			f.testOK = true
			f.testMsg = clusterSummary(info)
		})
	}()
}

func clusterSummary(info map[string]any) string {
	name, _ := info["cluster_name"].(string)
	ver, _ := info["version"].(map[string]any)
	num, _ := ver["number"].(string)
	dist, _ := ver["distribution"].(string)
	if dist == "" {
		dist = "elasticsearch"
	}
	s := name + " · " + dist + " " + num
	return s
}
