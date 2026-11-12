package teo_test

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"

	tcacctest "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/acctest"
	tcfwacctest "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/framework/acctest"
)

// The account must own exactly one Makers zone.
func TestAccTencentCloudTeoMakersProjectResource_basic(t *testing.T) {
	siteDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(siteDir, "index.html"), []byte("<h1>tf acc</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	siteZip := filepath.Join(t.TempDir(), "site.zip")
	testAccTeoMakersWriteZip(t, siteZip, "index.html", "<h1>tf acc zip</h1>")
	dirSource := fmt.Sprintf("source_dir = %q", siteDir)
	zipSource := fmt.Sprintf("source_zip = %q", siteZip)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { tcacctest.AccPreCheck(t) },
		ProtoV6ProviderFactories: tcfwacctest.AccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTeoMakersProject(dirSource, "tf-acc-makers-a", "v1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("tencentcloud_teo_makers_project.example", "project_id"),
					resource.TestCheckResourceAttrSet("tencentcloud_teo_makers_project.example", "preset_domain"),
					resource.TestCheckResourceAttr("tencentcloud_teo_makers_project.example", "area", "overseas"),
					resource.TestCheckResourceAttr("tencentcloud_teo_makers_project_env_vars.example", "env_vars.#", "2"),
					resource.TestCheckResourceAttr("tencentcloud_teo_makers_deployment.example", "status", "Success"),
					resource.TestCheckResourceAttrSet("tencentcloud_teo_makers_deployment.example", "source_hash"),
				),
			},
			{
				Config: testAccTeoMakersProject(dirSource, "tf-acc-makers-b", "v2"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("tencentcloud_teo_makers_project.example", "name", "tf-acc-makers-b"),
					resource.TestCheckResourceAttr("tencentcloud_teo_makers_deployment.example", "status", "Success"),
				),
			},
			{
				Config: testAccTeoMakersProject(zipSource, "tf-acc-makers-b", "v2"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("tencentcloud_teo_makers_deployment.example", "source_zip", siteZip),
					resource.TestCheckResourceAttr("tencentcloud_teo_makers_deployment.example", "status", "Success"),
				),
			},
			{
				ResourceName:      "tencentcloud_teo_makers_project.example",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "tencentcloud_teo_makers_project_env_vars.example",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:            "tencentcloud_teo_makers_deployment.example",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"source_dir", "source_zip", "source_hash", "timeouts"},
			},
		},
	})
}

func testAccTeoMakersWriteZip(t *testing.T, file, name, content string) {
	t.Helper()
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	entry, err := w.Create(name)
	if err == nil {
		_, err = entry.Write([]byte(content))
	}
	if err == nil {
		err = w.Close()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
}

func testAccTeoMakersProject(source, name, apiVersion string) string {
	return fmt.Sprintf(`
data "tencentcloud_teo_makers_zones" "this" {}

resource "tencentcloud_teo_makers_project" "example" {
  zone_id = data.tencentcloud_teo_makers_zones.this.zone_ids[0]
  name    = %[2]q
  area    = "overseas"
}

resource "tencentcloud_teo_makers_project_env_vars" "example" {
  zone_id    = tencentcloud_teo_makers_project.example.zone_id
  project_id = tencentcloud_teo_makers_project.example.project_id

  env_vars = [
    { key = "API_VERSION", value = %[3]q },
    { key = "ONLY_PREVIEW", value = "1", environments = ["Preview"], comment = "acc" },
  ]
}

resource "tencentcloud_teo_makers_deployment" "example" {
  zone_id    = tencentcloud_teo_makers_project.example.zone_id
  project_id = tencentcloud_teo_makers_project.example.project_id
  %[1]s

  lifecycle {
    replace_triggered_by = [tencentcloud_teo_makers_project_env_vars.example]
  }
}
`, source, name, apiVersion)
}
