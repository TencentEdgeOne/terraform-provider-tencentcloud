package teo

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func makersInt64(v int64) *int64    { return &v }
func makersString(v string) *string { return &v }

func TestMakersActionOf(t *testing.T) {
	cases := map[string]string{
		"DescribePagesProjects":          makersActionDescribe,
		"CreatePagesDeployment":          makersActionCreate,
		"ModifyPagesZoneCustomDomainEnv": makersActionModify,
		"DeletePagesProjectEnvs":         makersActionDelete,
	}
	for iface, want := range cases {
		got, err := makersActionOf(iface)
		if err != nil || got != want {
			t.Errorf("makersActionOf(%s) = %s, %v; want %s", iface, got, err, want)
		}
	}
	if _, err := makersActionOf("VerifyPagesProjectDomains"); err == nil {
		t.Error("expected an error for an interface without a CRUD verb")
	}
}

func TestMakersReconcileEnvVars(t *testing.T) {
	all := []string{"Production", "Preview", "Staging"}
	remote := []MakersEnvVar{
		{Id: makersInt64(1), Key: "SAME", Value: "v", Comment: makersString(""), Env: all},
		{Id: makersInt64(2), Key: "CHANGED", Value: "old", Env: []string{"Preview", "Production", "Staging"}},
		{Id: makersInt64(3), Key: "STALE", Value: "x", Env: all},
		{Id: makersInt64(4), Key: "SCOPED", Value: "p", Env: []string{"Production"}},
		{Id: makersInt64(5), Key: "SCOPED", Value: "rest", Env: []string{"Preview", "Staging"}},
	}
	empty := ""
	wanted := []MakersEnvVar{
		{Key: "SAME", Value: "v", Comment: &empty},
		{Key: "CHANGED", Value: "new", Comment: &empty},
		{Key: "SCOPED", Value: "p", Comment: &empty, Env: []string{"Production"}},
		{Key: "SCOPED", Value: "preview", Comment: &empty, Env: []string{"Preview"}},
		{Key: "NEW", Value: "n", Comment: &empty},
	}

	deletes, upserts := makersReconcileEnvVars(remote, wanted, all)

	sort.Slice(deletes, func(i, j int) bool { return deletes[i] < deletes[j] })
	if !reflect.DeepEqual(deletes, []int64{3, 5}) {
		t.Errorf("deletes = %v, want [3 5]", deletes)
	}

	got := map[string]*int64{}
	for _, u := range upserts {
		got[u.Key+"="+u.Value] = u.Id
	}
	if len(upserts) != 3 {
		t.Fatalf("upserts = %+v, want 3 items", upserts)
	}
	if id := got["CHANGED=new"]; id == nil || *id != 2 {
		t.Errorf("CHANGED must update row 2 in place, got %v", id)
	}
	if id, ok := got["SCOPED=preview"]; !ok || id != nil {
		t.Errorf("SCOPED for Preview must be created, got %v %v", id, ok)
	}
	if id, ok := got["NEW=n"]; !ok || id != nil {
		t.Errorf("NEW must be created, got %v %v", id, ok)
	}
}

func TestMakersFlattenEnvVarsScope(t *testing.T) {
	ctx := context.Background()
	all := []string{"Production", "Preview"}
	remote := []MakersEnvVar{
		{Key: "GLOBAL", Value: "1", Env: []string{"Preview", "Production"}},
		{Key: "EXPLICIT", Value: "2", Env: []string{"Production", "Preview"}},
		{Key: "PROD", Value: "3", Comment: makersString("note"), Env: []string{"Production"}},
	}
	explicitEnvs, _ := types.SetValueFrom(ctx, types.StringType, []string{"Preview", "Production"})
	prior := []makersEnvVarModel{{Key: types.StringValue("EXPLICIT"), Environments: explicitEnvs}}

	set, diags := makersFlattenEnvVars(ctx, remote, all, prior)
	if diags.HasError() {
		t.Fatal(diags)
	}
	var items []makersEnvVarModel
	if diags := set.ElementsAs(ctx, &items, false); diags.HasError() {
		t.Fatal(diags)
	}
	byKey := map[string]makersEnvVarModel{}
	for _, it := range items {
		byKey[it.Key.ValueString()] = it
	}
	if !byKey["GLOBAL"].Environments.IsNull() {
		t.Error("a row scoped to every environment must flatten to null environments")
	}
	if byKey["EXPLICIT"].Environments.IsNull() {
		t.Error("an explicitly listed full scope from prior state must be kept")
	}
	if byKey["PROD"].Environments.IsNull() || byKey["PROD"].Comment.ValueString() != "note" {
		t.Errorf("unexpected PROD item %+v", byKey["PROD"])
	}
	if !byKey["GLOBAL"].Comment.IsNull() {
		t.Error("a missing comment must flatten to null")
	}
}

func TestMakersSourceHash(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.html", "hello")
	write("cloud-functions/api/region.js", "export default 1")
	write("node_modules/x/index.js", "ignored")
	write(".env", "SECRET=1")
	write("sub/.DS_Store", "ignored")

	files, err := makersSourceFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files, []string{"cloud-functions/api/region.js", "index.html"}) {
		t.Errorf("files = %v", files)
	}

	h1, err := makersSourceHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	write("node_modules/x/index.js", "still ignored")
	if h2, _ := makersSourceHash(dir); h2 != h1 {
		t.Error("changes in ignored paths must not change the hash")
	}
	write("index.html", "changed")
	if h3, _ := makersSourceHash(dir); h3 == h1 {
		t.Error("content changes must change the hash")
	}
}

func TestMakersSourceZipHash(t *testing.T) {
	dir := t.TempDir()
	writeZip := func(name, content string) string {
		p := filepath.Join(dir, name)
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		w := zip.NewWriter(f)
		entry, err := w.Create("index.html")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		return p
	}

	h1, err := makersSourceZipHash(writeZip("site.zip", "hello"))
	if err != nil {
		t.Fatal(err)
	}
	if h2, _ := makersSourceZipHash(writeZip("site.zip", "changed")); h2 == h1 {
		t.Error("content changes must change the hash")
	}

	notZip := filepath.Join(dir, "fake.zip")
	if err := os.WriteFile(notZip, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, file := range map[string]string{
		"missing":      filepath.Join(dir, "missing.zip"),
		"directory":    dir,
		"wrong suffix": writeZip("site.tar", "hello"),
		"not a zip":    notZip,
	} {
		if _, err := makersSourceZipHash(file); err == nil {
			t.Errorf("%s: expected an error for %s", name, file)
		}
	}
}

func TestMakersFormatLog(t *testing.T) {
	raw := []byte(`{"t":"i","ls":[[1,"install"],[2,"build"]]}
{"t":"e","ls":[[3,"\u001b[31mboom\u001b[39m"]]}
not json
{"eot":1}
`)
	if got := makersFormatLog(raw, 0); got != "install\nbuild\nERROR boom" {
		t.Errorf("full log = %q", got)
	}
	if got := makersFormatLog(raw, 2); got != "build\nERROR boom" {
		t.Errorf("tail = %q", got)
	}
}
