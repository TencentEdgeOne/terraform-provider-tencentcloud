package teo

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

// Makers interfaces are not exposed as typed actions in the SDK. They
// are reached through four TEO actions that forward `Interface` with a JSON
// `Payload`, and the business result comes back as a JSON string in `Result`.
// The verb of the action must match the verb of the interface.
const (
	makersActionDescribe = "DescribePagesResources"
	makersActionCreate   = "CreatePagesResources"
	makersActionModify   = "ModifyPagesResources"
	makersActionDelete   = "DeletePagesResources"

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

type TeoMakersService struct {
	client *connectivity.TencentCloudClient
}

func NewTeoMakersService(client *connectivity.TencentCloudClient) TeoMakersService {
	return TeoMakersService{client: client}
}

type makersForwardResponse struct {
	*tchttp.BaseResponse
	Response *struct {
		Result    *string `json:"Result,omitempty"`
		RequestId *string `json:"RequestId,omitempty"`
	} `json:"Response"`
}

func (r *makersForwardResponse) ToJsonString() string {
	b, _ := json.Marshal(r)
	return string(b)
}

func (r *makersForwardResponse) FromJsonString(s string) error {
	return json.Unmarshal([]byte(s), &r)
}

func makersActionOf(iface string) (string, error) {
	switch {
	case strings.HasPrefix(iface, "Describe"):
		return makersActionDescribe, nil
	case strings.HasPrefix(iface, "Create"), strings.HasPrefix(iface, "Bind"):
		return makersActionCreate, nil
	case strings.HasPrefix(iface, "Modify"):
		return makersActionModify, nil
	case strings.HasPrefix(iface, "Delete"), strings.HasPrefix(iface, "Unbind"):
		return makersActionDelete, nil
	}
	return "", fmt.Errorf("unsupported Makers interface %s", iface)
}

// call sends one Makers interface and decodes `Result` into out. out may be
// nil for write interfaces that return no result.
func (me TeoMakersService) call(ctx context.Context, zoneId, iface string, payload interface{}, out interface{}) error {
	logId := tccommon.GetLogId(ctx)

	action, err := makersActionOf(iface)
	if err != nil {
		return err
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request := tchttp.NewCommonRequest("teo", "2022-09-01", action)
	if err := request.SetActionParameters(map[string]interface{}{
		"ZoneId":    zoneId,
		"Interface": "pages:" + iface,
		"Payload":   string(body),
	}); err != nil {
		return err
	}
	response := &makersForwardResponse{BaseResponse: &tchttp.BaseResponse{}}

	ratelimit.Check(action)
	if err := me.client.UseTeoClient().Send(request, response); err != nil {
		log.Printf("[CRITAL]%s api[%s] fail, reason[%s]\n", logId, iface, err.Error())
		return err
	}
	log.Printf("[DEBUG]%s api[%s] success\n", logId, iface)

	if out == nil || response.Response == nil || response.Response.Result == nil || *response.Response.Result == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(*response.Response.Result), out); err != nil {
		return fmt.Errorf("decode %s result: %w", iface, err)
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
	NodejsVersion *string `json:"NodejsVersion,omitempty"`
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
	NodejsVersion *string `json:"NodejsVersion,omitempty"`
	RootDir       *string `json:"RootDir,omitempty"`
	OutputDir     *string `json:"OutputDir,omitempty"`
	BuildCmd      *string `json:"BuildCmd,omitempty"`
	InstallCmd    *string `json:"InstallCmd,omitempty"`
	Framework     *string `json:"Framework,omitempty"`
}

func (me TeoMakersService) CreateProject(ctx context.Context, zoneId string, request makersCreateProjectRequest) (string, error) {
	var result struct {
		ProjectId string `json:"ProjectId"`
	}
	if err := me.call(ctx, zoneId, "CreatePagesProject", request, &result); err != nil {
		return "", err
	}
	if result.ProjectId == "" {
		return "", fmt.Errorf("CreatePagesProject returned an empty ProjectId")
	}
	return result.ProjectId, nil
}

// DescribeProjectById returns nil when the project does not exist.
func (me TeoMakersService) DescribeProjectById(ctx context.Context, zoneId, projectId string) (*MakersProject, error) {
	var result struct {
		Projects []MakersProject `json:"Projects"`
	}
	err := me.call(ctx, zoneId, "DescribePagesProjects", map[string]interface{}{
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

func (me TeoMakersService) ModifyProject(ctx context.Context, zoneId string, request makersModifyProjectRequest) error {
	return me.call(ctx, zoneId, "ModifyPagesProject", request, nil)
}

func (me TeoMakersService) DeleteProject(ctx context.Context, zoneId, projectId string) error {
	return me.call(ctx, zoneId, "DeletePagesProject", map[string]interface{}{"ProjectId": projectId}, nil)
}

type MakersEnvVar struct {
	Id      *int64   `json:"Id,omitempty"`
	Key     string   `json:"Key"`
	Value   string   `json:"Value"`
	Comment *string  `json:"Comment,omitempty"`
	Env     []string `json:"Env,omitempty"`
}

func (me TeoMakersService) DescribeProjectEnvVars(ctx context.Context, zoneId, projectId string) ([]MakersEnvVar, error) {
	var result struct {
		EnvVars []MakersEnvVar `json:"EnvVars"`
	}
	if err := me.call(ctx, zoneId, "DescribePagesProjectEnvs", map[string]interface{}{"ProjectId": projectId}, &result); err != nil {
		return nil, err
	}
	return result.EnvVars, nil
}

func (me TeoMakersService) ModifyProjectEnvVars(ctx context.Context, zoneId, projectId string, envVars []MakersEnvVar) error {
	if len(envVars) == 0 {
		return nil
	}
	return me.call(ctx, zoneId, "ModifyPagesProjectEnvs", map[string]interface{}{
		"ProjectId": projectId,
		"EnvVars":   envVars,
	}, nil)
}

// DeleteProjectEnvVarsById deletes single rows. Deleting by Key would remove
// every row of that key, including rows scoped to other environments.
func (me TeoMakersService) DeleteProjectEnvVarsById(ctx context.Context, zoneId, projectId string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	items := make([]map[string]interface{}, 0, len(ids))
	for _, id := range ids {
		// Value is required to be non-empty but is not used for matching.
		items = append(items, map[string]interface{}{"Id": id, "Key": "-", "Value": "-"})
	}
	return me.call(ctx, zoneId, "DeletePagesProjectEnvs", map[string]interface{}{
		"ProjectId": projectId,
		"EnvVars":   items,
	}, nil)
}

type MakersEnvironment struct {
	Slug string `json:"Slug"`
	Name string `json:"Name"`
	Type string `json:"Type"`
}

func (me TeoMakersService) DescribeEnvironments(ctx context.Context, zoneId, projectId string) ([]MakersEnvironment, error) {
	var result struct {
		Environments []MakersEnvironment `json:"Environments"`
	}
	if err := me.call(ctx, zoneId, "DescribePagesEnvironments", map[string]interface{}{"ProjectId": projectId}, &result); err != nil {
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
func (me TeoMakersService) tempCosClient(ctx context.Context, zoneId, projectId string) (*cos.Client, string, error) {
	var token makersCosTempToken
	if err := me.call(ctx, zoneId, "DescribePagesCosTempToken", map[string]interface{}{"ProjectId": projectId}, &token); err != nil {
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
func (me TeoMakersService) UploadSourceZip(ctx context.Context, zoneId, projectId, file string) (string, error) {
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
func (me TeoMakersService) UploadSource(ctx context.Context, zoneId, projectId, dir string) (string, int, error) {
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
func (me TeoMakersService) CreateDeployment(ctx context.Context, zoneId, projectId, env, distType, targetPath string) (string, error) {
	var result struct {
		DeploymentId string `json:"DeploymentId"`
	}
	err := me.call(ctx, zoneId, "CreatePagesDeployment", map[string]interface{}{
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
		return "", fmt.Errorf("CreatePagesDeployment returned an empty DeploymentId")
	}
	return result.DeploymentId, nil
}

// DescribeDeploymentById returns nil when the deployment does not exist.
func (me TeoMakersService) DescribeDeploymentById(ctx context.Context, zoneId, projectId, deploymentId string) (*MakersDeployment, error) {
	var result struct {
		Deployments []MakersDeployment `json:"Deployments"`
	}
	err := me.call(ctx, zoneId, "DescribePagesDeployments", map[string]interface{}{
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
func (me TeoMakersService) DeploymentLogTail(ctx context.Context, zoneId, projectId, deploymentId string, lines int) (string, error) {
	var result struct {
		LogUrl string `json:"LogUrl"`
	}
	err := me.call(ctx, zoneId, "DescribePagesDeploymentLog", map[string]interface{}{
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
	CurrentCname          string                       `json:"CurrentCname"`
	OwnershipVerification *MakersOwnershipVerification `json:"OwnershipVerification"`
}

func (me TeoMakersService) CreateCustomDomain(ctx context.Context, zoneId, projectId, domain, env string) error {
	return me.call(ctx, zoneId, "CreatePagesZoneCustomDomain", map[string]interface{}{
		"ProjectId": projectId,
		"Domain":    domain,
		"Env":       env,
	}, nil)
}

// DescribeCustomDomain returns nil when the domain is not bound to the project.
func (me TeoMakersService) DescribeCustomDomain(ctx context.Context, zoneId, projectId, domain string) (*MakersDomain, error) {
	var result struct {
		MakersDomains []MakersDomain `json:"PagesDomains"`
	}
	err := me.call(ctx, zoneId, "DescribePagesZoneCustomDomains", map[string]interface{}{"ProjectId": projectId}, &result)
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

func (me TeoMakersService) ModifyCustomDomainEnv(ctx context.Context, zoneId, projectId, domain, env string) error {
	return me.call(ctx, zoneId, "ModifyPagesZoneCustomDomainEnv", map[string]interface{}{
		"ProjectId": projectId,
		"Domain":    domain,
		"Env":       env,
	}, nil)
}

func (me TeoMakersService) DeleteCustomDomain(ctx context.Context, zoneId, projectId, domain string) error {
	return me.call(ctx, zoneId, "DeletePagesProjectCustomDomains", map[string]interface{}{
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
