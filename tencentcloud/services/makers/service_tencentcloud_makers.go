package makers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	tchttp "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/http"
	"github.com/tencentyun/cos-go-sdk-v5"

	tccommon "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/common"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/connectivity"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/ratelimit"
)

// tencentcloud-sdk-go has no makers package yet, so actions are sent as
// common requests against the makers product.
const (
	makersService = "makers"
	makersVersion = "2026-11-13"

	makersProjectSource = "terraform"

	makersErrResourceNotFound = "ResourceNotFound"
)

// makersSourceIgnore lists path segments that are neither hashed nor uploaded.
var makersSourceIgnore = map[string]bool{
	".DS_Store":    true,
	".edgeone":     true,
	".env":         true,
	".git":         true,
	"node_modules": true,
}

type MakersService struct {
	client *connectivity.TencentCloudClient
}

func NewMakersService(client *connectivity.TencentCloudClient) MakersService {
	return MakersService{client: client}
}

type makersResponse struct {
	*tchttp.BaseResponse
	Response json.RawMessage `json:"Response"`
}

func (r *makersResponse) ToJsonString() string {
	b, _ := json.Marshal(r)
	return string(b)
}

func (r *makersResponse) FromJsonString(s string) error {
	return json.Unmarshal([]byte(s), &r)
}

// makersParams flattens payload into action parameters and adds ZoneId, which
// every makers action requires.
func makersParams(zoneId string, payload interface{}) (map[string]interface{}, error) {
	params := map[string]interface{}{}
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		decoder := json.NewDecoder(strings.NewReader(string(body)))
		decoder.UseNumber()
		if err := decoder.Decode(&params); err != nil {
			return nil, err
		}
		params = makersNumbers(params).(map[string]interface{})
	}
	params["ZoneId"] = zoneId
	return params, nil
}

// makersNumbers replaces json.Number with int64 (or float64), because the SDK
// common request marshals json.Number as a string and the gateway rejects it.
func makersNumbers(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, e := range t {
			t[k] = makersNumbers(e)
		}
	case []interface{}:
		for i, e := range t {
			t[i] = makersNumbers(e)
		}
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return n
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
	}
	return v
}

// call sends one makers action and decodes `Response` into out. out may be
// nil for write actions that return nothing.
func (me MakersService) call(ctx context.Context, zoneId, action string, payload interface{}, out interface{}) error {
	logId := tccommon.GetLogId(ctx)

	params, err := makersParams(zoneId, payload)
	if err != nil {
		return err
	}
	request := tchttp.NewCommonRequest(makersService, makersVersion, action)
	if err := request.SetActionParameters(params); err != nil {
		return err
	}
	response := &makersResponse{BaseResponse: &tchttp.BaseResponse{}}

	ratelimit.Check(action)
	if err := me.client.UseMakersClient().Send(request, response); err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, reason[%s]\n", logId, action, err.Error())
		return err
	}
	log.Printf("[DEBUG]%s api[%s] success\n", logId, action)

	if out == nil || len(response.Response) == 0 {
		return nil
	}
	if err := json.Unmarshal(response.Response, out); err != nil {
		return fmt.Errorf("decode %s response: %w", action, err)
	}
	return nil
}

type makersFilter struct {
	Name   string   `json:"Name"`
	Values []string `json:"Values"`
}

type MakersProject struct {
	ProjectId     string  `json:"ProjectId"`
	Name          string  `json:"Name"`
	Status        string  `json:"Status"`
	Provider      string  `json:"Provider"`
	Area          string  `json:"Area"`
	ZoneId        string  `json:"ZoneId"`
	PresetDomain  string  `json:"PresetDomain"`
	NodejsVersion string  `json:"NodejsVersion"`
	RootDir       *string `json:"RootDir"`
	OutputDir     *string `json:"OutputDir"`
	BuildCmd      *string `json:"BuildCmd"`
	InstallCmd    *string `json:"InstallCmd"`
	Framework     *string `json:"Framework"`
	CreatedOn     string  `json:"CreatedOn"`
	ModifiedOn    string  `json:"ModifiedOn"`
}

