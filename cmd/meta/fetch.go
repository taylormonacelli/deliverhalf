/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	log "github.com/taylormonacelli/deliverhalf/cmd/logging"
)

// fetchCmd represents the fetch command
var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		data := Fetch()
		log.Logger.Traceln(getMapAsString(data))
	},
}

func init() {
	metaCmd.AddCommand(fetchCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// fetchCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// fetchCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func parseData(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("error parsing JSON data: %s", err)
	}
	return result, nil
}

func addFetchTimestamp(data map[string]interface{}) map[string]interface{} {
	newData := map[string]interface{}{
		"fetchTimestamp": time.Now(),
	}
	for k, v := range newData {
		data[k] = v
	}
	return data
}

func mergeData(data []byte) map[string]interface{} {
	parsedData, err := parseData(data)
	if err != nil {
		log.Logger.Fatalf("Error parsing JSON data:%s", err)
	}

	newData := addFetchTimestamp(parsedData)
	return newData
}

func mapToJsonStr(data map[string]interface{}) string {
	// Convert the map to a flat JSON string
	jsonStr, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Logger.Println("Error parsing JSON data:", err)
	}
	log.Logger.Printf("json: %s", jsonStr)
	return string(jsonStr)
}

func toJsonPrettyStr(data map[string]interface{}) string {
	// Convert the map to a pretty JSON string
	jsonStrPretty, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		log.Logger.Println("Error marshaling data:", err)
	}
	log.Logger.Printf("json: %s", jsonStrPretty)
	return string(jsonStrPretty)
}

func fetchData() ([]byte, error) {
	url := "http://169.254.169.254/latest/dynamic/instance-identity/document"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %s", err)
	}

	client := &http.Client{
		Timeout: time.Second * 2,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making HTTP request: %s", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %s", err)
	}

	return body, nil
}

func enableMetaDataFetching() {
	err := os.Unsetenv("AWS_EC2_METADATA_DISABLED")
	if err != nil {
		log.Logger.Warn("Error unsetting environment variable:", err)
		return
	}
	log.Logger.Trace("environment variable AWS_EC2_METADATA_DISABLED unset successfully.")
}

func Fetch() map[string]interface{} {
	enableMetaDataFetching()
	body, err := fetchData()
	if err != nil {
		log.Logger.Fatalf("Error fetching data: %s", err)
	}

	mergedData := mergeData(body)
	return mergedData
}
