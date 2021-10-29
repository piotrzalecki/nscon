package inventory

import (
	"io/ioutil"
	"strings"

	"github.com/piotrzalecki/nscon/internal/namespace"
	"gopkg.in/yaml.v2"
)

type Inventory map[string][]namespace.NamespaceLocation

func (i Inventory) Save(fileName string) error {

	yamlData, err := yaml.Marshal(&i)

	if err != nil {
		return err
	}

	err = ioutil.WriteFile(fileName, yamlData, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (i Inventory) Load(fileName string) (Inventory, error) {
	yamlFile, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	ip := &i
	err = yaml.Unmarshal(yamlFile, ip)
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (i Inventory)CreateFromProjectNamespaces(pns []namespace.ProjectNamespaces){
	for _, projectNamespace := range pns{
		projectId := projectNamespace.ProjectId

		for _, cluster := range projectNamespace.Clusters{
			clusterGKEname := cluster.ClusterName
			splittedGKEname := strings.Split(clusterGKEname, "_")
			clusterName := splittedGKEname[3]
			clusterLocation := splittedGKEname[2]

			for _, ns := range cluster.Namespaces{
				namespaceLocation :=  namespace.NamespaceLocation {
					Cluster: clusterName,
					ProjectID: projectId,
					Location: clusterLocation,
				}
				i[ns] = append(i[ns], namespaceLocation)
			}
		}

	}
}

func (i Inventory) NamespaceLocationForCluster(ns, cluster string) []namespace.NamespaceLocation {
	var nls []namespace.NamespaceLocation
	for _, nl := range i[ns] {
		if strings.Contains(nl.Cluster, cluster) {
			nls = append(nls, nl)
		}
	}
	return nls
}

func (i Inventory) NamespaceLocationForProject(ns, project string) []namespace.NamespaceLocation {
	var nls []namespace.NamespaceLocation
	for _, nl := range i[ns] {
		if strings.Contains(nl.ProjectID, project) {
			nls = append(nls, nl)
		}
	}
	return nls
}
