package ugotsrvd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"text/template"

	"github.com/gin-gonic/gin"
)

// const basedDir = "/Users/composr/work/ugotsrvd-data/"

// const uploadDir = "upload"
// const chartsBaseDir = "generated/charts"
// const appsBaseDir = "generated/apps"
// const repoBaseDir = "repos"

const uploadDir = "/Users/composr/work/ugotsrvd-data/upload"
const chartsBaseDir = "/Users/composr/work/ugotsrvd-data/generated/charts"
const appsBaseDir = "/Users/composr/work/ugotsrvd-data/generated/apps"
const repoBaseDir = "/Users/composr/work/ugotsrvd-data/repos"

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

func IndexHandler(c *gin.Context) {
	// Handle /index
	c.HTML(200, "index.html", nil)
}

func writeToFile(f string, d []byte) {
	// d1 := []byte("hello\ngo\n")
	// err := os.WriteFile("./tmp/dat1", d, 0644)
	err := os.Remove(f)
	check(err)
	err2 := os.WriteFile(f, d, 0644)
	check(err2)
}

func check(e error) {
	if e != nil {
		log.Println(e)
	}
}

func LogEnvVars() {
	cmd := exec.Command("env")
	stdout0, err := cmd.Output()
	if err != nil {
		log.Println(err.Error())
	}
	log.Println(string(stdout0))
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

	// ################################################
	// // TODO: Need to move variable definitions
	filename := uploadDir + "/" + file
	gitRepo := "autocharts"
	gitUrl := "https://github.com/rcompos/" + gitRepo
	username := "rcompos"
	token := "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"
	// ################################################

	if !fileExists(filename) { // file not exists is bad
		c.String(http.StatusOK, "File %s not found!", filename)
		return
	}

	chartnameTmp := fmt.Sprintf("%v-%v", environment, file)
	extension := path.Ext(chartnameTmp)
	chartname := chartnameTmp[0 : len(chartnameTmp)-len(extension)]

	repoDir := fmt.Sprintf("%v/%v", repoBaseDir, gitRepo)
	cloneOrPullRepo(gitUrl, repoDir, username, token)

	// Cluster-API Cluster Helm Chart
	// Create Helm chart
	pathToChart := createHelmChart(chartname, filename, chartsBaseDir)

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
	templateFile := "argocd-templates/argocd-application.tmpl"
	pathToApp := CreateArgoCDApp(appChartName, templateFile, appsBaseDir)
	log.Println("pathToApp:", pathToApp)

	pathToAppChart := createHelmChart(appChartName, pathToApp, appsBaseDir)

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
	gitPush(repoDir, username, token)
	c.String(http.StatusOK, "CAPI Workload Cluster Helm and ArgoCD app charts pushed! %s", chartDir)
}

type ArgoCDApp struct {
	Appname        string
	Project        string
	RepoURL        string
	TargetRevision string
	Path           string
}

func CreateArgoCDApp(appname, templateFile, appsBaseDir string) string {
	log.Println("appname:", appname)
	log.Println("templateFile:", templateFile)
	log.Println("appsBaseDir:", appsBaseDir)

	tmp, err := template.ParseFiles("argocd-templates/argocd-application.yaml")
	if err != nil {
		log.Fatal(err)
	}

	argoCDAppFile := appsBaseDir + "/" + appname
	log.Println("argoCDAppFile:", argoCDAppFile)
	f, err := os.Create(argoCDAppFile)
	defer f.Close()

	if err != nil {
		log.Println("create file: ", err)
		return ""
	}

	data := ArgoCDApp{
		Appname:        appname,
		Project:        "defaultus",
		RepoURL:        "https://github.com/rcompos/autocharts",
		TargetRevision: "HEAD",
		Path:           appname,
	}
	err = tmp.Execute(f, data)
	if err != nil {
		log.Print("execute: ", err)
		return ""
	}

	return argoCDAppFile

}