type makersCreateProjectRequest struct {
	Name          string  `json:"Name"`
	Channel       string  `json:"Channel"`
	Provider      string  `json:"Provider"`
	Source        string  `json:"Source"`
	Area          *string `json:"Area,omitempty"`
	NodeJsVersion *string `json:"NodeJsVersion,omitempty"`
	RootDir       *string `json:"RootDir,omitempty"`
	OutputDir     *string `json:"OutputDir,omitempty"`
	BuildCmd      *string `json:"BuildCmd,omitempty"`
	InstallCmd    *string `json:"InstallCmd,omitempty"`
	Framework     *string `json:"Framework,omitempty"`
}

// makersModifyProjectRequest only carries the fields that changed: the backend
// rejects a Name equal to the current one as a duplicate.
type makersModifyProjectRequest struct {
	ProjectId     string  `json:"ProjectId"`
	Name          *string `json:"Name,omitempty"`
	NodeJsVersion *string `json:"NodeJsVersion,omitempty"`
	RootDir       *string `json:"RootDir,omitempty"`
	OutputDir     *string `json:"OutputDir,omitempty"`
	BuildCmd      *string `json:"BuildCmd,omitempty"`
	InstallCmd    *string `json:"InstallCmd,omitempty"`
	Framework     *string `json:"Framework,omitempty"`
}

func (me MakersService) CreateProject(ctx context.Context, zoneId string, request makersCreateProjectRequest) (string, error) {
	var result struct {
		ProjectId string `json:"ProjectId"`
	}
	if err := me.call(ctx, zoneId, "CreateProject", request, &result); err != nil {
		return "", err
	}
	if result.ProjectId == "" {
		return "", fmt.Errorf("CreateProject returned an empty ProjectId")
	}
	return result.ProjectId, nil
}

// DescribeProjectById returns nil when the project does not exist.
func (me MakersService) DescribeProjectById(ctx context.Context, zoneId, projectId string) (*MakersProject, error) {
	var result struct {
		Projects []MakersProject `json:"Projects"`
	}
	err := me.call(ctx, zoneId, "DescribeProjects", map[string]interface{}{
		"Filters": []makersFilter{{Name: "ProjectId", Values: []string{projectId}}},
		"Limit":   10,
	}, &result)
	if err != nil {
		if makersIsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	for i := range result.Projects {
		if result.Projects[i].ProjectId == projectId {
			return &result.Projects[i], nil
		}
	}
	return nil, nil
}

func (me MakersService) ModifyProject(ctx context.Context, zoneId string, request makersModifyProjectRequest) error {
	return me.call(ctx, zoneId, "ModifyProject", request, nil)
}

func (me MakersService) DeleteProject(ctx context.Context, zoneId, projectId string) error {
	return me.call(ctx, zoneId, "DeleteProject", map[string]interface{}{"ProjectId": projectId}, nil)
}

type MakersEnvVar struct {
	Id      *int64   `json:"Id,omitempty"`
	Key     string   `json:"Key"`
	Value   string   `json:"Value"`
	Comment *string  `json:"Comment,omitempty"`
	Env     []string `json:"Env,omitempty"`
}

func (me MakersService) DescribeProjectEnvVars(ctx context.Context, zoneId, projectId string) ([]MakersEnvVar, error) {
	var result struct {
		EnvVars []MakersEnvVar `json:"EnvVars"`
	}
	if err := me.call(ctx, zoneId, "DescribeProjectEnvs", map[string]interface{}{"ProjectId": projectId}, &result); err != nil {
		return nil, err
	}
	return result.EnvVars, nil
}

func (me MakersService) ModifyProjectEnvVars(ctx context.Context, zoneId, projectId string, envVars []MakersEnvVar) error {
	if len(envVars) == 0 {
		return nil
	}
	return me.call(ctx, zoneId, "ModifyProjectEnvs", map[string]interface{}{
		"ProjectId": projectId,
		"EnvVars":   envVars,
	}, nil)
}

// DeleteProjectEnvVarsById deletes single rows. Deleting by Key would remove
// every row of that key, including rows scoped to other environments.
func (me MakersService) DeleteProjectEnvVarsById(ctx context.Context, zoneId, projectId string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	items := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		// Value is required to be non-empty but is not used for matching.
		items = append(items, map[string]interface{}{"Id": id, "Key": "-", "Value": "-"})
	}
	return me.call(ctx, zoneId, "DeleteProjectEnvs", map[string]interface{}{
		"ProjectId": projectId,
		"EnvVars":   items,
	}, nil)
}

