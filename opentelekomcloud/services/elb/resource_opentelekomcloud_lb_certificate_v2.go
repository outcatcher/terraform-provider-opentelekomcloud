package elb

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/opentelekomcloud/gophertelekomcloud"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/lbaas_v2/certificates"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v2/extensions/lbaas_v2/listeners"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common"
	"github.com/opentelekomcloud/terraform-provider-opentelekomcloud/opentelekomcloud/common/cfg"
)

func ResourceCertificateV2() *schema.Resource {
	return &schema.Resource{
		Create: resourceCertificateV2Create,
		Read:   resourceCertificateV2Read,
		Update: resourceCertificateV2Update,
		Delete: resourceCertificateV2Delete,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Update: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(5 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"region": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"name": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"domain": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"private_key": {
				Type:             schema.TypeString,
				Optional:         true,
				DiffSuppressFunc: common.SuppressStrippedNewLines,
			},

			"certificate": {
				Type:             schema.TypeString,
				Required:         true,
				DiffSuppressFunc: common.SuppressStrippedNewLines,
			},

			"type": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice([]string{"server", "client"}, false),
			},

			"update_time": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"create_time": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceCertificateV2Create(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	networkingClient, err := config.NetworkingV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud networking client: %s", err)
	}

	createOpts := certificates.CreateOpts{
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
		Domain:      d.Get("domain").(string),
		PrivateKey:  d.Get("private_key").(string),
		Certificate: d.Get("certificate").(string),
		Type:        d.Get("type").(string),
	}

	log.Printf("[DEBUG] Create Options: %#v", createOpts)
	c, err := certificates.Create(networkingClient, createOpts).Extract()
	if err != nil {
		return fmt.Errorf("error creating Certificate: %s", err)
	}

	// If all has been successful, set the ID on the resource
	d.SetId(c.ID)

	return resourceCertificateV2Read(d, meta)
}

func resourceCertificateV2Read(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	networkingClient, err := config.NetworkingV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud networking client: %s", err)
	}

	c, err := certificates.Get(networkingClient, d.Id()).Extract()
	if err != nil {
		return common.CheckDeleted(d, err, "certificate")
	}
	log.Printf("[DEBUG] Retrieved certificate %s: %#v", d.Id(), c)

	mErr := multierror.Append(nil,
		d.Set("name", c.Name),
		d.Set("description", c.Description),
		d.Set("domain", c.Domain),
		d.Set("certificate", c.Certificate),
		d.Set("private_key", c.PrivateKey),
		d.Set("type", c.Type),
		d.Set("create_time", c.CreateTime),
		d.Set("update_time", c.UpdateTime),
		d.Set("region", config.GetRegion(d)),
	)
	return mErr.ErrorOrNil()
}

func resourceCertificateV2Update(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	networkingClient, err := config.NetworkingV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud networking client: %s", err)
	}

	var updateOpts certificates.UpdateOpts
	if d.HasChange("name") {
		updateOpts.Name = d.Get("name").(string)
	}
	if d.HasChange("description") {
		updateOpts.Description = d.Get("description").(string)
	}
	if d.HasChange("domain") {
		updateOpts.Domain = d.Get("domain").(string)
	}
	if d.HasChange("private_key") {
		updateOpts.PrivateKey = d.Get("private_key").(string)
	}
	if d.HasChange("certificate") {
		updateOpts.Certificate = d.Get("certificate").(string)
	}

	log.Printf("[DEBUG] Updating certificate %s with options: %#v", d.Id(), updateOpts)

	timeout := d.Timeout(schema.TimeoutUpdate)
	err = resource.Retry(timeout, func() *resource.RetryError {
		_, err := certificates.Update(networkingClient, d.Id(), updateOpts).Extract()
		if err != nil {
			return common.CheckForRetryableError(err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error updating certificate %s: %s", d.Id(), err)
	}

	return resourceCertificateV2Read(d, meta)
}

func resourceCertificateV2Delete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*cfg.Config)
	client, err := config.NetworkingV2Client(config.GetRegion(d))
	if err != nil {
		return fmt.Errorf("error creating OpenTelekomCloud networking client: %s", err)
	}

	log.Printf("[DEBUG] Deleting certificate %s", d.Id())
	timeout := d.Timeout(schema.TimeoutDelete)
	err = resource.Retry(timeout, func() *resource.RetryError {
		err := certificates.Delete(client, d.Id()).ExtractErr()
		if err != nil {
			return common.CheckForRetryableError(err)
		}
		return nil
	})
	if err != nil {
		return handleCertificateDeletionError(d, client, err)
	}

	return nil
}

func handleCertificateDeletionError(d *schema.ResourceData, client *golangsdk.ServiceClient, err error) error {
	if common.IsResourceNotFound(err) {
		log.Printf("[INFO] deleting an unavailable certificate: %s", d.Id())
		return nil
	}

	if err409, ok := err.(golangsdk.ErrDefault409); ok { // certificate in use
		var dep struct {
			ListenerIDs []string `json:"listener_ids"`
		}
		if err := json.Unmarshal(err409.Body, &dep); err != nil {
			return fmt.Errorf("error loading assigned listeners: %w", err)
		}

		mErr := new(multierror.Error)
		for _, listenerID := range dep.ListenerIDs {
			mErr = multierror.Append(mErr, unassignCertWithRetry(client, d.Timeout(schema.TimeoutUpdate), d.Id(), listenerID))
		}
		if mErr.ErrorOrNil() != nil {
			return mErr
		}

		log.Printf("[DEBUG] Retry deleting certificate %s", d.Id())
		timeout := d.Timeout(schema.TimeoutDelete)
		err := resource.Retry(timeout, func() *resource.RetryError {
			err := certificates.Delete(client, d.Id()).ExtractErr()
			if err != nil {
				return common.CheckForRetryableError(err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("error retrying certificate delition: %w", err)
		}
		return nil
	}
	return fmt.Errorf("error deleting certificate %s: %w", d.Id(), err)
}

func unassignCertWithRetry(client *golangsdk.ServiceClient, timeout time.Duration, certID, listenerID string) error {
	listener, err := listeners.Get(client, listenerID).Extract()
	if err != nil {
		return fmt.Errorf("failed to get listener %s: %w", listenerID, err)
	}

	var otherCerts []string
	for _, cert := range listener.SniContainerRefs {
		if cert != certID {
			otherCerts = append(otherCerts, cert)
		}
	}
	opts := listeners.UpdateOpts{
		SniContainerRefs: otherCerts,
	}
	err = resource.Retry(timeout, func() *resource.RetryError {
		_, err := listeners.Update(client, listener.ID, opts).Extract()
		if err != nil {
			return common.CheckForRetryableError(err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error unassigning certificate %s from listener %s: %w", certID, listener.ID, err)
	}
	return nil
}
