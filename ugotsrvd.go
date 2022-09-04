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

	"github.com/gin-gonic/gin"
)

const uploadDir = "upload"
const chartsBaseDir = "generated/charts"
const appsBaseDir = "generated/apps"
const repoBaseDir = "repos"

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

	filename := uploadDir + "/" + file
	gitRepo := "autocharts"
	gitUrl := "https://github.com/rcompos/" + gitRepo
	username := "rcompos"
	token := "c82f1f163e5884760dbc8d7456740b5083952b25"

	if !fileExists(filename) { // file not exists is bad
		c.String(http.StatusOK, "File %s not found!", filename)
		return
	}

	chartnameTmp := fmt.Sprintf("%v-%v", environment, file)
	extension := path.Ext(chartnameTmp)
	chartname := chartnameTmp[0 : len(chartnameTmp)-len(extension)]

	// Create Helm chart
	pathToChart := createHelmChart(chartname, filename, chartsBaseDir)

	// Copy created Helm chart to repoDir
	repoDir := fmt.Sprintf("%v/%v", repoBaseDir, gitRepo)

	cloneOrPullRepo(gitUrl, repoDir, username, token)

	// Check if chart already exists
	chartDir := fmt.Sprintf("%v/%v", repoDir, chartname)
	if fileExists(chartDir) {
		log.Println("Chart already exists!", chartDir)
		c.String(http.StatusOK, "Chart already exists! %s", chartDir)
		return
	}
	copyChartToRepo(pathToChart, repoDir)

	// Create ArgoCD application
	// createArgoCDApp()

	message := "Add new Helm Chart." + chartname
	gitCommit(repoDir, message, chartname)
	gitPush(repoDir, username, token)
	c.String(http.StatusOK, "Helm chart pushed! %s", chartDir)
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

// func createArgoCDApplication() {
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

func copyChartToRepo(chartDirectory, repoDir string) {
	// Copy Helm chart to chart git repo
	cmdCopyYaml := fmt.Sprintf("cp -a %v %v", chartDirectory, repoDir)
	log.Println(cmdCopyYaml)
	outCopyYaml, errCopyYaml := exec.Command("bash", "-c", cmdCopyYaml).Output()
	if errCopyYaml != nil {
		log.Printf("Failed to execute command: %s", cmdCopyYaml)
		log.Printf("Error: %v", errCopyYaml)
	}
	log.Println(string(outCopyYaml))

	// Delete Helm chart directory
	cmdDeleteChart := fmt.Sprintf("rm -fr %v", chartDirectory)
	log.Println(cmdDeleteChart)
	outDeleteChart, errDeleteChart := exec.Command("bash", "-c", cmdDeleteChart).Output()
	if errDeleteChart != nil {
		log.Printf("Failed to execute command: %s", cmdDeleteChart)
		log.Printf("Error: %v", errDeleteChart)
	}
	log.Println(string(outDeleteChart))
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
