package ugotsrvd

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"text/template"

	"github.com/gin-gonic/gin"
)

const uploadDir = "/Users/composr/work/ugotsrvd-data/upload"
const chartsBaseDir = "/Users/composr/work/ugotsrvd-data/generated/charts"
const appsBaseDir = "/Users/composr/work/ugotsrvd-data/generated/apps"
const repoBaseDir = "/Users/composr/work/ugotsrvd-data/repos"

const gitRepo = "autocharts"
const gitAccount = "https://github.com/rcompos"
const gitUsername = "rcompos"

func Package(c *gin.Context) {
	fileList := filesInDir(uploadDir)
	yamlList := []string{}
	for _, f := range fileList {
		fileExtension := filepath.Ext(f)
		log.Println("fileExtension:", fileExtension)
		if fileExtension == ".yaml" {
			yamlList = append(yamlList, f)
		}
	}
	log.Println("CAPI YAML manifests:\n", yamlList)
	c.HTML(http.StatusOK, "package.tmpl", gin.H{"yamlList": yamlList})
}

// TODO: Change to multiple file upload
// https://github.com/gin-gonic/examples/tree/master/upload-file/multiple
func Upload(c *gin.Context) {
	name := c.PostForm("name")
	email := c.PostForm("email")

	// Source
	file, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, "get form err: %s", err.Error())
		return
	}

	createCfgDir(uploadDir)
	filename := uploadDir + "/" + filepath.Base(file.Filename)
	if err := c.SaveUploadedFile(file, filename); err != nil {
		c.String(http.StatusBadRequest, "upload file err: %s", err.Error())
		return
	}

	c.String(http.StatusOK, "File %s uploaded successfully with fields name=%s and email=%s.", file.Filename, name, email)
}

func Create(c *gin.Context) {
	environment := c.PostForm("environment")
	fileList := filesInDir(uploadDir)
	log.Println("Config files:\n", fileList)
	file := c.PostForm("file")

	filename := uploadDir + "/" + file
	// gitRepo := "autocharts"
	// gitUrl := "https://github.com/rcompos/" + gitRepo
	gitUrl := gitAccount + "/" + gitRepo
	// gitUsername := "rcompos"
	token := os.Getenv("GITHUB_TOKEN")

	if !fileExists(filename) { // file not exists is bad
		c.String(http.StatusOK, "File %s not found!", filename)
		return
	}

	chartnameTmp := fmt.Sprintf("%v-%v", environment, file)
	extension := path.Ext(chartnameTmp)
	chartname := chartnameTmp[0 : len(chartnameTmp)-len(extension)]

	repoDir := fmt.Sprintf("%v/%v", repoBaseDir, gitRepo)
	cloneOrPullRepo(gitUrl, repoDir, gitUsername, token)

	// Cluster-API Cluster Helm Chart
	// Create Helm chart
	pathToChart := createHelmChart(chartname, filename, chartsBaseDir)
	log.Println("pathToChart:", pathToChart)

	// Check if chart already exists
	chartDir := fmt.Sprintf("%v/%v", repoDir, chartname)
	if fileExists(chartDir) {
		log.Println("Chart already exists!", chartDir)
		c.String(http.StatusOK, "Chart already exists! %s", chartDir)
		return
	}
	copyToRepo(pathToChart, repoDir)
	messageChart := "Add new Helm Chart." + chartname
	gitCommit(repoDir, messageChart, chartname)

	// ArgoCD Helm Chart
	// Create ArgoCD application yaml from template
	appChartName := chartname + "-app"
	templateFile := "argocd-templates/argocd-application.yaml"
	pathToApp := CreateArgoCDApp(appChartName, templateFile, appsBaseDir)
	log.Println("pathToApp:", pathToApp)

	pathToAppChart := createHelmChart(appChartName, pathToApp, appsBaseDir)
	log.Println("pathToAppChart:", pathToAppChart)

	// Check if chart already exists
	appChartDir := fmt.Sprintf("%v/%v", repoDir, appChartName)
	if fileExists(appChartDir) {
		log.Println("Chart already exists!", appChartDir)
		c.String(http.StatusOK, "Chart already exists! %s", appChartDir)
		return
	}
	copyToRepo(pathToAppChart, repoDir)
	messageAppChart := "Add new ArgoCD app." + appChartName
	gitCommit(repoDir, messageAppChart, appChartName)

	// Git push workload cluster Helm chart
	gitPush(repoDir, gitUsername, token)
	c.String(http.StatusOK, "CAPI Workload Cluster Helm and ArgoCD app charts pushed! %s", chartDir)
}

