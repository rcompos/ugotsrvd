package ugotsrvd

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gin-gonic/gin"
)

// const uploadDir = "/Users/composr/work/ugotsrvd-data/upload"
// const chartsBaseDir = "/Users/composr/work/ugotsrvd-data/generated/charts"
// const appsBaseDir = "/Users/composr/work/ugotsrvd-data/generated/apps"
// const repoBaseDir = "/Users/composr/work/ugotsrvd-data/repos"

const uploadDir = "/Users/roncompos/work/ugotsrvd-data/upload"
const chartsBaseDir = "/Users/roncompos/work/ugotsrvd-data/generated/charts"
const appsBaseDir = "/Users/roncompos/work/ugotsrvd-data/generated/apps"
const repoBaseDir = "/Users/roncompos/work/ugotsrvd-data/repos"

const gitRepo = "autocharts"
const gitAccount = "https://github.com/rcompos"
const gitUsername = "rcompos"
const revision = "HEAD"
const argoCDurl = "https://argocd.example.com"

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

func GetUpload(c *gin.Context) {
	var values []int
	for i := 0; i < 5; i++ {
		values = append(values, i)
	}
	c.HTML(http.StatusOK, "upload.tmpl", gin.H{"msg": "Welcome to ugotsrvd."})
}

// TODO: Change to multiple file upload
// https://github.com/gin-gonic/examples/tree/master/upload-file/multiple
func PostUpload(c *gin.Context) {
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

	appOfAppName := "proj-workload-clusters"
	deployEnv := "ksa-poc"

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
	pathToChart := createHelmChart(chartname, filename, chartsBaseDir, deployEnv)
	log.Println("pathToChart:", pathToChart)

	// Check if chart already exists
	chartDir := fmt.Sprintf("%v/%v", repoDir, chartname)
	if fileExists(chartDir) {
		log.Println("Chart already exists!", chartDir)
		c.String(http.StatusOK, "Chart already exists! %s", chartDir)
		return
	}
	copyToRepo(pathToChart, repoDir)

	// ArgoCD Helm Chart
	// Create ArgoCD application yaml from template
	appChartName := chartname + "-app"
	templateFile := "argocd-templates/argocd-application.yaml"
	pathToApp := CreateArgoCDApp(appChartName, chartname, templateFile, appsBaseDir)
	log.Println("pathToApp:", pathToApp)

	// Create Helm chart for ArgoCD application
	pathToAppChart := createHelmChart(appChartName, pathToApp, appsBaseDir, deployEnv)
	log.Println("pathToAppChart:", pathToAppChart)

	// Check if chart already exists
	appChartDir := fmt.Sprintf("%v/%v", repoDir, appChartName)
	if fileExists(appChartDir) {
		log.Println("Chart already exists!", appChartDir)
		c.String(http.StatusOK, "Chart already exists! %s", appChartDir)
		return
	}
	copyToRepo(pathToAppChart, repoDir)

	successfulAdd := checkAppInArgoCDAppOfApps(chartname, appOfAppName, repoDir, deployEnv)
	if successfulAdd == false {
		c.String(http.StatusOK, "ERROR: Existing ArgoCD application exists: %s", chartname)
		return
	}

	// Add to ArgoCD app-of-apps
	appOfAppValuesFile := fmt.Sprintf("%v/%v/env/%v/values.yaml", repoDir, appOfAppName, deployEnv)
	appOfAppValuesFileShort := fmt.Sprintf("%v/env/%v/values.yaml", appOfAppName, deployEnv)
	addAppToArgoCDAppOfApps(chartname, appOfAppName, repoDir, appOfAppValuesFile, deployEnv)

	filesToAdd := []string{chartname, appChartName, appOfAppValuesFileShort}
	messageAppChart := "Add new ArgoCD app." + appChartName
	gitCommit(repoDir, messageAppChart, filesToAdd)

	// Git push workload cluster Helm chart
	gitCommitSHA := gitPush(repoDir, gitUsername, token, revision)
	// c.String(http.StatusOK, "CAPI Workload Cluster Helm and ArgoCD app charts pushed!\n%s\nGit commit: %v/commit/%v\nArgoCD: %v", chartDir, gitUrl, gitCommitSHA, argoCDUrl)
	// c.String(http.StatusOK, "CAPI Workload Cluster Helm and ArgoCD app charts pushed!\n\nGit commit: %v/commit/%v\nArgoCD: %v", gitUrl, gitCommitSHA, argoCDUrl)
	gitCommitUrl := fmt.Sprintf("%v/commit/%v", gitUrl, gitCommitSHA)
	c.HTML(http.StatusOK, "result.tmpl", gin.H{"giturl": gitCommitUrl, "argoCDurl": argoCDurl})
}

