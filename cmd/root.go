/*
Copyright © 2021 Piotr Zalecki piotrzalecki@gmail.com

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/fatih/color"

	"github.com/spf13/cobra"

	"github.com/piotrzalecki/nscon/internal/gcp"
	"github.com/piotrzalecki/nscon/internal/inventory"
	"github.com/piotrzalecki/nscon/internal/namespace"
	"github.com/piotrzalecki/nscon/internal/ui"
	"github.com/spf13/viper"
)

var (
	cfgFile                    string
	project                    string
	cluster                    string
	verbose                    bool
	scan                       bool
	ns                         string
	namespaceInventoryLocation string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nscon NAMESPACE_NAME",
	Short: "nscon allows you quickly switch between namespaces in differnet GKE clusters",
	Long: `nscon it is tool that allows you to easily and quickly connect to clusters based on given namespace name.
It scans all GKE clusters in all GCP projects you have configured for you user, and index all namespaces in cluster.
File with namespace inventory is stored in your home directory under $HOME/.nscon/namespaces.yaml.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			if !scan {
				return errors.New("requires namespace name as an argument")
			}
			return nil
		} else if len(args) > 1 {
			return errors.New("takes only one argument (namespace name)")
		}
		return nil
	},
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		//var nI inventory.Inventory

		nI := make(inventory.Inventory)
		namespaceInventoryLocation = viper.GetString("inventory_location")
		if scan {
			color.Green("Scaning...")
			projects := gcp.GetProjectsList()
			result := namespace.ScanProjectsForNamespaces(projects, verbose)
			nI.CreateFromProjectNamespaces(result)
			nI.Save(namespaceInventoryLocation)
			color.Green("Inventory saved to file %s\n", namespaceInventoryLocation)

		} else {
			ns = args[0]

			nIl, err := nI.Load(namespaceInventoryLocation)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			var nls []namespace.NamespaceLocation
			if cluster != "" {
				nls = nIl.NamespaceLocationForCluster(ns, cluster)
				if len(nls) > 1 {
					location := ui.MultipleNamespaceLocationsPrompt(nls)
					location.ConnectToCluster()
				} else if len(nls) == 1 {
					nls[0].ConnectToCluster()
				} else {
					color.Red("Can't find namespace %s in cluster %s\n", ns, project)
					fmt.Println("HINT: Check if data you provided is correct. " +
						"If you are sure namespace exists use --scan flag to update namespace inventory")
				}
			} else if project != "" {
				nls = nIl.NamespaceLocationForProject(ns, project)
				if len(nls) > 1 {
					location := ui.MultipleNamespaceLocationsPrompt(nls)
					location.ConnectToCluster()
				} else if len(nls) == 1 {
					nls[0].ConnectToCluster()
				} else {
					color.Red("Can't find namespace %s in project %s\n", ns, project)
					fmt.Println("HINT: Check if data you provided is correct. " +
						"If you are sure namespace exists use --scan flag to update namespace inventory")
				}

			} else {
				nls = nIl[ns]
				if len(nls) > 1 {
					location := ui.MultipleNamespaceLocationsPrompt(nls)
					location.ConnectToCluster()
				} else if len(nls) == 1 {
					location := nls[0]
					location.ConnectToCluster()
				} else {
					color.Red("Namespace %s not found !!\n", ns)
					fmt.Println("HINT: Check if data you provided is correct. " +
						"If you are sure namespace exists use --scan flag to update namespace inventory")
				}

			}

		}

	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "gives verbose output of some actions application will perform")
	rootCmd.PersistentFlags().BoolVarP(&scan, "scan", "s", false, "scans all clusters in all cloud projects for namespaces")
	rootCmd.PersistentFlags().StringVarP(&project, "project", "p", "", "project you want to search namespaces in")
	rootCmd.PersistentFlags().StringVarP(&cluster, "cluster", "c", "", "cluster name you want to search namespaces in")
	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Find home directory.
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in home directory with name ".nscon" (without extension).
		viper.AddConfigPath(home + "/.nscon")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		color.Red("Configuration file not found. Creating default configuration")

		configDir := home + "/.nscon"
		configFilePath := configDir + "/config.yaml"
		err := os.Mkdir(home+"/.nscon", 0755)
		if err != nil {
			log.Fatal(err)
		}
		f, err := os.OpenFile(configFilePath, os.O_CREATE, 0755)
		if err != nil {
			log.Fatal(err)
		}
		f.Close()

		nsInventoryLocation := configDir + "/namespaces.yaml"
		f, err = os.OpenFile(nsInventoryLocation, os.O_CREATE, 0755)
		if err != nil {
			log.Fatal(err)
		}
		f.Close()

		viper.Set("inventory_location", nsInventoryLocation)
		viper.WriteConfigAs(configFilePath)
		err = viper.WriteConfig()
		if err != nil {
			color.Red("Config file initialisation filed")
			fmt.Println(err)
		}

		color.Green("Configuration initialised!! Execute 'nscon --scan' to create namespaces inventory")
		os.Exit(0)

	}
}

//TODO: refactor code !!
//TODO: standard input implementation
