package rds

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	golangsdk "github.com/opentelekomcloud/gophertelekomcloud"

	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common/cfg"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common/fmterr"
)

func DataSourceRdsFlavorV3() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceRdsFlavorV3Read,

		Schema: map[string]*schema.Schema{
			"db_type": {
				Type:     schema.TypeString,
				Required: true,
			},
			"db_version": {
				Type:     schema.TypeString,
				Required: true,
			},
			"instance_mode": {
				Type:     schema.TypeString,
				Required: true,
			},
			"flavors": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"vcpus": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"memory": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"name": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"mode": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func dataSourceRdsFlavorV3Read(_ context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	config := meta.(*cfg.Config)

	client, err := config.RdsV1Client(config.GetRegion(d))
	if err != nil {
		return fmterr.Errorf("error creating OpenTelekomCloud rds client: %s", err)
	}
	client.Endpoint = strings.Replace(client.Endpoint, "/rds/v1/", "/v3/", 1)

	link := fmt.Sprintf("flavors/%s?version_name=%s", d.Get("db_type").(string), d.Get("db_version").(string))
	url := client.ServiceURL(link)

	r, err := sendRdsFlavorV3ListRequest(client, url)
	if err != nil {
		return diag.FromErr(err)
	}

	mode := d.Get("instance_mode").(string)
	flavors := make([]interface{}, 0, len(r.([]interface{})))
	for _, item := range r.([]interface{}) {
		val := item.(map[string]interface{})

		if mode == val["instance_mode"].(string) {
			flavors = append(flavors, map[string]interface{}{
				"vcpus":  val["vcpus"],
				"memory": val["ram"],
				"name":   val["spec_code"],
				"mode":   val["instance_mode"],
			})
		}
	}

	d.SetId("flavors")
	return diag.FromErr(d.Set("flavors", flavors))
}

func sendRdsFlavorV3ListRequest(client *golangsdk.ServiceClient, url string) (interface{}, error) {
	r := golangsdk.Result{}
	_, r.Err = client.Get(url, &r.Body, &golangsdk.RequestOpts{
		MoreHeaders: map[string]string{
			"Content-Type": "application/json",
			"X-Language":   "en-us",
		}})
	if r.Err != nil {
		return nil, fmt.Errorf("error fetching flavors for rds v3, error: %s", r.Err)
	}

	v, err := common.NavigateValue(r.Body, []string{"flavors"}, nil)
	if err != nil {
		return nil, err
	}
	return v, nil
}
