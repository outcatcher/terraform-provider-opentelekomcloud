package acceptance

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"

	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/acceptance/common"
)

func TestAccOTCRtsSoftwareConfigV1_importBasic(t *testing.T) {
	resourceName := "opentelekomcloud_rts_software_config_v1.config_1"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { common.TestAccPreCheck(t) },
		Providers:    common.TestAccProviders,
		CheckDestroy: testAccCheckRtsSoftwareConfigV1Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRtsSoftwareConfigV1_basic,
			},

			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
