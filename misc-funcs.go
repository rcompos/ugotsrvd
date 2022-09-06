package ugotsrvd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/gin-gonic/gin"
)

// func IndexHandler(c *gin.Context) {
// 	// Handle /index
// 	c.HTML(200, "index.html", nil)
// }

func IndexHandler(c *gin.Context) {
	// var values []int
	// for i := 0; i < 5; i++ {
	// 	values = append(values, i)
	// }
	c.HTML(http.StatusOK, "index.tmpl", gin.H{"msg": "Welcome to ugotsrvd."})
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

	// // Delete source directory
	// cmdDelete := fmt.Sprintf("rm -fr %v", sourceDir)
	// log.Println(cmdDelete)
	// outDelete, errDelete := exec.Command("bash", "-c", cmdDelete).Output()
	// if errDelete != nil {
	// 	log.Printf("Failed to execute command: %s", cmdDelete)
	// 	log.Printf("Error: %v", errDelete)
	// }
	// log.Println(string(outDelete))
}

func createCfgDir(p string) {
	// p path to file
	if _, err := os.Stat(p); os.IsNotExist(err) {
		os.MkdirAll(p, 0777) // Create directory
	}
}

func fileExists(f string) bool {
	if _, err := os.Stat(f); err == nil { // path/to/whatever exists
		return true
	} else if errors.Is(err, os.ErrNotExist) { // path/to/whatever does *not* exist
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

func cloneOrPullRepo(url, directory, gitUsername, token string) {
	if !dirExists(directory) {
		// Repo dir not exist. Need to git clone.
		gitClone(url, directory, gitUsername, token)
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

func GetArray(c *gin.Context) { // For DEVTEST
	var values []int
	for i := 0; i < 5; i++ {
		values = append(values, i)
	}
	c.HTML(http.StatusOK, "array.tmpl", gin.H{"values": values})
} // For DEVTEST

func ListFiles(c *gin.Context) {
	var values []int
	for i := 0; i < 5; i++ {
		values = append(values, i)
	}
	fileList := filesInDir(uploadDir)
	c.HTML(http.StatusOK, "listfiles.tmpl", gin.H{"values": fileList})
}
