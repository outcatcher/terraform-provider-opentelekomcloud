package acceptance

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/opentelekomcloud/gophertelekomcloud/acceptance/tools"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/rds/v3/instances"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/acceptance/common"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/acceptance/env"
)

func TestAccRdsReadReplicaV3Basic(t *testing.T) {
	postfix := tools.RandomString("rr", 3)
	var rdsInstance instances.RdsInstanceResponse

	resName := "opentelekomcloud_rds_read_replica_v3.replica"

	secondAZ := "eu-de-02"

	if env.OS_AVAILABILITY_ZONE == secondAZ {
		t.Skip("OS_AVAILABILITY_ZONE should be set to value !=", secondAZ)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { common.TestAccPreCheck(t) },
		ProviderFactories: common.TestAccProviderFactories,
		CheckDestroy:      testAccCheckRdsInstanceV3Destroy,
		Steps: []resource.TestStep{
			{
				Config: testAccRdsReadReplicaV3Basic(postfix),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckRdsInstanceV3Exists(resName, &rdsInstance),
					resource.TestCheckResourceAttr(resName, "availability_zone", secondAZ),
					resource.TestCheckResourceAttr(resName, "volume.0.size", "40"),
				),
			},
		},
	})
}

func testAccRdsReadReplicaV3Basic(postfix string) string {
	return fmt.Sprintf(`
resource "opentelekomcloud_networking_secgroup_v2" "sg" {
  name = "sg-rds-replica-test"
}

resource "opentelekomcloud_rds_instance_v3" "instance" {
  name              = "tf_rds_instance_%s"
  availability_zone = ["%s"]
  db {
    password = "Postgres!120521"
    type     = "PostgreSQL"
    version  = "10"
    port     = "8635"
  }
  security_group_id = opentelekomcloud_networking_secgroup_v2.sg.id
  subnet_id = "%s"
  vpc_id    = "%s"
  volume {
    type = "COMMON"
    size = 40
  }
  flavor = "rds.pg.c2.medium"
  backup_strategy {
    start_time = "08:00-09:00"
    keep_days  = 1
  }
  tag = {
    foo = "bar"
    key = "value"
  }
}

resource "opentelekomcloud_rds_read_replica_v3" "replica" {
  name = "test-replica"
  replica_of_id = opentelekomcloud_rds_instance_v3.instance.id
  flavor_ref = "${opentelekomcloud_rds_instance_v3.instance.flavor}.rr"

  availability_zone = "eu-de-02"

  volume {
    type = "COMMON"
  }
}
`, postfix, env.OS_AVAILABILITY_ZONE, env.OS_NETWORK_ID, env.OS_VPC_ID)
}