type MakersEnvironment struct {
	Slug string `json:"Slug"`
	Name string `json:"Name"`
	Type string `json:"Type"`
}

func (me MakersService) DescribeEnvironments(ctx context.Context, zoneId, projectId string) ([]MakersEnvironment, error) {
	var result struct {
		Environments []MakersEnvironment `json:"Environments"`
	}
	if err := me.call(ctx, zoneId, "DescribeEnvironments", map[string]interface{}{"ProjectId": projectId}, &result); err != nil {
		return nil, err
	}
	return result.Environments, nil
}

type MakersDeployment struct {
	DeploymentId string `json:"DeploymentId"`
	ProjectId    string `json:"ProjectId"`
	Status       string `json:"Status"`
	Code         int64  `json:"Code"`
	BuildCost    string `json:"BuildCost"`
	Env          string `json:"Env"`
	PreviewUrl   string `json:"PreviewUrl"`
	ProjectUrl   string `json:"ProjectUrl"`
	UsedInProd   bool   `json:"UsedInProd"`
	CreatedOn    string `json:"CreatedOn"`
}

func makersDeploymentFinished(status string) bool {
	return status != "Pending" && status != "Process"
}

type makersCosTempToken struct {
	Bucket      string `json:"Bucket"`
	Region      string `json:"Region"`
	TargetPath  string `json:"TargetPath"`
	Credentials struct {
		TmpSecretId  string `json:"TmpSecretId"`
		TmpSecretKey string `json:"TmpSecretKey"`
		Token        string `json:"Token"`
	} `json:"Credentials"`
}

const (
	makersDistTypeFolder = "Folder"
	makersDistTypeZip    = "Zip"
)

// tempCosClient returns a COS client bound to the temporary credentials the
// platform issues for the project, together with the target path to upload to.
func (me MakersService) tempCosClient(ctx context.Context, zoneId, projectId string) (*cos.Client, string, error) {
	project, err := me.DescribeProjectById(ctx, zoneId, projectId)
	if err != nil {
		return nil, "", err
	}
	if project == nil {
		return nil, "", fmt.Errorf("project %s does not exist", projectId)
	}
	var token makersCosTempToken
	err = me.call(ctx, zoneId, "DescribeCosTempToken", map[string]interface{}{
		"ProjectId":   projectId,
		"ProjectName": project.Name,
	}, &token)
	if err != nil {
		return nil, "", err
	}
	bucketURL, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", token.Bucket, token.Region))
	if err != nil {
		return nil, "", err
	}
	client := cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Timeout: 10 * time.Minute,
		Transport: &cos.AuthorizationTransport{
			SecretID:     token.Credentials.TmpSecretId,
			SecretKey:    token.Credentials.TmpSecretKey,
			SessionToken: token.Credentials.Token,
		},
	})
	return client, token.TargetPath, nil
}

// UploadSourceZip uploads a zip archive to the temporary COS path of the
// project and returns its object key, which is what a Zip deployment expects.
func (me MakersService) UploadSourceZip(ctx context.Context, zoneId, projectId, file string) (string, error) {
	if err := makersCheckZip(file); err != nil {
		return "", err
	}
	cosClient, targetPath, err := me.tempCosClient(ctx, zoneId, projectId)
	if err != nil {
		return "", err
	}
	key := targetPath + "/" + filepath.Base(file)
	if err := makersPutFile(ctx, cosClient, key, file); err != nil {
		return "", err
	}
	return key, nil
}

