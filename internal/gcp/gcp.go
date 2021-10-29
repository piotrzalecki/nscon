package gcp

import (
	"fmt"
	"github.com/fatih/color"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/go-ini/ini"
)


func GetProjectsList() []string {

	var projects []string
	homeDir, err := os.UserHomeDir()
	if err != nil {

		log.Panic(color.RedString("cant obtain home directory", err))
	}

	configDirPAth := fmt.Sprintf("%s/.config/gcloud/configurations", homeDir)
	var files []fs.FileInfo
	if _, err := os.Stat(configDirPAth); !os.IsNotExist(err) {
		files, err = ioutil.ReadDir(configDirPAth)
		if err != nil {
			log.Fatal(err)
		}
	}

	if len(files) > 0 {
		for _, f := range files {
			cfg, err := ini.Load(fmt.Sprintf("%s/.config/gcloud/configurations/%s", homeDir, f.Name()))
			if err != nil {
				log.Panic(color.RedString("can't parse configuratino file %s", f.Name()))

			}
			account := cfg.Section("core").Key("account")
			project := cfg.Section("core").Key("project")

			if strings.Contains(account.String(), "@blackduckcloud.com") {
				// pl.Projects = append(pl.Projects, project.String())
				projects = append(projects, project.String())
			}
		}
	} else {
		log.Panic(color.RedString("no google cloud configuration files found"))
	}
	// return pl.Projects
	return projects
}