func addAppToArgoCDAppOfApps(chartname, appOfAppName, repoDir, appOfAppValuesFile, deployEnv string) {
	// TODO: Need to refactor for safer addition of yaml
	appsNameString := strings.Replace(chartname, "-", "", -1)

	nameString := fmt.Sprintf("name: %s\n", chartname)
	fullnameOverrideString := "fullnameOverride: \"\"\n"
	valuesString := "values: \"\"\n"
	namespaceString := "namespace: \"\"\n"
	repoString := "repo: \"\"\n"
	pathString := "path: \"\"\n"
	targetRevString := "targetRev: \"\"\n"

	newAppString := "\n  " + appsNameString + ":\n" +
		"    " + nameString +
		"    " + fullnameOverrideString +
		"    " + valuesString +
		"    " + namespaceString +
		"    " + repoString +
		"    " + pathString +
		"    " + targetRevString

	log.Println(newAppString)

	// TODO: Update Chart.yaml with new version

	log.Println("appOfAppsValuesFile: ", appOfAppValuesFile)

	f, err := os.OpenFile(appOfAppValuesFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()
	if _, err := f.WriteString(newAppString); err != nil {
		log.Println(err)
	}
}

func checkAppInArgoCDAppOfApps(chartname, appOfAppName, repoDir, deployEnv string) bool {
	// Add to existing ArgoCD App-of-Apps
	// look at file repoDir/proj-workload-clusters/env/ksa-poc/values.yaml
	appOfAppValuesFile := fmt.Sprintf("%v/%v/env/%v/values.yaml", repoDir, appOfAppName, deployEnv)
	if !fileExists(appOfAppValuesFile) {
		log.Println("App-of-app values file not found!", appOfAppValuesFile)
		// c.String(http.StatusOK, "App-of-app values file not found! %s", appOfAppValuesFile)
		return false
	}

	cmdCatValues := fmt.Sprintf("cat %v", appOfAppValuesFile)
	log.Println(cmdCatValues)
	outCatValues, errCatValues := exec.Command("bash", "-c", cmdCatValues).Output()
	if errCatValues != nil {
		log.Printf("Failed to execute command: %s", cmdCatValues)
		log.Printf("Error: %v", errCatValues)
		return false
	}
	log.Println(string(outCatValues))

	// Check for existence of apps.MYAPPNAME
	// cat ~/work/ugotsrvd-work/autocharts/proj-workload-clusters/env/ksa-poc/values.yaml | yq .apps.dataengDev0Cornholio2022aws.name
	cmdExistingApps := fmt.Sprintf("cat %v |  yq '.apps.[].name'", appOfAppValuesFile)
	log.Println(cmdExistingApps)
	tmpExistingApps, errExistingApps := exec.Command("bash", "-c", cmdExistingApps).Output()
	outExistingApps := string(tmpExistingApps)

	if errExistingApps != nil {
		log.Printf("Failed to execute command: %s", cmdExistingApps)
		log.Printf("Error: %v", errExistingApps)
		return false
	}
	log.Printf("Existing applications:\n%v\n", string(outExistingApps))

	for _, v := range strings.Split(outExistingApps, "\n") {
		log.Printf("Existing app in app-of-apps %v: %v\n", appOfAppName, v)
		if string(v) == chartname {
			log.Println("ArgoCD app-of-apps:", appOfAppName)
			log.Println("ERROR: Existing ArgoCD application exists:", chartname)
			// Return without creating new app
			// c.String(http.StatusOK, "Existing app in app-of-apps %v! %v", appOfAppName, chartname)
			return false
		}
	}

	// Add block to app-of-app values.yaml for new Helm chart
	//
	//
	// // Add to app-of-apps

	return true

}

type ArgoCDApp struct {
	Appname              string
	HelmChart            string
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

// TODO: Instantiate the ArgoCD app-of-apps helm chart
// func CreateArgoCDAppOfApps(appname, helmchart, templateFile, appsBaseDir string) string {
// }

func CreateArgoCDApp(appname, helmchart, templateFile, appsBaseDir string) string {
	// TODO: Add more template params
	log.Println("appname:", appname)
	log.Println("helmchart:", helmchart)
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
		HelmChart:      helmchart,
		Project:        "default",
		RepoURL:        "https://github.com/rcompos/autocharts.git",
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

func createHelmChart(chartName, yamlFile, chartsDir, deployEnv string) string {
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

	// Create env-specific values file
	cmdEnvValues := fmt.Sprintf("echo -n \"\" > %v/%v/values-%v.yaml", chartsDir, chartName, deployEnv)
	log.Println(cmdEnvValues)
	outEnvValues, errEnvValues := exec.Command("bash", "-c", cmdEnvValues).Output()
	if errEnvValues != nil {
		log.Printf("Failed to execute command: %s", cmdEnvValues)
		log.Printf("Error: %v", errEnvValues)
		return ""
	}
	log.Println(string(outEnvValues))

	// Clear out test templates
	cmdClearTests := fmt.Sprintf("cd %v/%v; rm -fr ./templates/tests ./templates/NOTES.txt", chartsDir, chartName)
	log.Println(cmdClearTests)
	outClearTests, errClearTests := exec.Command("bash", "-c", cmdClearTests).Output()
	if errClearTests != nil {
		log.Printf("Failed to execute command: %s", cmdClearTests)
		log.Printf("Error: %v", errClearTests)
		return ""
	}
	log.Println(string(outClearTests))

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
