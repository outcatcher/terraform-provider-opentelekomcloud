package nat

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/opentelekomcloud/gophertelekomcloud"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/snatrules"

	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common/cfg"
)

func ResourceNatSnatRuleV2() *schema.Resource {
	return &schema.Resource{
		Create: resourceNatSnatRuleV2Create,
		Read:   resourceNatSnatRuleV2Read,
		Delete: resourceNatSnatRuleV2Delete,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"region": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},
			"nat_gateway_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"network_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"cidr": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: common.ValidateCIDR,
			},
			"source_type": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},
			"floating_ip_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceNatSnatRuleV2Create(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	_, net_ok := d.GetOk("network_id")
	_, cidr_ok := d.GetOk("cidr")

	if !net_ok && !cidr_ok {
		return fmt.Errorf("Both network_id and cidr are empty, must specify one of them.")
	}
	NatV2Client, err := config.NatV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud nat client: %s", err)
	}

	createOpts := &snatrules.CreateOpts{
		NatGatewayID: d.Get("nat_gateway_id").(string),
		NetworkID:    d.Get("network_id").(string),
		FloatingIPID: d.Get("floating_ip_id").(string),
		SourceType:   d.Get("source_type").(int),
		Cidr:         d.Get("cidr").(string),
	}

	log.Printf("[DEBUG] Create Options: %#v", createOpts)
	snatRule, err := snatrules.Create(NatV2Client, createOpts).Extract()
	if err != nil {
		return fmt.Errorf("Error creatting Snat Rule: %s", err)
	}

	log.Printf("[DEBUG] Waiting for OpenTelekomCloud Snat Rule (%s) to become available.", snatRule.ID)

	stateConf := &resource.StateChangeConf{
		Target:     []string{"ACTIVE"},
		Refresh:    waitForSnatRuleActive(NatV2Client, snatRule.ID),
		Timeout:    d.Timeout(schema.TimeoutCreate),
		Delay:      5 * time.Second,
		MinTimeout: 3 * time.Second,
	}

	_, err = stateConf.WaitForState()
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud Snat Rule: %s", err)
	}

	d.SetId(snatRule.ID)

	return resourceNatSnatRuleV2Read(d, meta)
}

func resourceNatSnatRuleV2Read(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	NatV2Client, err := config.NatV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud nat client: %s", err)
	}

	snatRule, err := snatrules.Get(NatV2Client, d.Id()).Extract()
	if err != nil {
		return common.CheckDeleted(d, err, "Snat Rule")
	}

	d.Set("nat_gateway_id", snatRule.NatGatewayID)
	d.Set("network_id", snatRule.NetworkID)
	d.Set("floating_ip_id", snatRule.FloatingIPID)
	d.Set("source_type", snatRule.SourceType)
	d.Set("cidr", snatRule.Cidr)

	d.Set("region", config.GetRegion(d))

	return nil
}

func resourceNatSnatRuleV2Delete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	NatV2Client, err := config.NatV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud nat client: %s", err)
	}

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"ACTIVE"},
		Target:     []string{"DELETED"},
		Refresh:    waitForSnatRuleDelete(NatV2Client, d.Id()),
		Timeout:    d.Timeout(schema.TimeoutDelete),
		Delay:      5 * time.Second,
		MinTimeout: 3 * time.Second,
	}

	_, err = stateConf.WaitForState()
	if err != nil {
		return fmt.Errorf("Error deleting OpenTelekomCloud Snat Rule: %s", err)
	}

	d.SetId("")
	return nil
}

func waitForSnatRuleActive(NatV2Client *golangsdk.ServiceClient, nId string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		n, err := snatrules.Get(NatV2Client, nId).Extract()
		if err != nil {
			return nil, "", err
		}

		log.Printf("[DEBUG] OpenTelekomCloud Snat Rule: %+v", n)
		if n.Status == "ACTIVE" {
			return n, "ACTIVE", nil
		}

		return n, "", nil
	}
}

func waitForSnatRuleDelete(NatV2Client *golangsdk.ServiceClient, nId string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		log.Printf("[DEBUG] Attempting to delete OpenTelekomCloud Snat Rule %s.\n", nId)

		n, err := snatrules.Get(NatV2Client, nId).Extract()
		if err != nil {
			if _, ok := err.(golangsdk.ErrDefault404); ok {
				log.Printf("[DEBUG] Successfully deleted OpenTelekomCloud Snat Rule %s", nId)
				return n, "DELETED", nil
			}
			return n, "ACTIVE", err
		}

		err = snatrules.Delete(NatV2Client, nId).ExtractErr()
		if err != nil {
			if _, ok := err.(golangsdk.ErrDefault404); ok {
				log.Printf("[DEBUG] Successfully deleted OpenTelekomCloud Snat Rule %s", nId)
				return n, "DELETED", nil
			}
			return n, "ACTIVE", err
		}

		log.Printf("[DEBUG] OpenTelekomCloud Snat Rule %s still active.\n", nId)
		return n, "ACTIVE", nil
	}
}
