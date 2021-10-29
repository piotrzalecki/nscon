package namespace

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/fatih/color"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"google.golang.org/api/container/v1"
	"gopkg.in/ini.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp" // register GCP auth provider
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type ProjectNamespaces struct {
	ProjectId string
	Clusters  []ClusterNamespaces
}

type ClusterNamespaces struct {
	ClusterName      string
	Namespaces       []string
}

type NamespaceLocation struct {
	Cluster   string `yaml:"clusterName"`
	ProjectID string `yaml:"projectId"`
	Location string `yaml:"clusterLocation"`
}

func (nl NamespaceLocation) ConnectToCluster() {
	project := nl.ProjectID
	cluster := nl.Cluster
	region := nl.Location

	if runtime.GOOS == "windows" {
		fmt.Println("Can't Execute this on a windows machine")
		os.Exit(1)
	} else {
		color.Green("Connecting to %s GCP project\n", project)
		out := ConfNameForProjectName(project)

		if out == "not-found" {
			color.Red("Configuration name for project %s not found", project)
			os.Exit(1)
		}
		err := exec.Command("gcloud", "config", "configurations", "activate", out).Run()
		if err != nil {
			fmt.Println(err)
		}

		color.Green("Connecting to %s cluster \n", cluster)
		err = exec.Command("gcloud", "container", "clusters", "get-credentials", cluster, "--region", region).Run()
		if err != nil {
			fmt.Println(err)
		}

	}
}

func ScanProjectsForNamespaces(projects []string, verbose bool) []ProjectNamespaces {
	var projectNamespacesList []ProjectNamespaces
	pnsChan := make(chan (ProjectNamespaces))
	for _, project := range projects {
		go func(pr string) {
			if verbose {
				fmt.Println("Scanning for project" + pr)
			}

			pns, err := GetNamespacesForProject(pr)
			if err != nil {
				color.Red("There was problem with scanning namespaces in project", pr)
				fmt.Println(err)
			}
			pnsChan <- pns

		}(project)
	}

	for result := range pnsChan {
		if verbose {
			fmt.Printf("Indexing %d clusters for project %s\n", len(result.Clusters) ,result.ProjectId)
		}
		projectNamespacesList = append(projectNamespacesList, result)
		if len(projectNamespacesList) == len(projects) {
			close(pnsChan)
		}
	}

	return projectNamespacesList
}

func GetNamespacesForProject(project string) (ProjectNamespaces, error) {
	var projectNamespaces ProjectNamespaces
	projectNamespaces, err := GetPNs(context.Background(), project)
	if err != nil {
		return projectNamespaces, err
	}

	return projectNamespaces, nil

}

func GetPNs(ctx context.Context, projectId string) (ProjectNamespaces, error) {
	var projectNamespaces ProjectNamespaces
	projectNamespaces.ProjectId = projectId

	kubeConfig, err := getK8sClusterConfigs(ctx, projectId)
	if err != nil {
		return projectNamespaces, err
	}

	// Just list all the namespaces found in the project to test the API.
	for clusterName := range kubeConfig.Clusters {
		var clusterNamespaces ClusterNamespaces
		clusterNamespaces.ClusterName = clusterName

		cfg, err := clientcmd.NewNonInteractiveClientConfig(*kubeConfig, clusterName, &clientcmd.ConfigOverrides{CurrentContext: clusterName}, nil).ClientConfig()
		if err != nil {
			return projectNamespaces, fmt.Errorf("failed to create Kubernetes configuration cluster=%s: %w", clusterName, err)
		}

		k8s, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			return projectNamespaces, fmt.Errorf("failed to create Kubernetes client cluster=%s: %w", clusterName, err)
		}

		ns, err := k8s.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return projectNamespaces, fmt.Errorf("failed to list namespaces cluster=%s: %w", clusterName, err)
		}

		for _, item := range ns.Items {
			clusterNamespaces.Namespaces = append(clusterNamespaces.Namespaces, item.Name)
			// log.Println(item.Name)
		}
		// fmt.Println(clusterNamespaces)
		projectNamespaces.Clusters = append(projectNamespaces.Clusters, clusterNamespaces)
	}

	return projectNamespaces, nil
}

func getK8sClusterConfigs(ctx context.Context, projectId string) (*api.Config, error) {
	svc, err := container.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("container.NewService: %w", err)
	}

	// Basic config structure
	ret := api.Config{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters:   map[string]*api.Cluster{},  // Clusters is a map of referencable names to cluster configs
		AuthInfos:  map[string]*api.AuthInfo{}, // AuthInfos is a map of referencable names to user configs
		Contexts:   map[string]*api.Context{},  // Contexts is a map of referencable names to context configs
	}

	// Ask Google for a list of all kube clusters in the given project.
	resp, err := svc.Projects.Zones.Clusters.List(projectId, "-").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("clusters list project=%s: %w", projectId, err)
	}

	for _, f := range resp.Clusters {
		name := fmt.Sprintf("gke_%s_%s_%s", projectId, f.Zone, f.Name)
		cert, err := base64.StdEncoding.DecodeString(f.MasterAuth.ClusterCaCertificate)
		if err != nil {
			return nil, fmt.Errorf("invalid certificate cluster=%s cert=%s: %w", name, f.MasterAuth.ClusterCaCertificate, err)
		}
		// example: gke_my-project_us-central1-b_cluster-1 => https://XX.XX.XX.XX
		ret.Clusters[name] = &api.Cluster{
			CertificateAuthorityData: cert,
			Server:                   "https://" + f.Endpoint,
		}
		// Just reuse the context name as an auth name.
		ret.Contexts[name] = &api.Context{
			Cluster:  name,
			AuthInfo: name,
		}
		// GCP specific configation; use cloud platform scope.
		ret.AuthInfos[name] = &api.AuthInfo{
			AuthProvider: &api.AuthProviderConfig{
				Name: "gcp",
				Config: map[string]string{
					"scopes": "https://www.googleapis.com/auth/cloud-platform",
				},
			},
		}
	}

	return &ret, nil
}

func ConfNameForProjectName(name string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		color.Red("cant obtain home directory", err)
		fmt.Println(err)
		os.Exit(1)
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
				color.Red("can't parse configuratino file %s", f.Name())
				os.Exit(1)
			}
			project := cfg.Section("core").Key("project")
			if strings.EqualFold(project.String(), name) {
				exp := strings.Split(string(f.Name()), "_")
				return exp[1]
			}
		}
	} else {
		color.Red("No google cloud configuration files found")
		os.Exit(1)
	}

	return "not-found"
}