// UploadSource uploads every file of dir (ignored segments excluded) to the
// temporary COS path of the project and returns that path.
func (me MakersService) UploadSource(ctx context.Context, zoneId, projectId, dir string) (string, int, error) {
	files, err := makersSourceFiles(dir)
	if err != nil {
		return "", 0, err
	}
	if len(files) == 0 {
		return "", 0, fmt.Errorf("source directory %s has no files to upload", dir)
	}

	cosClient, targetPath, err := me.tempCosClient(ctx, zoneId, projectId)
	if err != nil {
		return "", 0, err
	}

	const workers = 8
	jobs := make(chan string)
	errs := make(chan error, len(files))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for rel := range jobs {
				errs <- makersPutFile(ctx, cosClient, targetPath+"/"+rel, filepath.Join(dir, filepath.FromSlash(rel)))
			}
		}()
	}
	for _, rel := range files {
		jobs <- rel
	}
	close(jobs)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return "", 0, err
		}
	}
	return targetPath, len(files), nil
}

func makersPutFile(ctx context.Context, client *cos.Client, key, file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err := client.Object.Put(ctx, key, f, nil); err != nil {
		return fmt.Errorf("upload %s: %w", key, err)
	}
	return nil
}

// CreateDeployment builds what was uploaded to targetPath: a directory prefix
// for the Folder dist type, or the object key of the archive for Zip.
func (me MakersService) CreateDeployment(ctx context.Context, zoneId, projectId, env, distType, targetPath string) (string, error) {
	var result struct {
		DeploymentId string `json:"DeploymentId"`
	}
	err := me.call(ctx, zoneId, "CreateDeployment", map[string]interface{}{
		"ProjectId":      projectId,
		"ViaMeta":        "Upload",
		"Provider":       "Upload",
		"Env":            env,
		"DistType":       distType,
		"TempBucketPath": targetPath,
	}, &result)
	if err != nil {
		return "", err
	}
	if result.DeploymentId == "" {
		return "", fmt.Errorf("CreateDeployment returned an empty DeploymentId")
	}
	return result.DeploymentId, nil
}

// DescribeDeploymentById returns nil when the deployment does not exist.
func (me MakersService) DescribeDeploymentById(ctx context.Context, zoneId, projectId, deploymentId string) (*MakersDeployment, error) {
	var result struct {
		Deployments []MakersDeployment `json:"Deployments"`
	}
	err := me.call(ctx, zoneId, "DescribeDeployments", map[string]interface{}{
		"ProjectId":     projectId,
		"DeploymentIds": []string{deploymentId},
	}, &result)
	if err != nil {
		if makersIsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	for i := range result.Deployments {
		if result.Deployments[i].DeploymentId == deploymentId {
			return &result.Deployments[i], nil
		}
	}
	return nil, nil
}

// DeploymentLogTail returns the last lines of the build log, errors marked.
func (me MakersService) DeploymentLogTail(ctx context.Context, zoneId, projectId, deploymentId string, lines int) (string, error) {
	var result struct {
		LogUrl string `json:"LogUrl"`
	}
	err := me.call(ctx, zoneId, "DescribeDeploymentLog", map[string]interface{}{
		"ProjectId":    projectId,
		"DeploymentId": deploymentId,
	}, &result)
	if err != nil || result.LogUrl == "" {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, result.LogUrl, nil)
	if err != nil {
		return "", err
	}
	resp, err := (&http.Client{Timeout: time.Minute}).Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return makersFormatLog(raw, lines), nil
}

var makersAnsiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// makersFormatLog renders the NDJSON build log: each line is
// {"t":"i"|"e","ls":[[timestampMs, message], ...]}.
func makersFormatLog(raw []byte, lines int) string {
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry struct {
			T  string              `json:"t"`
			Ls [][]json.RawMessage `json:"ls"`
		}
		if json.Unmarshal([]byte(line), &entry) != nil {
			continue
		}
		for _, l := range entry.Ls {
			if len(l) < 2 {
				continue
			}
			var msg string
			if json.Unmarshal(l[1], &msg) != nil {
				continue
			}
			msg = makersAnsiEscape.ReplaceAllString(msg, "")
			if entry.T == "e" {
				msg = "ERROR " + msg
			}
			out = append(out, msg)
		}
	}
	if lines > 0 && len(out) > lines {
		out = out[len(out)-lines:]
	}
	return strings.Join(out, "\n")
}

