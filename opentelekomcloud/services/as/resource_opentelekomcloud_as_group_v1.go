package as

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/opentelekomcloud/gophertelekomcloud"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/common/tags"

	"github.com/opentelekomcloud/gophertelekomcloud/openstack/autoscaling/v1/groups"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/autoscaling/v1/instances"

	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common/cfg"
)

func ResourceASGroup() *schema.Resource {
	return &schema.Resource{
		Create: resourceASGroupCreate,
		Read:   resourceASGroupRead,
		Update: resourceASGroupUpdate,
		Delete: resourceASGroupDelete,

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
			"scaling_group_name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: common.ValidateName,
			},
			"scaling_configuration_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"desire_instance_number": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"min_instance_number": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  0,
			},
			"max_instance_number": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  0,
			},
			"cool_down_time": {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      900,
				ValidateFunc: validation.IntBetween(0, 86400),
				Description:  "The cooling duration, in seconds.",
			},
			"lb_listener_id": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: common.ValidateASGroupListenerID,
				Description:  "The system supports the binding of up to three ELB listeners, the IDs of which are separated using a comma.",
			},
			"lbaas_listeners": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"pool_id": {
							Type:     schema.TypeString,
							Required: true,
						},
						"protocol_port": {
							Type:     schema.TypeInt,
							Required: true,
						},
						"weight": {
							Type:     schema.TypeInt,
							Optional: true,
							Default:  1,
						},
					},
				},
			},
			"available_zones": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"networks": {
				Type:     schema.TypeList,
				MaxItems: 5,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"security_groups": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"vpc_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"health_periodic_audit_method": {
				Type:     schema.TypeString,
				Optional: true,
				ValidateFunc: validation.StringInSlice([]string{
					"ELB_AUDIT", "NOVA_AUDIT",
				}, true),
				Default: "NOVA_AUDIT",
			},
			"health_periodic_audit_time": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  5,
				ValidateFunc: validation.IntInSlice([]int{
					0, 1, 5, 15, 60, 180,
				}),
				Description: "The health check period for instances, in minutes.",
			},
			"health_periodic_audit_grace_period": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     600,
				Description: "The grace period for instance health check, in seconds.",
			},
			"instance_terminate_policy": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "OLD_CONFIG_OLD_INSTANCE",
				ValidateFunc: validation.StringInSlice([]string{
					"OLD_CONFIG_OLD_INSTANCE ", "OLD_CONFIG_NEW_INSTANCE", "OLD_INSTANCE", "NEW_INSTANCE",
				}, true),
			},
			"notifications": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"delete_publicip": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"delete_instances": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "no",
				Description: "Whether to delete instances when they are removed from the AS group.",
			},
			"instances": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The instances id list in the as group.",
			},
			"current_instance_number": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"tags": common.TagsSchema(),
		},
	}
}

func getAllNotifications(d *schema.ResourceData) []string {
	rawNotifications := d.Get("notifications").([]interface{})
	notifications := make([]string, len(rawNotifications))
	for i, raw := range rawNotifications {
		notifications[i] = raw.(string)
	}
	log.Printf("[DEBUG] getNotifications: %#v", notifications)

	return notifications
}

func getAllNetworks(d *schema.ResourceData) []groups.NetworkOpts {
	var networkOptsList []groups.NetworkOpts
	networks := d.Get("networks").([]interface{})
	for _, v := range networks {
		network := v.(map[string]interface{})
		networkID := network["id"].(string)
		val := groups.NetworkOpts{
			ID: networkID,
		}
		networkOptsList = append(networkOptsList, val)
	}

	log.Printf("[DEBUG] Got Networks Opts: %#v", networkOptsList)
	return networkOptsList
}

func getAllSecurityGroups(d *schema.ResourceData) []groups.SecurityGroupOpts {
	var Groups []groups.SecurityGroupOpts

	asGroups := d.Get("security_groups").([]interface{})
	for _, v := range asGroups {
		group := v.(map[string]interface{})
		groupID := group["id"].(string)
		v := groups.SecurityGroupOpts{
			ID: groupID,
		}
		Groups = append(Groups, v)
	}

	log.Printf("[DEBUG] Got Security Groups Opts: %#v", Groups)
	return Groups
}

