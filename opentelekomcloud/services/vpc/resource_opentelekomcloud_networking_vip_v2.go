package vpc

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/opentelekomcloud/gophertelekomcloud"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/ports"

	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common/cfg"
)

func ResourceNetworkingVIPV2() *schema.Resource {
	return &schema.Resource{
		Create: resourceNetworkingVIPV2Create,
		Read:   resourceNetworkingVIPV2Read,
		Delete: resourceNetworkingVIPV2Delete,

		Schema: map[string]*schema.Schema{
			"network_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"subnet_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"ip_address": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"name": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"tenant_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"device_owner": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceNetworkingVIPV2Create(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	networkingClient, err := config.NetworkingV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud networking client: %s", err)
	}

	// Contruct CreateOpts
	fixip := make([]ports.IP, 1)
	fixip[0] = ports.IP{
		SubnetID:  d.Get("subnet_id").(string),
		IPAddress: d.Get("ip_address").(string),
	}
	createOpts := ports.CreateOpts{
		Name:        d.Get("name").(string),
		NetworkID:   d.Get("network_id").(string),
		FixedIPs:    fixip,
		DeviceOwner: "neutron:VIP_PORT",
	}

	log.Printf("[DEBUG] Create Options: %#v", createOpts)
	vip, err := ports.Create(networkingClient, createOpts).Extract()
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud Neutron network: %s", err)
	}
	log.Printf("[DEBUG] Waiting for OpenTelekomCloud Neutron VIP (%s) to become available.", vip.ID)

	stateConf := &resource.StateChangeConf{
		Target:     []string{"ACTIVE"},
		Refresh:    waitForNetworkVIPActive(networkingClient, vip.ID),
		Timeout:    d.Timeout(schema.TimeoutCreate),
		Delay:      5 * time.Second,
		MinTimeout: 3 * time.Second,
	}

	_, err = stateConf.WaitForState()

	d.SetId(vip.ID)

	return resourceNetworkingVIPV2Read(d, meta)
}

func resourceNetworkingVIPV2Read(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	networkingClient, err := config.NetworkingV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud networking client: %s", err)
	}

	vip, err := ports.Get(networkingClient, d.Id()).Extract()
	if err != nil {
		return common.CheckDeleted(d, err, "vip")
	}

	log.Printf("[DEBUG] Retrieved VIP %s: %+v", d.Id(), vip)

	// Computed values
	d.Set("network_id", vip.NetworkID)
	if len(vip.FixedIPs) > 0 {
		d.Set("subnet_id", vip.FixedIPs[0].SubnetID)
		d.Set("ip_address", vip.FixedIPs[0].IPAddress)
	} else {
		d.Set("subnet_id", "")
		d.Set("ip_address", "")
	}
	d.Set("name", vip.Name)
	d.Set("status", vip.Status)
	d.Set("id", vip.ID)
	d.Set("tenant_id", vip.TenantID)
	d.Set("device_owner", vip.DeviceOwner)

	return nil
}

func resourceNetworkingVIPV2Delete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	networkingClient, err := config.NetworkingV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud networking client: %s", err)
	}

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"ACTIVE"},
		Target:     []string{"DELETED"},
		Refresh:    waitForNetworkVIPDelete(networkingClient, d.Id()),
		Timeout:    d.Timeout(schema.TimeoutDelete),
		Delay:      5 * time.Second,
		MinTimeout: 3 * time.Second,
	}

	_, err = stateConf.WaitForState()
	if err != nil {
		return fmt.Errorf("Error deleting OpenTelekomCloud Neutron Network: %s", err)
	}

	d.SetId("")
	return nil
}

func waitForNetworkVIPActive(networkingClient *golangsdk.ServiceClient, vipid string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		p, err := ports.Get(networkingClient, vipid).Extract()
		if err != nil {
			return nil, "", err
		}

		log.Printf("[DEBUG] OpenTelekomCloud Neutron Port: %+v", p)
		if p.Status == "DOWN" || p.Status == "ACTIVE" {
			return p, "ACTIVE", nil
		}

		return p, p.Status, nil
	}
}

func waitForNetworkVIPDelete(networkingClient *golangsdk.ServiceClient, vipid string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		log.Printf("[DEBUG] Attempting to delete OpenTelekomCloud Neutron VIP %s", vipid)

		p, err := ports.Get(networkingClient, vipid).Extract()
		if err != nil {
			if _, ok := err.(golangsdk.ErrDefault404); ok {
				log.Printf("[DEBUG] Successfully deleted OpenTelekomCloud VIP %s", vipid)
				return p, "DELETED", nil
			}
			return p, "ACTIVE", err
		}

		err = ports.Delete(networkingClient, vipid).ExtractErr()
		if err != nil {
			if _, ok := err.(golangsdk.ErrDefault404); ok {
				log.Printf("[DEBUG] Successfully deleted OpenTelekomCloud VIP %s", vipid)
				return p, "DELETED", nil
			}
			return p, "ACTIVE", err
		}

		log.Printf("[DEBUG] OpenTelekomCloud VIP %s still active.\n", vipid)
		return p, "ACTIVE", nil
	}
}