type MakersOwnershipVerification struct {
	DnsVerification *struct {
		RecordType  string `json:"RecordType"`
		Subdomain   string `json:"Subdomain"`
		RecordValue string `json:"RecordValue"`
	} `json:"DnsVerification"`
	FileVerification *struct {
		Path    string `json:"Path"`
		Content string `json:"Content"`
	} `json:"FileVerification"`
}

type MakersDomain struct {
	Type                  string                       `json:"Type"`
	Domain                string                       `json:"Domain"`
	Env                   string                       `json:"Env"`
	Status                string                       `json:"Status"`
	Cname                 *string                      `json:"Cname"`
	OwnershipVerification *MakersOwnershipVerification `json:"OwnershipVerification"`
}

func (me MakersService) CreateCustomDomain(ctx context.Context, zoneId, projectId, domain, env string) error {
	return me.call(ctx, zoneId, "CreateZoneCustomDomain", map[string]interface{}{
		"ProjectId": projectId,
		"Domain":    domain,
		"Env":       env,
	}, nil)
}

// DescribeCustomDomain returns nil when the domain is not bound to the project.
func (me MakersService) DescribeCustomDomain(ctx context.Context, zoneId, projectId, domain string) (*MakersDomain, error) {
	var result struct {
		MakersDomains []MakersDomain `json:"PagesDomains"`
	}
	err := me.call(ctx, zoneId, "DescribeZoneCustomDomains", map[string]interface{}{"ProjectId": projectId}, &result)
	if err != nil {
		if makersIsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	for i := range result.MakersDomains {
		d := &result.MakersDomains[i]
		if d.Type == "Custom" && strings.EqualFold(d.Domain, domain) {
			return d, nil
		}
	}
	return nil, nil
}

func (me MakersService) ModifyCustomDomainEnv(ctx context.Context, zoneId, projectId, domain, env string) error {
	return me.call(ctx, zoneId, "ModifyZoneCustomDomainEnv", map[string]interface{}{
		"ProjectId": projectId,
		"Domain":    domain,
		"Env":       env,
	}, nil)
}

func (me MakersService) DeleteCustomDomain(ctx context.Context, zoneId, projectId, domain string) error {
	return me.call(ctx, zoneId, "DeleteProjectCustomDomains", map[string]interface{}{
		"ProjectId":     projectId,
		"CustomDomains": []string{domain},
	}, nil)
}

func makersIsNotFound(err error) bool {
	return makersErrorCode(err) == makersErrResourceNotFound
}

func makersErrorCode(err error) string {
	type coder interface{ GetCode() string }
	if c, ok := err.(coder); ok {
		return c.GetCode()
	}
	return ""
}

// makersSourceFiles lists files under dir as slash-separated relative paths,
// sorted, skipping any path that contains an ignored segment.
func makersSourceFiles(dir string) ([]string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	var files []string
	err = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if makersSourceIgnore[d.Name()] && p != dir {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// makersSourceHash hashes "<path> <sha256>" lines of every uploaded file, so
// renames and content changes both change the result.
func makersSourceHash(dir string) (string, error) {
	files, err := makersSourceFiles(dir)
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(files))
	for _, rel := range files {
		sum, err := makersFileSha256(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return "", err
		}
		lines = append(lines, rel+" "+sum)
	}
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

// makersCheckZip rejects anything that is not a regular file with a zip
// signature, so a wrong path fails at plan rather than in the platform build.
func makersCheckZip(file string) error {
	info, err := os.Stat(file)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", file)
	}
	if !strings.EqualFold(filepath.Ext(file), ".zip") {
		return fmt.Errorf("%s must have the .zip extension", file)
	}
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil || string(magic) != "PK\x03\x04" {
		return fmt.Errorf("%s is not a zip archive", file)
	}
	return nil
}

// makersSourceZipHash is the sha256 of the archive itself.
func makersSourceZipHash(file string) (string, error) {
	if err := makersCheckZip(file); err != nil {
		return "", err
	}
	return makersFileSha256(file)
}

func makersFileSha256(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
