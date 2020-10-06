package opentelekomcloud

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"

	"regexp"

	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/lbaas_v2/loadbalancers"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/security/groups"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/ports"
)

func TestAccLBV2LoadBalancer_basic(t *testing.T) {
	var lb loadbalancers.LoadBalancer

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckLBV2LoadBalancerDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccLBV2LoadBalancerConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLBV2LoadBalancerExists("opentelekomcloud_lb_loadbalancer_v2.loadbalancer_1", &lb),
				),
			},
			{
				Config: testAccLBV2LoadBalancerConfig_update,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"opentelekomcloud_lb_loadbalancer_v2.loadbalancer_1", "name", "loadbalancer_1_updated"),
					resource.TestMatchResourceAttr(
						"opentelekomcloud_lb_loadbalancer_v2.loadbalancer_1", "vip_port_id",
						regexp.MustCompile("^[a-f0-9-]+")),
				),
			},
		},
	})
}

func TestAccLBV2LoadBalancer_secGroup(t *testing.T) {
	var lb loadbalancers.LoadBalancer
	var sg_1, sg_2 groups.SecGroup

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckLBV2LoadBalancerDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccLBV2LoadBalancer_secGroup,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLBV2LoadBalancerExists(
						"opentelekomcloud_lb_loadbalancer_v2.loadbalancer_1", &lb),
					testAccCheckNetworkingV2SecGroupExists(
						"opentelekomcloud_networking_secgroup_v2.secgroup_1", &sg_1),
					testAccCheckNetworkingV2SecGroupExists(
						"opentelekomcloud_networking_secgroup_v2.secgroup_1", &sg_2),
					resource.TestCheckResourceAttr(
						"opentelekomcloud_lb_loadbalancer_v2.loadbalancer_1", "security_group_ids.#", "1"),
					testAccCheckLBV2LoadBalancerHasSecGroup(&lb, &sg_1),
				),
			},
			{
				Config: testAccLBV2LoadBalancer_secGroup_update1,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLBV2LoadBalancerExists(
						"opentelekomcloud_lb_loadbalancer_v2.loadbalancer_1", &lb),
					testAccCheckNetworkingV2SecGroupExists(
						"opentelekomcloud_networking_secgroup_v2.secgroup_2", &sg_1),
					testAccCheckNetworkingV2SecGroupExists(
						"opentelekomcloud_networking_secgroup_v2.secgroup_2", &sg_2),
					resource.TestCheckResourceAttr(
						"opentelekomcloud_lb_loadbalancer_v2.loadbalancer_1", "security_group_ids.#", "2"),
					testAccCheckLBV2LoadBalancerHasSecGroup(&lb, &sg_1),
					testAccCheckLBV2LoadBalancerHasSecGroup(&lb, &sg_2),
				),
			},
			{
				Config: testAccLBV2LoadBalancer_secGroup_update2,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckLBV2LoadBalancerExists(
						"opentelekomcloud_lb_loadbalancer_v2.loadbalancer_1", &lb),
					testAccCheckNetworkingV2SecGroupExists(
						"opentelekomcloud_networking_secgroup_v2.secgroup_2", &sg_1),
					testAccCheckNetworkingV2SecGroupExists(
						"opentelekomcloud_networking_secgroup_v2.secgroup_2", &sg_2),
					resource.TestCheckResourceAttr(
						"opentelekomcloud_lb_loadbalancer_v2.loadbalancer_1", "security_group_ids.#", "1"),
					testAccCheckLBV2LoadBalancerHasSecGroup(&lb, &sg_2),
				),
			},
		},
	})
}

func testAccCheckLBV2LoadBalancerDestroy(s *terraform.State) error {
	config := testAccProvider.Meta().(*Config)
	networkingClient, err := config.networkingV2Client(OS_REGION_NAME)
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud networking client: %s", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "opentelekomcloud_lb_loadbalancer_v2" {
			continue
		}

		_, err := loadbalancers.Get(networkingClient, rs.Primary.ID).Extract()
		if err == nil {
			return fmt.Errorf("LoadBalancer still exists: %s", rs.Primary.ID)
		}
	}

	return nil
}

