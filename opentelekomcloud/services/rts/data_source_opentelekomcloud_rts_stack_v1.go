package rts

import (
	"fmt"
	"log"
	"reflect"
	"unsafe"

	"github.com/hashicorp/errwrap"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/rts/v1/stacks"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/rts/v1/stacktemplates"

	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common/cfg"
)

func DataSourceRTSStackV1() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceRTSStackV1Read,

		Schema: map[string]*schema.Schema{
			"region": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Computed: true,
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"status_reason": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"outputs": {
				Type:     schema.TypeMap,
				Computed: true,
			},
			"parameters": {
				Type:     schema.TypeMap,
				Computed: true,
			},
			"timeout_mins": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"disable_rollback": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"capabilities": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
			"notification_topics": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
			"template_body": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceRTSStackV1Read(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	orchestrationClient, err := config.OrchestrationV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenTelekomCloud rts client: %s", err)
	}
	stackName := d.Get("name").(string)

	stack, err := stacks.Get(orchestrationClient, stackName).Extract()
	if err != nil {
		return fmt.Errorf("Unable to retrieve stack %s: %s", stackName, err)
	}

	log.Printf("[INFO] Retrieved Stack %s", stackName)
	d.SetId(stack.ID)

	d.Set("disable_rollback", stack.DisableRollback)

	d.Set("parameters", stack.Parameters)
	d.Set("status_reason", stack.StatusReason)
	d.Set("name", stack.Name)
	d.Set("outputs", flattenStackOutputs(stack.Outputs))
	d.Set("capabilities", stack.Capabilities)
	d.Set("notification_topics", stack.NotificationTopics)
	d.Set("timeout_mins", stack.Timeout)
	d.Set("status", stack.Status)
	d.Set("region", config.GetRegion(d))

	out, err := stacktemplates.Get(orchestrationClient, stack.Name, stack.ID).Extract()
	if err != nil {
		return err
	}

	sTemplate := BytesToString(out)
	template, error := normalizeStackTemplate(sTemplate)
	if error != nil {
		return errwrap.Wrapf("template body contains an invalid JSON or YAML: {{err}}", err)
	}
	d.Set("template_body", template)

	return nil
}

func BytesToString(b []byte) string {
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh := reflect.StringHeader{Data: bh.Data, Len: bh.Len}
	return *(*string)(unsafe.Pointer(&sh))
}
