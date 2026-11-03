//go:build liveacceptance

// Package acceptance contains opt-in tests against a real Merge account.
// Default go test ./... runs never include this package.
package acceptance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bogware/terraform-provider-mergeah/internal/client"
	"github.com/bogware/terraform-provider-mergeah/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestLiveToolPackAndRegisteredUser(t *testing.T) {
	// A build tag alone is insufficient to enable real account mutations.
	if os.Getenv("MERGEAH_LIVE_TESTS") != "1" || os.Getenv("TF_ACC") != "1" {
		t.Skip("live tests require the liveacceptance tag, MERGEAH_LIVE_TESTS=1, and TF_ACC=1; see docs/guides/live-testing.md")
	}
	key := os.Getenv("MERGEAH_API_KEY")
	if strings.TrimSpace(key) == "" {
		t.Fatal("MERGEAH_API_KEY must contain a dedicated Merge test key")
	}
	base := os.Getenv("MERGEAH_API_URL")
	if base == "" {
		base = client.DefaultURL
	}
	c, err := client.New(base, key, "acceptance")
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	name := "mergeah-acceptance-" + hex.EncodeToString(suffix[:])
	t.Logf("Disposable resource name/origin ID: %s", name)

	// Capture IDs without logging credentials or full Terraform state.
	ids := map[string]string{}
	capture := func(kind string) resource.TestCheckFunc {
		return resource.TestCheckResourceAttrWith("mergeah_"+kind+".test", "id", func(id string) error {
			ids[kind] = id
			return nil
		})
	}
	// Terraform's harness destroys managed resources at the end of the test.
	// Independently verify that the account no longer contains their IDs.
	t.Cleanup(func() {
		for kind, id := range ids {
			collection := "tool-packs/"
			if kind == "registered_user" {
				collection = "registered-users/"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, err := c.Do(ctx, http.MethodGet, collection+url.PathEscape(id)+"/", nil)
			cancel()
			if !client.IsNotFound(err) {
				t.Errorf("cleanup could not confirm deletion of %s %s: %v; inspect disposable resources named %s", kind, id, err, name)
			}
		}
	})
	config := func(label string) string {
		return fmt.Sprintf(`provider "mergeah" {}

resource "mergeah_tool_pack" "test" {
  name        = %q
  description = %q
}

resource "mergeah_registered_user" "test" {
  origin_user_id   = %q
  origin_user_name = %q
  user_type       = "SYSTEM"
}

data "mergeah_tool_pack" "test" {
  resource_id = mergeah_tool_pack.test.id
}

data "mergeah_registered_user" "test" {
  resource_id = mergeah_registered_user.test.id
}
`, name, label, name, label)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"mergeah": providerserver.NewProtocol6WithError(provider.New("acceptance")()),
		},
		Steps: []resource.TestStep{
			{Config: config("Initial acceptance configuration"), Check: resource.ComposeTestCheckFunc(
				capture("tool_pack"), capture("registered_user"),
				resource.TestCheckResourceAttr("mergeah_registered_user.test", "is_test", "true"),
				resource.TestCheckResourceAttrSet("data.mergeah_tool_pack.test", "response_json"),
				resource.TestCheckResourceAttrSet("data.mergeah_registered_user.test", "response_json"),
			)},
			{ResourceName: "mergeah_tool_pack.test", ImportState: true, ImportStateVerify: true},
			{ResourceName: "mergeah_registered_user.test", ImportState: true, ImportStateVerify: true},
			{Config: config("Updated acceptance configuration"), Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("mergeah_tool_pack.test", "description", "Updated acceptance configuration"),
				resource.TestCheckResourceAttr("mergeah_registered_user.test", "origin_user_name", "Updated acceptance configuration"),
			)},
			{Config: config("Updated acceptance configuration"), PlanOnly: true},
		},
	})
}