func getAllLBaaSListeners(d *schema.ResourceData) []groups.LBaaSListenerOpts {
	var asListeners []groups.LBaaSListenerOpts

	listeners := d.Get("lbaas_listeners").([]interface{})
	for _, v := range listeners {
		listener := v.(map[string]interface{})
		s := groups.LBaaSListenerOpts{
			PoolID:       listener["pool_id"].(string),
			ProtocolPort: listener["protocol_port"].(int),
			Weight:       listener["weight"].(int),
		}
		asListeners = append(asListeners, s)
	}

	log.Printf("[DEBUG] getAllLBaaSListeners: %#v", asListeners)
	return asListeners
}

func getInstancesInGroup(client *golangsdk.ServiceClient, groupID string, opts instances.ListOptsBuilder) ([]instances.Instance, error) {
	var instanceList []instances.Instance
	page, err := instances.List(client, groupID, opts).AllPages()
	if err != nil {
		return instanceList, fmt.Errorf("error getting instances in ASGroup %q: %s", groupID, err)
	}
	instanceList, err = page.(instances.InstancePage).Extract()
	return instanceList, err
}

func getInstancesIDs(instanceList []instances.Instance) []string {
	var instanceIDs []string
	for _, instance := range instanceList {
		// Maybe the instance is pending, so we can't get the id,
		// so unable to delete the instance this time, maybe next time to execute
		// terraform destroy will works
		if instance.ID != "" {
			instanceIDs = append(instanceIDs, instance.ID)
		}
	}
	log.Printf("[DEBUG] Get instances in ASGroups: %#v", instanceIDs)
	return instanceIDs
}

func getInstancesLifeStates(allIns []instances.Instance) []string {
	var allLifeStates []string
	for _, ins := range allIns {
		allLifeStates = append(allLifeStates, ins.LifeCycleStatus)
	}
	log.Printf("[DEBUG] Get instances lifecycle status in ASGroups: %#v", allLifeStates)
	return allLifeStates
}

func refreshInstancesLifeStates(client *golangsdk.ServiceClient, groupID string, instanceNumber int, checkInService bool) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		var opts instances.ListOptsBuilder
		instanceList, err := getInstancesInGroup(client, groupID, opts)
		if err != nil {
			return nil, "ERROR", err
		}
		// maybe the instances (or some of the instances) have not put in the asg when creating
		if checkInService && len(instanceList) != instanceNumber {
			return instanceList, "PENDING", err
		}
		allLifeStatus := getInstancesLifeStates(instanceList)
		for _, lifeStatus := range allLifeStatus {
			log.Printf("[DEBUG] Get lifecycle status in group %s: %s", groupID, lifeStatus)
			// check for creation
			if checkInService {
				if lifeStatus == "PENDING" || lifeStatus == "REMOVING" {
					return instanceList, lifeStatus, err
				}
			}
			// check for removal
			if !checkInService {
				if lifeStatus != "INSERVICE" {
					return instanceList, lifeStatus, err
				}
			}
		}
		if checkInService {
			return instanceList, "INSERVICE", err
		}
		log.Printf("[DEBUG] Exit refreshInstancesLifeStates for %q!", groupID)
		return instanceList, "", err
	}
}

func resourceASGroupCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.AutoscalingV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud AutoScaling client: %s", err)
	}

	minNum := d.Get("min_instance_number").(int)
	maxNum := d.Get("max_instance_number").(int)
	desireNum := d.Get("desire_instance_number").(int)
	log.Printf("[DEBUG] Min instance number is: %#v", minNum)
	log.Printf("[DEBUG] Max instance number is: %#v", maxNum)
	log.Printf("[DEBUG] Desire instance number is: %#v", desireNum)
	if desireNum < minNum || desireNum > maxNum {
		return fmt.Errorf("invalid parameters: it should be `min_instance_number`<=`desire_instance_number`<=`max_instance_number`")
	}
	var initNum int
	if desireNum > 0 {
		initNum = desireNum
	} else {
		initNum = minNum
	}
	log.Printf("[DEBUG] Init instance number is: %#v", initNum)

	networks := getAllNetworks(d)
	secGroups := getAllSecurityGroups(d)
	asgLBaaSListeners := getAllLBaaSListeners(d)
	isDeletePublicIp := d.Get("delete_publicip").(bool)

	log.Printf("[DEBUG] available_zones: %#v", d.Get("available_zones"))
	createOpts := groups.CreateOpts{
		Name:                      d.Get("scaling_group_name").(string),
		ConfigurationID:           d.Get("scaling_configuration_id").(string),
		DesireInstanceNumber:      desireNum,
		MinInstanceNumber:         minNum,
		MaxInstanceNumber:         maxNum,
		CoolDownTime:              d.Get("cool_down_time").(int),
		LBListenerID:              d.Get("lb_listener_id").(string),
		LBaaSListeners:            asgLBaaSListeners,
		AvailableZones:            common.GetAllAvailableZones(d),
		Networks:                  networks,
		SecurityGroup:             secGroups,
		VpcID:                     d.Get("vpc_id").(string),
		HealthPeriodicAuditMethod: d.Get("health_periodic_audit_method").(string),
		HealthPeriodicAuditTime:   d.Get("health_periodic_audit_time").(int),
		HealthPeriodicAuditGrace:  d.Get("health_periodic_audit_grace_period").(int),
		InstanceTerminatePolicy:   d.Get("instance_terminate_policy").(string),
		Notifications:             getAllNotifications(d),
		IsDeletePublicip:          &isDeletePublicIp,
	}

	log.Printf("[DEBUG] Create Options: %#v", createOpts)
	asGroupID, err := groups.Create(client, createOpts).Extract()
	if err != nil {
		return fmt.Errorf("error creating ASGroup: %s", err)
	}

	time.Sleep(5 * time.Second)

	// enable AutoScaling Group
	enableResult := groups.Enable(client, asGroupID)
	if enableResult.Err != nil {
		return fmt.Errorf("error enabling ASGroup %q: %s", asGroupID, enableResult.Err)
	}
	log.Printf("[DEBUG] Enable ASGroup %q success!", asGroupID)
	// check all instances are inService
	if initNum > 0 {
		stateConf := &resource.StateChangeConf{
			Pending: []string{"PENDING"},
			Target:  []string{"INSERVICE"}, // if there is no lifecycle status, meaning no instances in asg
			Refresh: refreshInstancesLifeStates(client, asGroupID, initNum, true),
			Timeout: d.Timeout(schema.TimeoutCreate),
			Delay:   10 * time.Second,
		}

		_, err := stateConf.WaitForState()
		if err != nil {
			return fmt.Errorf("error waiting for instances in the ASGroup %q to become inservice: %s", asGroupID, err)
		}
	}

	// set tags
	tagRaw := d.Get("tags").(map[string]interface{})
	if len(tagRaw) > 0 {
		tagList := common.ExpandResourceTags(tagRaw)
		if err := tags.Create(client, "scaling_group_tag", asGroupID, tagList).ExtractErr(); err != nil {
			return fmt.Errorf("error setting tags of AutoScaling Group: %s", err)
		}
	}

	d.SetId(asGroupID)

	return resourceASGroupRead(d, meta)
}

func resourceASGroupRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.AutoscalingV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud autoscaling client: %s", err)
	}

	asGroup, err := groups.Get(client, d.Id()).Extract()
	if err != nil {
		if _, ok := err.(golangsdk.ErrDefault404); ok {
			d.SetId("")
			return nil
		}

		return fmt.Errorf("error retrieving OpenTelekomCloud AutoScaling Group: %s", err)
	}

	// set properties based on the read info
	mErr := multierror.Append(nil,
		d.Set("scaling_group_name", asGroup.Name),
		d.Set("status", asGroup.Status),
		d.Set("current_instance_number", asGroup.ActualInstanceNumber),
		d.Set("desire_instance_number", asGroup.DesireInstanceNumber),
		d.Set("min_instance_number", asGroup.MinInstanceNumber),
		d.Set("max_instance_number", asGroup.MaxInstanceNumber),
		d.Set("cool_down_time", asGroup.CoolDownTime),
		d.Set("lb_listener_id", asGroup.LBListenerID),
		d.Set("health_periodic_audit_method", asGroup.HealthPeriodicAuditMethod),
		d.Set("health_periodic_audit_time", asGroup.HealthPeriodicAuditTime),
		d.Set("health_periodic_audit_grace_period", asGroup.HealthPeriodicAuditGrace),
		d.Set("instance_terminate_policy", asGroup.InstanceTerminatePolicy),
		d.Set("scaling_configuration_id", asGroup.ConfigurationID),
		d.Set("delete_publicip", asGroup.DeletePublicIP),
		d.Set("region", config.GetRegion(d)),
	)
	if len(asGroup.Notifications) >= 1 {
		if err := d.Set("notifications", asGroup.Notifications); err != nil {
			return err
		}
	}
	if len(asGroup.LBaaSListeners) >= 1 {
		listeners := make([]map[string]interface{}, len(asGroup.LBaaSListeners))
		for i, listener := range asGroup.LBaaSListeners {
			listeners[i] = make(map[string]interface{})
			listeners[i]["pool_id"] = listener.PoolID
			listeners[i]["protocol_port"] = listener.ProtocolPort
			listeners[i]["weight"] = listener.Weight
		}
		if err := d.Set("lbaas_listeners", listeners); err != nil {
			return err
		}
	}

	var opts instances.ListOptsBuilder
	instancesList, err := getInstancesInGroup(client, d.Id(), opts)
	if err != nil {
		return fmt.Errorf("can not get the instances in ASGroup %q: %s", d.Id(), err)
	}
	instanceIDs := getInstancesIDs(instancesList)
	if err := d.Set("instances", instanceIDs); err != nil {
		return err
	}

	if mErr.ErrorOrNil() != nil {
		return mErr
	}

	// save tags
	resourceTags, err := tags.Get(client, "scaling_group_tag", d.Id()).Extract()
	if err != nil {
		return fmt.Errorf("error fetching OpenTelekomCloud AutoScaling Group tags: %s", err)
	}
	tagMap := common.TagsToMap(resourceTags)
	if err := d.Set("tags", tagMap); err != nil {
		return fmt.Errorf("error saving tags for OpenTelekomCloud AutoScaling Group: %s", err)
	}

	return nil
}

func resourceASGroupUpdate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.AutoscalingV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud autoscaling client: %s", err)
	}
	d.Partial(true)

	if d.HasChange("min_instance_number") || d.HasChange("max_instance_number") || d.HasChange("desire_instance_number") || d.HasChange("lbaas_listeners") {
		minNum := d.Get("min_instance_number").(int)
		maxNum := d.Get("max_instance_number").(int)
		desireNum := d.Get("desire_instance_number").(int)
		log.Printf("[DEBUG] Min instance number is: %#v", minNum)
		log.Printf("[DEBUG] Max instance number is: %#v", maxNum)
		log.Printf("[DEBUG] Desire instance number is: %#v", desireNum)
		if desireNum < minNum || desireNum > maxNum {
			return fmt.Errorf("invalid parameters: it should be min_instance_number<=desire_instance_number<=max_instance_number")
		}

	}

	networks := getAllNetworks(d)
	secGroups := getAllSecurityGroups(d)
	isDeletePublicIp := d.Get("delete_publicip").(bool)

	asgLBaaSListeners := getAllLBaaSListeners(d)
	updateOpts := groups.UpdateOpts{
		Name:                      d.Get("scaling_group_name").(string),
		ConfigurationID:           d.Get("scaling_configuration_id").(string),
		DesireInstanceNumber:      d.Get("desire_instance_number").(int),
		MinInstanceNumber:         d.Get("min_instance_number").(int),
		MaxInstanceNumber:         d.Get("max_instance_number").(int),
		CoolDownTime:              d.Get("cool_down_time").(int),
		LBListenerID:              d.Get("lb_listener_id").(string),
		LBaaSListeners:            asgLBaaSListeners,
		AvailableZones:            common.GetAllAvailableZones(d),
		Networks:                  networks,
		SecurityGroup:             secGroups,
		HealthPeriodicAuditMethod: d.Get("health_periodic_audit_method").(string),
		HealthPeriodicAuditTime:   d.Get("health_periodic_audit_time").(int),
		HealthPeriodicAuditGrace:  d.Get("health_periodic_audit_grace_period").(int),
		InstanceTerminatePolicy:   d.Get("instance_terminate_policy").(string),
		Notifications:             getAllNotifications(d),
		IsDeletePublicip:          &isDeletePublicIp,
	}
	asGroupID, err := groups.Update(client, d.Id(), updateOpts).Extract()
	if err != nil {
		return fmt.Errorf("error updating ASGroup %q: %s", asGroupID, err)
	}

	// update tags
	if d.HasChange("tags") {
		if err := common.UpdateResourceTags(client, d, "scaling_group_tag", d.Id()); err != nil {
			return fmt.Errorf("error updating tags of AutoScaling Group %s: %s", d.Id(), err)
		}
	}

	d.Partial(false)
	return resourceASGroupRead(d, meta)
}

