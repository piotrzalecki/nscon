package ui

import (
	"bufio"
	"fmt"
	"github.com/fatih/color"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/piotrzalecki/nscon/internal/namespace"
)


func MultipleNamespaceLocationsPrompt(nsls []namespace.NamespaceLocation) namespace.NamespaceLocation {
	color.Green("This namespace exists in multiple locations:")

	for i, nl := range nsls {
		if i % 2 ==0 {
			fmt.Printf("%-2d\tcluster: %-25s\tproject: %-s\n", i, nl.Cluster, nl.ProjectID)
		} else{
			color.Cyan("%-2d\tcluster: %-25s\tproject: %-s\n", i, nl.Cluster, nl.ProjectID)
		}

	}
	color.Green("where do you want to connect (pick option number):")
	reader := bufio.NewReader(os.Stdin)
	num, err := reader.ReadString('\n')

	if err != nil {
		log.Panic(err)
	}
	nli, err := strconv.Atoi(strings.TrimSuffix(num, "\n"))
	if err != nil {
		log.Panic(err)
	}
	return nsls[nli]
}
