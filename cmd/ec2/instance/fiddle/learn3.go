//lint:file-ignore U1000 Return to this when i've pulled my head out of my ass
package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/spf13/cobra"

	mydb "github.com/taylormonacelli/deliverhalf/cmd/db"
	instance "github.com/taylormonacelli/deliverhalf/cmd/ec2/instance"
	volume "github.com/taylormonacelli/deliverhalf/cmd/ec2/volume"
	log "github.com/taylormonacelli/deliverhalf/cmd/logging"
)

// learn2Cmd represents the learn2 command
var learn3Cmd = &cobra.Command{
	Use:   "learn3",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Logger.Traceln("learn3 called")
		testExtractBlockDeviceMappingsFromInstanceWithName()
	},
}

func init() {
	fiddleCmd.AddCommand(learn3Cmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// learn3Cmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// learn2Cmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func testExtractBlockDeviceMappingsFromInstanceWithName() {
	var instanceNames []string
	mydb.Db.Table("extended_instance_details").Select("DISTINCT name").Find(&instanceNames)
	extractBlockDeviceMappingsFromInstanceWithName1(&instanceNames)
}

func extractBlockDeviceMappingsFromInstanceWithName(instanceName string) {
	var extendedInstances []instance.ExtendedInstanceDetail

	query := `
		JOIN (
			SELECT MAX(created_at) AS max_created_at
			FROM extended_instance_details
			GROUP BY name
		) AS subquery ON extended_instance_details.created_at = subquery.max_created_at
	`
	mydb.Db.Table("extended_instance_details").
		Select("extended_instance_details.*").
		Joins(query).
		Where("name = ?", instanceName).
		Find(&extendedInstances)

	if len(extendedInstances) != 1 {
		log.Logger.Fatalf("expect to find only a single instance for instance name %s", instanceName)
	}
	extInst := extendedInstances[0]

	var inst types.Instance
	err := json.Unmarshal([]byte(extInst.JsonDef), &inst)
	if err != nil {
		log.Logger.Fatal(err)
	}

	eebdms := make([]*volume.ExtendedEc2BlockDeviceMapping, 0)

	for _, mapping := range inst.BlockDeviceMappings {
		volumeId := aws.StringValue(mapping.Ebs.VolumeId)
		jsonData, err := json.MarshalIndent(mapping, "", "  ")
		if err != nil {
			log.Logger.Fatalln(err)
		}
		x1 := volume.ExtendedEc2BlockDeviceMapping{
			InstanceId:   *inst.InstanceId,
			JsonDef:      string(jsonData),
			InstanceName: instanceName,
			VolumeId:     volumeId,
		}
		eebdms = append(eebdms, &x1)
	}
	result := mydb.Db.Create(eebdms)

	if result.Error != nil {
		log.Logger.Errorln("error occurred:", result.Error)
		return
	}

	log.Logger.Debugf("%d block device mappings added", result.RowsAffected)
}

func extractBlockDeviceMappingsFromInstanceWithName1(instanceNames *[]string) {
	var extendedInstances []instance.ExtendedInstanceDetail

	query := `
		JOIN (
			SELECT MAX(created_at) AS max_created_at
			FROM extended_instance_details
			GROUP BY name
		) AS subquery ON extended_instance_details.created_at = subquery.max_created_at
	`
	mydb.Db.Table("extended_instance_details").
		Select("extended_instance_details.*").
		Joins(query).
		Find(&extendedInstances)

	instanceIds := make([]string, 0)
	volumeIds := make([]string, 0)

	for _, extInst := range extendedInstances {
		var inst types.Instance
		err := json.Unmarshal([]byte(extInst.JsonDef), &inst)
		if err != nil {
			log.Logger.Fatal(err)
		}

		for _, bdMap := range inst.BlockDeviceMappings {
			volumeId := aws.StringValue(bdMap.Ebs.VolumeId)
			instanceIds = append(instanceIds, *inst.InstanceId)
			volumeIds = append(volumeIds, volumeId)
		}
	}

	existingMappings := []volume.ExtendedEc2BlockDeviceMapping{}
	mydb.Db.Where("instance_id IN (?) AND volume_id IN (?)", instanceIds, volumeIds).Find(&existingMappings)

	if mydb.Db.Error != nil {
		log.Logger.Fatalln(mydb.Db.Error)
	}

	existingMappingsMap := make(map[string]volume.ExtendedEc2BlockDeviceMapping)
	for _, existingMapping := range existingMappings {
		key := existingMapping.InstanceId + existingMapping.VolumeId
		existingMappingsMap[key] = existingMapping
	}

	// Prepare the values for the bulk update
	var updates []string
	var values []interface{}

	for _, extInst := range extendedInstances {
		var inst types.Instance
		err := json.Unmarshal([]byte(extInst.JsonDef), &inst)
		if err != nil {
			log.Logger.Fatal(err)
		}

		for _, bdMap := range inst.BlockDeviceMappings {
			volumeId := aws.StringValue(bdMap.Ebs.VolumeId)
			jsonData, err := json.MarshalIndent(bdMap, "", "  ")
			if err != nil {
				log.Logger.Fatalln(err)
			}
			eebdm := volume.ExtendedEc2BlockDeviceMapping{
				InstanceId:   *inst.InstanceId,
				JsonDef:      string(jsonData),
				InstanceName: extInst.Name,
				VolumeId:     volumeId,
			}

			// Generate the SQL update statement and values
			updates = append(updates, "(?, ?, ?, ?, ?, ?)")
			values = append(values, eebdm.InstanceName, eebdm.JsonDef, eebdm.InstanceId, eebdm.VolumeId, time.Now(), time.Now())
		}
	}

	query = `INSERT INTO extended_ec2_block_device_mappings
	(
		instance_name,
		json_def,
		instance_id,
		volume_id,
		created_at,
        updated_at
	)
	VALUES %s
	ON conflict
	(
		instance_id,
		volume_id
	)
	DO UPDATE
	SET instance_name = excluded.instance_name,
		json_def = excluded.json_def
	`

	// Perform the bulk update
	updateStatement := fmt.Sprintf(query, strings.Join(updates, ", "))
	result := mydb.Db.Exec(updateStatement, values...)

	if result.Error != nil {
		log.Logger.Error(result.Error)
	}
}