// func copyArgoCDAppToRepo() {
// }

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
	}
	log.Println(string(out))

	// Clear out templates yamls
	cmdClearYaml := fmt.Sprintf("rm -f %v/%v/templates/*.yaml", chartsDir, chartName)
	log.Println(cmdClearYaml)
	outClearYaml, errClearYaml := exec.Command("bash", "-c", cmdClearYaml).Output()
	if errClearYaml != nil {
		log.Printf("Failed to execute command: %s", cmdClearYaml)
		log.Printf("Error: %v", errClearYaml)
	}
	log.Println(string(outClearYaml))

	// Clear out templates yamls
	cmdClearValues := fmt.Sprintf("echo -n \"\" > %v/%v/values.yaml", chartsDir, chartName)
	log.Println(cmdClearValues)
	outClearValues, errClearValues := exec.Command("bash", "-c", cmdClearValues).Output()
	if errClearValues != nil {
		log.Printf("Failed to execute command: %s", cmdClearValues)
		log.Printf("Error: %v", errClearValues)
	}
	log.Println(string(outClearValues))

	// Copy new yaml to templates
	cmdCopyYaml := fmt.Sprintf("cp -a %v %v/%v/templates", yamlFile, chartsDir, chartName)
	log.Println(cmdCopyYaml)
	outCopyYaml, errCopyYaml := exec.Command("bash", "-c", cmdCopyYaml).Output()
	if errCopyYaml != nil {
		log.Printf("Failed to execute command: %s", cmdCopyYaml)
		log.Printf("Error: %v", errCopyYaml)
	}
	log.Println(string(outCopyYaml))

	pathToChart := fmt.Sprintf("%v/%v", chartsDir, chartName)
	return pathToChart

}

func copyToRepo(sourceDir, repoDir string) {
	// Copy directory to git repo
	cmdCopy := fmt.Sprintf("cp -a %v %v", sourceDir, repoDir)
	log.Println(cmdCopy)
	outCopy, errCopy := exec.Command("bash", "-c", cmdCopy).Output()
	if errCopy != nil {
		log.Printf("Failed to execute command: %s", cmdCopy)
		log.Printf("Error: %v", errCopy)
	}
	log.Println(string(outCopy))

	// Delete source directory
	cmdDelete := fmt.Sprintf("rm -fr %v", sourceDir)
	log.Println(cmdDelete)
	outDelete, errDelete := exec.Command("bash", "-c", cmdDelete).Output()
	if errDelete != nil {
		log.Printf("Failed to execute command: %s", cmdDelete)
		log.Printf("Error: %v", errDelete)
	}
	log.Println(string(outDelete))
}

func createCfgDir(p string) {
	// p path to file
	if _, err := os.Stat(p); os.IsNotExist(err) {
		os.MkdirAll(p, 0777) // Create directory
	}
}

func fileExists(f string) bool {
	if _, err := os.Stat(f); err == nil {
		// path/to/whatever exists
		return true
	} else if errors.Is(err, os.ErrNotExist) {
		// path/to/whatever does *not* exist
		return false
	}
	return false
}

func dirExists(dirname string) bool {
	folderInfo, err := os.Stat(dirname)
	if os.IsNotExist(err) {
		log.Println("Folder does not exist.")
		return false
	}
	log.Println(folderInfo)
	return true
}

func cloneOrPullRepo(url, directory, username, token string) {
	if !dirExists(directory) {
		// Repo dir not exist. Need to git clone.
		gitClone(url, directory, username, token)
		log.Println("Git clone url: " + url)
		log.Println("Git directory: " + directory)
	} else {
		// Repo dir exists. Git pull.
		gitPull(directory)
	}
}

func filesInDir(dir string) []string {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	listOfFiles := []string{}
	for _, file := range files {
		log.Println(file.Name(), file.IsDir())
		if !file.IsDir() {
			listOfFiles = append(listOfFiles, file.Name())
		}
	}
	return listOfFiles
}

// For DEVTEST
func GetArray(c *gin.Context) {
	var values []int
	for i := 0; i < 5; i++ {
		values = append(values, i)
	}
	c.HTML(http.StatusOK, "array.tmpl", gin.H{"values": values})
}

func ListFiles(c *gin.Context) {
	var values []int
	for i := 0; i < 5; i++ {
		values = append(values, i)
	}
	fileList := filesInDir(uploadDir)
	c.HTML(http.StatusOK, "listfiles.tmpl", gin.H{"values": fileList})
}
