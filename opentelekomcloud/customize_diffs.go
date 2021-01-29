package opentelekomcloud

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/blockstorage/v1/volumetypes"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v1/subnets"
	"github.com/opentelekomcloud/gophertelekomcloud/openstack/networking/v1/vpcs"
)

func validateRDSv3Version(argumentName string) schema.CustomizeDiffFunc {
	return func(d *schema.ResourceDiff, meta interface{}) error {
		config, ok := meta.(*Config)
		if !ok {
			return fmt.Errorf("error retreiving configuration: can't convert %v to Config", meta)
		}

		rdsClient, err := config.rdsV3Client(GetRegion(d, config))
		if err != nil {
			return fmt.Errorf("error creating OpenTelekomCloud RDSv3 Client: %s", err)
		}

		dataStoreInfo := d.Get(argumentName).([]interface{})[0].(map[string]interface{})
		datastoreVersions, err := getRdsV3VersionList(rdsClient, dataStoreInfo["type"].(string))
		if err != nil {
			return fmt.Errorf("unable to get datastore versions: %s", err)
		}

		var matches = false
		for _, datastore := range datastoreVersions {
			if datastore == dataStoreInfo["version"] {
				matches = true
				break
			}
		}
		if !matches {
			return fmt.Errorf("can't find version `%s`", dataStoreInfo["version"])
		}

		return nil
	}
}

func validateCCEClusterNetwork(d *schema.ResourceDiff, meta interface{}) error {
	config, ok := meta.(*Config)
	if !ok {
		return fmt.Errorf("error retreiving configuration: can't convert %v to Config", meta)
	}
	vpcClient, err := config.networkingV1Client(GetRegion(d, config))
	if err != nil {
		return fmt.Errorf("error creating opentelekomcloud CCE Client: %s", err)
	}

	if vpcID := d.Get("vpc_id").(string); vpcID != "" {
		if err = vpcs.Get(vpcClient, vpcID).Err; err != nil {
			return fmt.Errorf("can't find VPC `%s`: %s", vpcID, err)
		}
	}

	if subnetID := d.Get("subnet_id").(string); subnetID != "" {
		if err = subnets.Get(vpcClient, subnetID).Err; err != nil {
			return fmt.Errorf("can't find subnet `%s`: %s", subnetID, err)
		}
	}

	return nil
}

const argMissingMsg = "schema missing %s argument"

func validateVolumeType(argName string) schema.CustomizeDiffFunc {
	return func(d *schema.ResourceDiff, meta interface{}) error {
		volumeType := d.Get(argName)
		if volumeType == nil {
			return fmt.Errorf(argMissingMsg, argName)
		}
		expectedType := strings.ToLower(volumeType.(string))
		if expectedType == "" {
			return nil
		}

		expectedAZ := d.Get("availability_zone")
		if expectedAZ == "" {
			log.Printf("[DEBUG] No AZ provided, can't define available volume types")
			return nil
		}
		config := meta.(*Config)
		client, err := config.blockStorageV3Client(GetRegion(d, config))
		if err != nil {
			return fmt.Errorf("error creating blockstorage v3 client: %s", err)
		}

		pages, err := volumetypes.List(client).AllPages()
		if err != nil {
			return fmt.Errorf("error retrieving volume types: %s", err)
		}
		types, err := volumetypes.ExtractVolumeTypes(pages)
		if err != nil {
			return err
		}

		var zones []string
		for _, t := range types {
			actualType := strings.ToLower(t.Name)
			if actualType == expectedType {
				zones = getZonesFromVolumeType(t)
				break
			}
		}
		if len(zones) == 0 {
			return fmt.Errorf("volume type `%s` doesn't exist", volumeType)
		}
		for _, az := range zones {
			if az == expectedAZ {
				return nil
			}
		}

		return fmt.Errorf(
			"volume type `%v` is not supported in AZ `%s`.\nSupported AZs: %v",
			volumeType, expectedAZ, zones,
		)
	}
}

func getZonesFromVolumeType(t volumetypes.VolumeType) []string {
	zonesStr := t.ExtraSpecs["RESKEY:availability_zones"].(string)
	return strings.Split(zonesStr, ",")
}

func validateVPC(argName string) schema.CustomizeDiffFunc {
	return func(d *schema.ResourceDiff, meta interface{}) error {
		vpcID := d.Get(argName)
		if vpcID == nil {
			return fmt.Errorf(argMissingMsg, argName)
		}
		if vpcID == "" {
			return nil
		}
		config := meta.(*Config)
		vpcClient, err := config.networkingV1Client(GetRegion(d, config))
		if err != nil {
			return fmt.Errorf("error creating opentelekomcloud CCE Client: %s", err)
		}
		if err := vpcs.Get(vpcClient, vpcID.(string)).Err; err != nil {
			return fmt.Errorf("can't find VPC `%s`: %s", vpcID, err)
		}
		return nil
	}
}

func multipleCustomizeDiffs(funcs ...schema.CustomizeDiffFunc) schema.CustomizeDiffFunc {
	return func(d *schema.ResourceDiff, meta interface{}) error {
		mErr := &multierror.Error{}
		for _, fn := range funcs {
			mErr = multierror.Append(mErr, fn(d, meta))
		}
		return mErr.ErrorOrNil()
	}
}