func testAccCheckLBV2LoadBalancerExists(
	n string, lb *loadbalancers.LoadBalancer) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No ID is set")
		}

		config := testAccProvider.Meta().(*Config)
		networkingClient, err := config.networkingV2Client(OS_REGION_NAME)
		if err != nil {
			return fmt.Errorf("Error creating OpenTelekomCloud networking client: %s", err)
		}

		found, err := loadbalancers.Get(networkingClient, rs.Primary.ID).Extract()
		if err != nil {
			return err
		}

		if found.ID != rs.Primary.ID {
			return fmt.Errorf("Member not found")
		}

		*lb = *found

		return nil
	}
}

func testAccCheckLBV2LoadBalancerHasSecGroup(
	lb *loadbalancers.LoadBalancer, sg *groups.SecGroup) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		config := testAccProvider.Meta().(*Config)
		networkingClient, err := config.networkingV2Client(OS_REGION_NAME)
		if err != nil {
			return fmt.Errorf("Error creating OpenTelekomCloud networking client: %s", err)
		}

		port, err := ports.Get(networkingClient, lb.VipPortID).Extract()
		if err != nil {
			return err
		}

		for _, p := range port.SecurityGroups {
			if p == sg.ID {
				return nil
			}
		}

		return fmt.Errorf("LoadBalancer does not have the security group")
	}
}

var testAccLBV2LoadBalancerConfig_basic = fmt.Sprintf(`
resource "opentelekomcloud_lb_loadbalancer_v2" "loadbalancer_1" {
  name = "loadbalancer_1"
  vip_subnet_id = "%s"

  timeouts {
    create = "5m"
    update = "5m"
    delete = "5m"
  }
}
`, OS_SUBNET_ID)

var testAccLBV2LoadBalancerConfig_update = fmt.Sprintf(`
resource "opentelekomcloud_lb_loadbalancer_v2" "loadbalancer_1" {
  name = "loadbalancer_1_updated"
  admin_state_up = "true"
  vip_subnet_id = "%s"

  timeouts {
    create = "5m"
    update = "5m"
    delete = "5m"
  }
}
`, OS_SUBNET_ID)

var testAccLBV2LoadBalancer_secGroup = fmt.Sprintf(`
resource "opentelekomcloud_networking_secgroup_v2" "secgroup_1" {
  name = "secgroup_1"
  description = "secgroup_1"
}

resource "opentelekomcloud_networking_secgroup_v2" "secgroup_2" {
  name = "secgroup_2"
  description = "secgroup_2"
}

resource "opentelekomcloud_lb_loadbalancer_v2" "loadbalancer_1" {
    name = "loadbalancer_1"
    vip_subnet_id = "%s"
    security_group_ids = [
      "${opentelekomcloud_networking_secgroup_v2.secgroup_1.id}"
    ]
}
`, OS_SUBNET_ID)

var testAccLBV2LoadBalancer_secGroup_update1 = fmt.Sprintf(`
resource "opentelekomcloud_networking_secgroup_v2" "secgroup_1" {
  name = "secgroup_1"
  description = "secgroup_1"
}

resource "opentelekomcloud_networking_secgroup_v2" "secgroup_2" {
  name = "secgroup_2"
  description = "secgroup_2"
}

resource "opentelekomcloud_lb_loadbalancer_v2" "loadbalancer_1" {
    name = "loadbalancer_1"
    vip_subnet_id = "%s"
    security_group_ids = [
      "${opentelekomcloud_networking_secgroup_v2.secgroup_1.id}",
      "${opentelekomcloud_networking_secgroup_v2.secgroup_2.id}"
    ]
}
`, OS_SUBNET_ID)

var testAccLBV2LoadBalancer_secGroup_update2 = fmt.Sprintf(`
resource "opentelekomcloud_networking_secgroup_v2" "secgroup_1" {
  name = "secgroup_1"
  description = "secgroup_1"
}

resource "opentelekomcloud_networking_secgroup_v2" "secgroup_2" {
  name = "secgroup_2"
  description = "secgroup_2"
}

resource "opentelekomcloud_lb_loadbalancer_v2" "loadbalancer_1" {
    name = "loadbalancer_1"
    vip_subnet_id = "%s"
    security_group_ids = [
      "${opentelekomcloud_networking_secgroup_v2.secgroup_2.id}"
    ]
    depends_on = ["opentelekomcloud_networking_secgroup_v2.secgroup_1"]
}
`, OS_SUBNET_ID)
