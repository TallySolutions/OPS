package main
import ("testing"
        "os"
        "fmt"
        "github.com/spf13/cobra"
        "bytes"
        "net/http"
        "io/ioutil"
        "time"
)

func TestDownloadImages(t *testing.T) {
    // remove the files to force a download
    // ignore any error from remove
    os.Remove("mkfs")
    os.Remove("staging/boot")
    os.Remove("staging/stage3")
    downloadImages()

    if _, err := os.Stat("staging/boot"); os.IsNotExist(err) {
        t.Errorf("staging/boot file not found")
    }
    
    if info, err := os.Stat("mkfs"); os.IsNotExist(err) {
        t.Errorf("mkfs not found")
    } else {
        mode := fmt.Sprintf("%04o", info.Mode().Perm())
        if mode != "0775"{
            t.Errorf("mkfs not executable")
        }
    }
    
    if _, err := os.Stat("staging/stage3"); os.IsNotExist(err) {
        t.Errorf("staging/stage3 file not found")
    }
}

func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	_, output, err = executeCommandC(root, args...)
	return output, err
}

func executeCommandC(root *cobra.Command, args ...string) (c *cobra.Command, output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOutput(buf)
    root.SetArgs(args)
    fmt.Println(args)
	c, err = root.ExecuteC()
	return c, buf.String(), err
}


// TODO 
func TestStartHypervisor(t *testing.T) {
    downloadImages()
    buildImage([]string {"./data/webs"})
    var hypervisor Hypervisor
    go func() {  
        hypervisor = hypervisors["qemu-system-x86_64"]()
        hypervisor.start(finalImg, 8080)
        }()
    time.Sleep(3 * time.Second)
    resp, err := http.Get("http://127.0.0.1:8080")
    if err != nil {
        t.Log(err)
        t.Errorf("failed to get 127.0.0.1:8080")
    }
    defer resp.Body.Close()
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        t.Log(err)
        t.Errorf("ReadAll failed")
    }

    if string(body) != "unibooty!" {
        t.Errorf("unexpected response" + string(body))
    }
    hypervisor.stop()
}