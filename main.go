package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

const (
	CONTAINER_NAME = "archive"
	SOURCE_ACCOUNT = "zoomrecordingarchive"
	DIST_ACCOUNT   = "newzoomrecordingarchive"
)

func handleErrors(err error) {
	if err != nil {
		if serr, ok := err.(azblob.StorageError); ok { // This error is a Service-specific
			switch serr.ServiceCode() { // Compare serviceCode to ServiceCodeXxx constants
			case azblob.ServiceCodeContainerAlreadyExists:
				fmt.Println("Received 409. Container already exists")
				return
			}
		}
		log.Fatal(err)
	}
}

func getContainer(accountName, accountKey, container string) (containerURL azblob.ContainerURL, sasToken string, err error) {
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	handleErrors(err)
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})
	// From the Azure portal, get your storage account blob service URL endpoint.
	URL, _ := url.Parse(
		fmt.Sprintf("https://%s.blob.core.windows.net/%s", accountName, CONTAINER_NAME))

	// Create a ContainerURL object that wraps the container URL and a request
	// pipeline to make requests.
	containerURL = azblob.NewContainerURL(*URL, p)
	sasQueryParams, err := azblob.AccountSASSignatureValues{
		Protocol:      azblob.SASProtocolHTTPSandHTTP,       // Users MUST use HTTPS (not HTTP)
		ExpiryTime:    time.Now().UTC().Add(48 * time.Hour), // 48-hours before expiration
		Permissions:   azblob.AccountSASPermissions{Read: true, Add: true, Create: true, Write: true, List: true}.String(),
		Services:      azblob.AccountSASServices{Blob: true}.String(),
		ResourceTypes: azblob.AccountSASResourceTypes{Container: true, Object: true}.String(),
	}.NewSASQueryParameters(credential)
	handleErrors(err)

	sasToken = sasQueryParams.Encode()
	return
}

func listBlobs(containerURL azblob.ContainerURL, prefix string, folder bool) ([]string, error) {
	ctx := context.Background()
	list := []string{}
	// List the container that we have created above
	for marker := (azblob.Marker{}); marker.NotDone(); {
		// Get a result segment starting with the blob indicated by the current Marker.
		listBlob, err := containerURL.ListBlobsHierarchySegment(ctx, marker, "/", azblob.ListBlobsSegmentOptions{
			Prefix:     prefix,
			MaxResults: 100,
		})
		handleErrors(err)
		// ListBlobs returns the start of the next segment; you MUST use this to get
		// the next segment (after processing the current result segment).
		marker = listBlob.NextMarker
		for _, blobInfo := range listBlob.Segment.BlobPrefixes {
			list = append(list, blobInfo.Name)
		}
	}
	return list, nil
}

// return what not in a of b
// so typically a is more, it's a super collection which contains b
func diff(a, b []string) []string {
	arr := []string{}
	keys := make(map[string]string, len(b))
	for _, k := range b {
		keys[k] = k
	}
	for _, s := range a {
		if _, ok := keys[s]; ok {
			continue
		}
		arr = append(arr, s)
	}
	return arr
}

// ./azcopy copy "https://zoomrecordingarchive.blob.core.windows.net/archive/190054/Y2VkOWRlYTEtNDljNC00MjJmLThkYjEtODY5YWI3YzVkODZk.MP4" "https://newzoomrecordingarchive.blob.core.windows.net/archive/Y2VkOWRlYTEtNDljNC00MjJmLThkYjEtODY5YWI3YzVkODZk.MP4" --s2s-preserve-access-tier=false --include-directory-stub=false --recursive;

func copy(blob, srcSas, distSas string) string {
	fmt.Printf("copy job: %s is started\n", blob)
	// copy folder by default
	command := "./azcopy copy \"https://zoomrecordingarchive.blob.core.windows.net/archive/" + blob + "?" + srcSas + "\" \"https://newzoomrecordingarchive.blob.core.windows.net/archive/?" + distSas + "\" --recursive"
	if !strings.HasSuffix(blob, "/") {
		// copy specific blob
		command = "./azcopy copy \"https://zoomrecordingarchive.blob.core.windows.net/archive/" + blob + "?" + srcSas + "\" \"https://newzoomrecordingarchive.blob.core.windows.net/archive/" + blob + "?" + distSas + "\" --recursive --s2s-preserve-access-tier=false --include-directory-stub=false"
	}
	output, err := runCommand(command)
	if err != nil {
		fmt.Println(command)
		fmt.Println("copy job:", blob, "failed!", err.Error())
		return ""
	}
	fmt.Printf("copy job: %s is completed\n", blob)
	return output
}

func runCommand(command string) (output string, err error) {
	cmd := exec.Command("sh", "-c", command)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	output = stdout.String()
	return
}

func main() {
	// From the Azure portal, get your storage account name and key and set environment variables.
	SOURCE_KEY, DIST_KEY := os.Getenv("AZURE_SOURCE_KEY"), os.Getenv("AZURE_DIST_KEY")
	if len(SOURCE_KEY) == 0 || len(DIST_KEY) == 0 {
		log.Fatal("Either the AZURE_SOURCE_KEY or AZURE_DIST_KEY environment variable is not set")
	}
	workerNumPtr := flag.Int("worker", 5, "number of workers")
	totalPtr := flag.Int("total", 10000, "total of job")
	maxPtr := flag.Int("max", 480000, "max blob id")
	minPtr := flag.Int("min", 100, "min blob id")
	folderPtr := flag.Bool("folder", true, "only search folder")
	flag.Parse()

	workerSize := *workerNumPtr
	max := *maxPtr
	min := *minPtr
	total := *totalPtr
	folder := *folderPtr

	// define a 10 sized queue
	queue := make(chan string, 10)

	finishSignal := make(chan bool)
	var workerFinishCounter int32
	// max id
	counter := 0

	// get container clients
	srcContainer, srcSas, _ := getContainer(SOURCE_ACCOUNT, SOURCE_KEY, CONTAINER_NAME)
	distContainer, distSas, _ := getContainer(DIST_ACCOUNT, DIST_KEY, CONTAINER_NAME)

	// define process for run copy command
	for i := 0; i < workerSize; i++ {
		go func() {
			for blobName := range queue {
				copy(blobName, srcSas, distSas)
			}
			// maybe occur an bug, multiple routine can access this variable
			// so, typically use the atomic to fix that
			atomic.AddInt32(&workerFinishCounter, 1)
			if workerFinishCounter >= int32(workerSize) {
				finishSignal <- true
			}
		}()
	}

	// fetch blob from 480000 ~ 100
	for i := max; i >= min; i -= 100 {
		prefix := fmt.Sprintf("%d", i/100)

		srcBlobs, srcErr := listBlobs(srcContainer, prefix, folder)
		handleErrors(srcErr)

		distBlobs, distErr := listBlobs(distContainer, prefix, folder)
		handleErrors(distErr)

		// diff container
		diffBlobs := diff(srcBlobs, distBlobs)

		// push into queue
		for _, blobName := range diffBlobs {
			counter++
			queue <- blobName
			if counter >= total {
				goto FINISH
			}
		}
	}
FINISH:
	close(queue)
	// wait for shutdown signal
	<-finishSignal
	fmt.Println("finish jobs")
}