func resourceASGroupDelete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.AutoscalingV1Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud AutoScaling client: %s", err)
	}

	log.Printf("[DEBUG] Begin to get instances of AutoScaling Group %q", d.Id())
	var listOpts instances.ListOptsBuilder
	instanceList, err := getInstancesInGroup(client, d.Id(), listOpts)
	if err != nil {
		return fmt.Errorf("error listing instances of AutoScaling Group: %s", err)
	}
	allLifeStatus := getInstancesLifeStates(instanceList)
	for _, lifeCycleState := range allLifeStatus {
		if lifeCycleState != "INSERVICE" {
			return fmt.Errorf("[DEBUG] Can't delete the ASGroup %q: There are some instances not in INSERVICE but in %s, try again latter", d.Id(), lifeCycleState)
		}
	}
	instanceIDs := getInstancesIDs(instanceList)
	log.Printf("[DEBUG] InstanceIDs in ASGroup %q: %+v", d.Id(), instanceIDs)
	log.Printf("[DEBUG] There are %d instances in ASGroup %q", len(instanceIDs), d.Id())
	if len(allLifeStatus) > 0 {
		minNumber := d.Get("min_instance_number").(int)
		// If you need to delete as_group with `min_instance_number` > 0
		// firstly we need to update `min_instance_number` = 0
		if minNumber > 0 {
			updateOpts := groups.UpdateOpts{
				MinInstanceNumber: 0,
			}
			_, err := groups.Update(client, d.Id(), updateOpts).Extract()
			if err != nil {
				return fmt.Errorf("error updating min_instance_number to 0: %s", err)
			}
		}
		deleteInstances := d.Get("delete_instances").(string)
		log.Printf("[DEBUG] The flag delete_instances in AutoScaling Group is %s", deleteInstances)

		if err := instances.BatchDelete(client, d.Id(), instanceIDs, deleteInstances).ExtractErr(); err != nil {
			return fmt.Errorf("error removing instancess of AutoScaling Group: %s", err)
		}
		log.Printf("[DEBUG] Begin to remove instances of ASGroup %q", d.Id())

		stateConf := &resource.StateChangeConf{
			Pending: []string{"REMOVING"},
			Target:  []string{""}, // if there is no lifecycle status, meaning no instances in asg
			Refresh: refreshInstancesLifeStates(client, d.Id(), 0, false),
			Timeout: d.Timeout(schema.TimeoutDelete),
			Delay:   10 * time.Second,
		}

		_, err := stateConf.WaitForState()

		if err != nil {
			return fmt.Errorf("[DEBUG] Error removing instances from ASGroup %q: %s", d.Id(), err)
		}
	}

	log.Printf("[DEBUG] Begin to delete ASGroup %q", d.Id())
	if err := groups.Delete(client, d.Id()).ExtractErr(); err != nil {
		return fmt.Errorf("error deleting AutoScaling Group: %s", err)
	}

	return nil
}
