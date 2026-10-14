package mfcli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/warriorguo/memory_flow/backend/internal/model"
)

func TestParseCICDLink(t *testing.T) {
	cases := map[string]model.CICDLink{
		"server=https://cicd/apps/36": {Label: "server", URL: "https://cicd/apps/36"},
		" admin = https://cicd/38 ":   {Label: "admin", URL: "https://cicd/38"},
		"https://cicd/apps/36":        {URL: "https://cicd/apps/36"},
		// '=' inside a bare URL's query string is not a label separator.
		"https://cicd/run?app=36":    {URL: "https://cicd/run?app=36"},
		"web=https://cicd/run?app=1": {Label: "web", URL: "https://cicd/run?app=1"},
	}
	for in, want := range cases {
		var f cicdLinksFlag
		if err := f.Set(in); err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if len(f) != 1 || f[0] != want {
			t.Errorf("%q parsed as %+v, want %+v", in, f, want)
		}
	}
}

const apmProject = `{"data":{"id":"22222222-2222-2222-2222-222222222222","key":"APM","name":"OZX APM",` +
	`"status":"active","cicd_url":"https://cicd/apps/36","cicd_links":[` +
	`{"label":"server","url":"https://cicd/apps/36"},{"label":"admin","url":"https://cicd/apps/38"}]}}`

func TestProjectUpdateSendsCICDLinks(t *testing.T) {
	stub, srv := newStubAPI(t, map[string]string{"PUT /api/v1/projects/APM": apmProject})

	_, stderr, code := runCLI(t, srv.URL, "project", "update", "APM",
		"--cicd-url", "server=https://cicd/apps/36", "--cicd-url", "admin=https://cicd/apps/38")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var req model.UpdateProjectRequest
	if err := json.Unmarshal([]byte(stub.bodies["PUT /api/v1/projects/APM"]), &req); err != nil {
		t.Fatal(err)
	}
	if req.CICDLinks == nil || len(*req.CICDLinks) != 2 || (*req.CICDLinks)[1].Label != "admin" {
		t.Fatalf("cicd_links = %+v", req.CICDLinks)
	}
	// The first URL rides along as cicd_url for servers that predate cicd_links.
	if req.CICDURL == nil || *req.CICDURL != "https://cicd/apps/36" {
		t.Errorf("cicd_url = %v", req.CICDURL)
	}
	// Fields that were not given stay omitted.
	if req.Name != nil || req.GitURL != nil {
		t.Errorf("unset fields were sent: %+v", req)
	}
}

func TestProjectUpdateClearsCICDLinks(t *testing.T) {
	stub, srv := newStubAPI(t, map[string]string{"PUT /api/v1/projects/APM": apmProject})

	if _, stderr, code := runCLI(t, srv.URL, "project", "update", "APM", "--cicd-url", ""); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	body := stub.bodies["PUT /api/v1/projects/APM"]
	if !strings.Contains(body, `"cicd_links":[]`) || !strings.Contains(body, `"cicd_url":""`) {
		t.Errorf("body = %s, want an empty cicd_links and cicd_url", body)
	}
}

func TestProjectUpdateLeavesCICDLinksAlone(t *testing.T) {
	stub, srv := newStubAPI(t, map[string]string{"PUT /api/v1/projects/APM": apmProject})

	if _, stderr, code := runCLI(t, srv.URL, "project", "update", "APM", "--name", "x"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	body := stub.bodies["PUT /api/v1/projects/APM"]
	if !strings.Contains(body, `"cicd_links":null`) || !strings.Contains(body, `"cicd_url":null`) {
		t.Errorf("body = %s, want cicd fields left unset", body)
	}
}

func TestProjectShowAndCtxListCICDLinks(t *testing.T) {
	list := `{"data":[` + strings.TrimSuffix(strings.TrimPrefix(apmProject, `{"data":`), `}`) + `]}`
	_, srv := newStubAPI(t, map[string]string{
		"GET /api/v1/projects/APM": apmProject,
		"GET /api/v1/projects":     list,
	})

	out, stderr, code := runCLI(t, srv.URL, "project", "show", "APM")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, want := range []string{
		"  cicd          server  https://cicd/apps/36\n",
		"                admin   https://cicd/apps/38\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("project show missing %q:\n%s", want, out)
		}
	}

	out, stderr, code = runCLI(t, srv.URL, "ctx")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(out, "CI/CD:") || !strings.Contains(out, "admin") ||
		!strings.Contains(out, "https://cicd/apps/38") {
		t.Errorf("ctx does not list the CI/CD links:\n%s", out)
	}
}
