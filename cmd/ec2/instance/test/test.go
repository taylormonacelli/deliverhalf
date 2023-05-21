/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/k0kubun/pp"
	"github.com/spf13/cobra"
	common "github.com/taylormonacelli/deliverhalf/cmd/common"
	db "github.com/taylormonacelli/deliverhalf/cmd/db"
	myec2 "github.com/taylormonacelli/deliverhalf/cmd/ec2"
	instance "github.com/taylormonacelli/deliverhalf/cmd/ec2/instance"
	log "github.com/taylormonacelli/deliverhalf/cmd/logging"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("test called")
		// writeInstancesJsonToFiles()
		// testReadInstanceFromJsonFile()
		// testWriteExtendedInstanceJsonDb()
		queryExtendedInstancesFromDb()
	},
}

func init() {
	instance.InstanceCmd.AddCommand(testCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// testCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// testCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func testReadInstanceFromJsonFile() (types.Instance, error) {
	fname := "data/types.instance/i-02a6935a1e72ebffd.json"

	inst, err := readInstanceFromJsonFile(fname)
	if err != nil {
		panic(err)
	}
	pp.Print(inst)
	return inst, nil
}

func readInstanceFromJsonFile(pathToJsonFile string) (types.Instance, error) {
	var inst types.Instance

	jsonBlob, err := os.ReadFile(pathToJsonFile)
	if err != nil {
		log.Logger.Fatalf("Reading JSON into byte slice failed with error: %s", err)
	}

	// unmarshal the JSON data into the map
	err = json.Unmarshal(jsonBlob, &inst)
	if err != nil {
		return types.Instance{}, err
	}
	return inst, nil
}

func queryExtendedInstancesFromDb() {
	conn, err := db.ConnectToSQLiteDB("test.db")
	if err != nil {
		log.Logger.Fatalf("failed to connect to database: %v", err)
	}
	defer func() {
		sqlDB, err := conn.DB()
		if err != nil {
			log.Logger.Fatalf("failed to get underlying database connection: %v", err)
		}
		if err := sqlDB.Close(); err != nil {
			log.Logger.Fatalf("failed to close database connection: %v", err)
		}
	}()

	conn.AutoMigrate(&instance.ExtendedInstance{})

	var extInst instance.ExtendedInstance
	writeInstancesJsonToFiles()
	testWriteExtendedInstanceJsonDb()
	conn.First(&extInst, 1)

	var instance types.Instance
	json.Unmarshal([]byte(extInst.JsonDef), &instance)

	fmt.Println(extInst.JsonDef)
	// fmt.Println(instance.InstanceId)

	// Create an empty array to store the strings
	var volumeIds []string

	// Loop over the list and append each item to the array
	for _, item := range instance.BlockDeviceMappings {
		volumeIds = append(volumeIds, *item.Ebs.VolumeId)
	}

	// Print the array volume ids
	for _, volId := range volumeIds {
		fmt.Println(volId)
	}
}

func testWriteExtendedInstanceJsonDb() {
	writeExtendedInstanceJsonDb()
}

func writeExtendedInstanceJsonDb() {
	conn, err := db.ConnectToSQLiteDB("test.db")
	if err != nil {
		log.Logger.Fatalf("failed to connect to database: %v", err)
	}
	defer func() {
		sqlDB, err := conn.DB()
		if err != nil {
			log.Logger.Fatalf("failed to get underlying database connection: %v", err)
		}
		if err := sqlDB.Close(); err != nil {
			log.Logger.Fatalf("failed to close database connection: %v", err)
		}
	}()

	inst, err := testReadInstanceFromJsonFile()
	if err != nil {
		panic(err)
	}

	jsonData, err := json.MarshalIndent(inst, "", "  ")
	if err != nil {
		panic(err)
	}

	var instExt instance.ExtendedInstance
	instExt.JsonDef = string(jsonData)
	instExt.InstanceId = *inst.InstanceId
	conn.AutoMigrate(&instance.ExtendedInstance{})
	conn.Create(&instExt)
}

func writeInstancesJsonToFiles() {
	client, err := myec2.GetEc2Client("us-west-2")
	if err != nil {
		log.Logger.Errorln(err)
	}

	input := &ec2.DescribeInstancesInput{
		// Filters: []types.Filter{
		// 	{
		// 		Name:   aws.String("instance-state-name"),
		// 		Values: []string{"running"},
		// 	},
		// },
	}

	resp, err := client.DescribeInstances(context.TODO(), input)
	if err != nil {
		fmt.Println("Failed to describe instances:", err)
		return
	}

	for _, reservation := range resp.Reservations {
		for _, instance := range reservation.Instances {
			fileName := getInstanceFileName(instance)
			path, err := filepath.Abs(filepath.Join("data", "types.instance", fileName))
			if err != nil {
				log.Logger.Errorln(err)
			}
			if common.FileExists(path) {
				log.Logger.Warnf("skipping %s because it already exists", path)
				continue
			}
			if err := writeInstanceDetailsToFile(path, instance); err != nil {
				fmt.Printf("Failed to write instance details to file: %v\n", err)
				continue
			}
			fmt.Printf("Successfully wrote instance details to file: %s\n", path)
		}
	}
}

func getInstanceFileName(instance types.Instance) string {
	return *instance.InstanceId + ".json"
}

func writeInstanceDetailsToFile(fileName string, instance types.Instance) error {
	common.EnsureParentDirectoryExists(fileName)

	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create file for instance %s: %v", *instance.InstanceId, err)
	}
	defer file.Close()

	data, err := json.MarshalIndent(instance, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON for instance %s: %v", *instance.InstanceId, err)
	}

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write JSON data to file for instance %s: %v", *instance.InstanceId, err)
	}

	return nil
}
