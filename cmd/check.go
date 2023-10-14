/*
Copyright © 2023 Filip Troníček
*/

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type SoftwareVersion struct {
	Cycle             string      `json:"cycle"`
	ReleaseDate       string      `json:"releaseDate"`
	Support           interface{} `json:"support"`
	EOL               string      `json:"eol"`
	Latest            string      `json:"latest"`
	LatestReleaseDate string      `json:"latestReleaseDate"`
	LTS               bool        `json:"lts"`
}

type Variant struct {
	Name string            `yaml:"name"`
	Args map[string]string `yaml:"args"`
}
type Chunk struct {
	Variants []Variant `yaml:"variants"`
}

func capitalize(word string) string {
	if len(word) == 0 {
		return ""
	}
	return strings.ToUpper(string(word[0])) + word[1:]
}

func CheckVersion(name string, version string) (SoftwareVersion, error) {
	httpClient := &http.Client{}
	req, err := http.NewRequest("GET", "https://endoflife.date/api/"+name+".json", nil)
	if err != nil {
		return SoftwareVersion{}, err
	}

	req.Header.Set("User-Agent", "date-reaper-cli")
	resp, err := httpClient.Do(req)
	if err != nil {
		return SoftwareVersion{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SoftwareVersion{}, fmt.Errorf("Error: Server returned status %d", resp.StatusCode)
	}

	var versions []SoftwareVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return SoftwareVersion{}, err
	}

	for _, v := range versions {
		if v.Cycle == version {
			return v, nil
		}
	}
	return SoftwareVersion{}, errors.New("Version not found")
}

var tool string

var checkChunkCmd = &cobra.Command{
	Use:  "check-chunk <path-to-chunk.yaml>",
	Long: "Checks a chunk.yaml's variants for those which are EOL'd",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		chunkPath := args[0]
		chunkFile, err := os.ReadFile(chunkPath)
		if err != nil {
			return fmt.Errorf("Error reading chunk file: %s", err)
		}

		var chunk Chunk
		if err := yaml.Unmarshal(chunkFile, &chunk); err != nil {
			return fmt.Errorf("Error parsing YAML: %s", err)
		}

		for _, variant := range chunk.Variants {
			version := variant.Name
			v, err := CheckVersion(tool, variant.Name)
			if err != nil {
				fmt.Printf("Error checking version %s: %s\n", version, err)
				continue
			}

			now := time.Now().Format("2006-01-02")
			if v.EOL <= now {
				fmt.Printf("Version %s is EOL since %s. Support ended on: %s\n", version, v.EOL, v.Support)
			} else {
				fmt.Printf("Version %s is not EOL yet. It will be EOL on %s.\n", version, v.EOL)
			}
		}

		return nil
	},
}

var failOnMissing bool
var failOnUnsupported bool

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check <name> <version>",
	Short: "Check if a software version is EOL",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, version := args[0], args[1]

		v, err := CheckVersion(name, version)
		if err != nil {
			return err
		}

		var supportEndDate string
		switch supportValue := v.Support.(type) {
		case string:
			supportEndDate = supportValue
		case bool:
			if !supportValue {
				supportEndDate = "No Support"
			}
		default:
			supportEndDate = "Unknown"
		}

		now := time.Now().Format("2006-01-02")
		if v.EOL > now {
			if failOnUnsupported {
				return fmt.Errorf("%s %s is not supported anymore", capitalize(name), version)
			}
			fmt.Printf("%s %s is not EOL yet. It will be EOL on %s. Support ends on %s\n", capitalize(name), version, v.EOL, supportEndDate)
			return nil
		} else {
			fmt.Printf("%s %s is EOL since %s. Support ended on: %s\n", capitalize(name), version, v.EOL, supportEndDate)
			return errors.New("EOL")
		}
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(checkChunkCmd)

	checkCmd.Flags().BoolVarP(&failOnMissing, "fail-on-missing", "m", false, "Fail if the version is not found in the database")
	checkCmd.Flags().BoolVarP(&failOnUnsupported, "fail-on-unsupported", "u", false, "Fail if the version is not supported by regular updates anymore")

	checkChunkCmd.Flags().StringVarP(&tool, "tool", "t", "", "Tool to check versions for")
}