type ArgoCDApp struct {
	Appname              string
	Namespace            string
	Project              string
	RepoURL              string
	TargetRevision       string
	Path                 string
	ReleaseName          string
	ValueFiles           []string
	HelmVersion          string
	DestinationServer    string
	DestinationNamespace string
}

func CreateArgoCDApp(appname, templateFile, appsBaseDir string) string {
	// TODO: Add more template params
	log.Println("appname:", appname)
	log.Println("templateFile:", templateFile)
	log.Println("appsBaseDir:", appsBaseDir)

	tmp, err := template.ParseFiles("argocd-templates/argocd-application.yaml")
	if err != nil {
		log.Fatal(err)
	}

	argoCDAppFile := appsBaseDir + "/" + appname + ".yaml"
	log.Println("argoCDAppFile:", argoCDAppFile)
	f, err := os.Create(argoCDAppFile)
	if err != nil {
		log.Println("create file: ", err)
		return ""
	}
	defer f.Close()

	data := ArgoCDApp{
		Appname:        appname,
		Project:        "default",
		RepoURL:        "https://github.com/rcompos/autocharts",
		TargetRevision: "main",
		Path:           appname,
		ReleaseName:    appname,
		// ValueFiles:           []string{"values.yaml", "values-prod-0.yaml"},
		ValueFiles:           []string{"values.yaml"},
		HelmVersion:          "v3",
		DestinationServer:    "https://kubernetes.default.svc",
		DestinationNamespace: appname,
	}
	err = tmp.Execute(f, data)
	if err != nil {
		log.Print("execute: ", err)
		return ""
	}
	return argoCDAppFile
}

func createHelmChart(chartName, yamlFile, chartsDir string) string {
	// chartName: Helm chart name
	// yamlFile: YAML file
	// chartsDir: Directory for Helm charts

	cmd := fmt.Sprintf("cd %v; helm create %v", chartsDir, chartName)
	log.Println(cmd)
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		log.Printf("Failed to execute command: %s", cmd)
		log.Printf("Error: %v", err)
		return ""
	}
	log.Println(string(out))

	// Clear out templates yamls
	cmdClearYaml := fmt.Sprintf("rm -f %v/%v/templates/*.yaml", chartsDir, chartName)
	log.Println(cmdClearYaml)
	outClearYaml, errClearYaml := exec.Command("bash", "-c", cmdClearYaml).Output()
	if errClearYaml != nil {
		log.Printf("Failed to execute command: %s", cmdClearYaml)
		log.Printf("Error: %v", errClearYaml)
		return ""
	}
	log.Println(string(outClearYaml))

	// Clear out templates yamls
	cmdClearValues := fmt.Sprintf("echo -n \"\" > %v/%v/values.yaml", chartsDir, chartName)
	log.Println(cmdClearValues)
	outClearValues, errClearValues := exec.Command("bash", "-c", cmdClearValues).Output()
	if errClearValues != nil {
		log.Printf("Failed to execute command: %s", cmdClearValues)
		log.Printf("Error: %v", errClearValues)
		return ""
	}
	log.Println(string(outClearValues))

	// Copy new yaml to templates
	cmdCopyYaml := fmt.Sprintf("cp -a %v %v/%v/templates", yamlFile, chartsDir, chartName)
	log.Println(cmdCopyYaml)
	outCopyYaml, errCopyYaml := exec.Command("bash", "-c", cmdCopyYaml).Output()
	if errCopyYaml != nil {
		log.Printf("Failed to execute command: %s", cmdCopyYaml)
		log.Printf("Error: %v", errCopyYaml)
		return ""
	}
	log.Println(string(outCopyYaml))

	pathToChart := fmt.Sprintf("%v/%v", chartsDir, chartName)
	return pathToChart

}
