package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/html"
)

func RecurseStatus(argList []string) (bool, error) {
	recurse := false
	found := false
	for i := 0; i < len(argList); i++ {
		if argList[i] == "-r" {
			if found {
				return false, errors.New("error: -r flag specified multiple time")
			}
			found = true
			recurse = true
		}
	}
	return recurse, nil
}

func getRecurseLevel(argList []string) (int, error) {
	recurseLevel := 0
	found := false
	isRecurse, err := RecurseStatus(argList)
	if err != nil {
		return 0, err
	}
	if isRecurse {
		recurseLevel = 5
	}
	for i := 0; i < len(argList); i++ {
		if argList[i] == "-l" {
			if found {
				return 0, errors.New("error: -l flag specified multiple time")
			} else if !isRecurse {
				return 0, errors.New("error: -l flag without associated -r flag")
			}
			found = true

			if i+1 < len(argList) {
				level, err := strconv.Atoi(argList[i+1])
				if err != nil || level < 1 || level > 10 {
					return 0, fmt.Errorf("error: invlid value for -l flag '%s'", argList[i+1])
				}
				recurseLevel = level
			} else {
				return 0, errors.New("error: missing value for -l flag")
			}
		}
	}
	return recurseLevel, nil
}

func getOutputFolder(argList []string) (string, error) {
	outputFolder := "./data/"
	found := false
	for i := 0; i < len(argList); i++ {
		if argList[i] == "-p" {
			if found {
				return "", errors.New("error: -p flag specified multiple time")
			}
			found = true

			if i+1 < len(argList) {
				outputFolder = argList[i+1]
			} else {
				return "", errors.New("error: missing value for -p flag")
			}
		}
	}
	if info, err := os.Stat(outputFolder); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(outputFolder, 0755); err != nil {
				return "", fmt.Errorf("error: could not create directory '%s'", outputFolder)
			}
		} else {
			return "", fmt.Errorf("error: could not access '%s', %v", outputFolder, err)
		}
	} else if !info.IsDir() {
		return "", fmt.Errorf("error: '%s' exists but is not a directory", outputFolder)
	} else {
		mode := info.Mode()
		if mode&0200 == 0 {
			return "", fmt.Errorf("error: no write permission for directory '%s'", outputFolder)
		}
	}
	return outputFolder, nil
}

func IsFlag(arg string) bool {
	if arg == "-l" || arg == "-r" || arg == "-p" || arg == "-v" {
		return true
	}
	return false
}

func getURL(argList []string) (string, error) {
	url := ""
	for i := 0; i < len(argList); i++ {
		if IsFlag(argList[i]) {
			if argList[i] == "-l" || argList[i] == "-p" {
				i++
			}
			continue
		}
		if url == "" {
			url = argList[i]
		} else {
			return "", errors.New("error: too many URLs")
		}
	}
	if url == "" {
		return url, errors.New("error: missing URL")
	}
	return url, nil
}

func ErrorExit(err error) {
	fmt.Println(err)
	os.Exit(1)
}

func isVerbose(argList []string) bool {
	for i := 0; i < len(argList); i++ {
		if argList[i] == "-v" {
			return true
		}
	}
	return false
}

func parser(argList []string) (string, int, string, bool, error) {
	recurseLevel, err := getRecurseLevel(argList)
	if err != nil {
		return "", 0, "", false, err
	}
	url, err := getURL(argList)
	if err != nil {
		return "", 0, "", false, err
	}
	outputFolder, err := getOutputFolder(argList)
	if err != nil {
		return "", 0, "", false, err
	}
	verbose := isVerbose(argList)
	return url, recurseLevel, outputFolder, verbose, nil
}

func hasImageSuffix(link string) bool {
	if strings.HasSuffix(link, ".jpg") || strings.HasSuffix(link, ".jpeg") ||
		strings.HasSuffix(link, ".png") || strings.HasSuffix(link, ".bmp") ||
		strings.HasSuffix(link, ".gif") {
		return true
	}
	return false
}

func downloadImages(pageURL string, outputFolder string) {
	resp, err := http.Get(pageURL)
	if err != nil {
		fmt.Println("Error fetching page:", err)
		return
	}
	defer resp.Body.Close()

	links := extractLinks(resp.Body, pageURL)
	for _, link := range links {
		if hasImageSuffix(link) {
			downloadFile(link, outputFolder)
		}
	}
}

func extractLinks(body io.Reader, baseURL string) []string {
	var links []string
	tokenizer := html.NewTokenizer(body)
	base, _ := url.Parse(baseURL)

	for {
		token := tokenizer.Next()
		switch token {
		case html.ErrorToken:
			return links
		case html.StartTagToken:
			tagName, _ := tokenizer.TagName()
			if string(tagName) == "a" || string(tagName) == "img" {
				for {
					attrName, attrVal, moreAttr := tokenizer.TagAttr()
					if string(attrName) == "href" || string(attrName) == "src" {
						link, err := url.Parse(string(attrVal))
						if err == nil {
							absoluteURL := base.ResolveReference(link).String()
							links = append(links, absoluteURL)
						}
					}
					if !moreAttr {
						break
					}
				}
			}
		}
	}
}

func downloadFile(fileURL string, outputFolder string) {
	resp, err := http.Get(fileURL)
	if err != nil {
		fmt.Println("Error downloading file:", err)
		return
	}
	defer resp.Body.Close()

	fileName := path.Base(fileURL)
	filePath := path.Join(outputFolder, fileName)

	out, err := os.Create(filePath)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Println("Error saving file:", err)
	}
}

func processDownload(jobId int,
	activeJob struct {
		url   string
		depth int
	}, outputMutex *sync.Mutex, outputFolder string, activeJobs *int32,
) {
	defer atomic.AddInt32(activeJobs, -1)
	resp, err := http.Get(activeJob.url)
	if err != nil {
		fmt.Println("Error fetching page:", err)
		return
	}
	defer resp.Body.Close()
	outputMutex.Lock()
	fmt.Printf("Downloading images from '%s'\n", activeJob.url)
	outputMutex.Unlock()
	downloadImages(activeJob.url, outputFolder)
}

func crawl(startURL string, recurseLevel int, outputFolder string, verbose bool) {
	visited := make(map[string]bool)
	var wg sync.WaitGroup
	mutex := &sync.Mutex{}
	outputMutex := &sync.Mutex{}

	startDomain, _ := url.Parse(startURL)
	queue := make(chan struct {
		url   string
		depth int
	}, 500)
	jobs := make(chan struct {
		url   string
		depth int
	}, 100)

	var activeWorkers int32 = 0
	var activeJobs int32 = 0
	numWorkers := 5
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			for item := range queue {
				processURL(item.url, item.depth, recurseLevel, queue, jobs,
					startDomain, mutex, outputMutex, visited, &activeWorkers, &activeJobs, verbose)
			}
		}(i)
	}

	numJobs := 5
	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func(jobId int) {
			defer wg.Done()
			for item := range jobs {
				processDownload(jobId, item,
					outputMutex, outputFolder, &activeJobs)
			}
		}(i)
	}
	done := make(chan struct{})
	go func() {
		atomic.AddInt32(&activeWorkers, 1)
		queue <- struct {
			url   string
			depth int
		}{startURL, 0}

		for {
			time.Sleep(100 * time.Millisecond) // Avoid tight loop
			if atomic.LoadInt32(&activeWorkers) == 0 && atomic.LoadInt32(&activeJobs) == 0 {
				fmt.Println("closing q and j")
				close(queue)
				close(jobs)
			}
			if verbose {
				outputMutex.Lock()
				fmt.Printf("stats:\n active link jobs %d || active downloads job %d\n Channel status: link_process %d || download_process %d\n", atomic.LoadInt32(&activeWorkers), atomic.LoadInt32(&activeJobs), len(queue), len(jobs))
				outputMutex.Unlock()
			}
		}
	}()
	select {
	case <-done:
		fmt.Println("All downloads completed successfully")
	case <-time.After(1 * time.Minute):
		fmt.Println("Spider timed out")
	}
	wg.Wait()
}

func processURL(pageURL string, depth, recurseLevel int,
	queue chan struct {
		url   string
		depth int
	},
	job chan struct {
		url   string
		depth int
	},
	startDomain *url.URL, mutex *sync.Mutex,
	outputMutex *sync.Mutex,
	visited map[string]bool, activeWorker, activeJob *int32,
	verbose bool,
) {
	defer atomic.AddInt32(activeWorker, -1)
	resp, err := http.Get(pageURL)
	if err != nil {
		fmt.Println("Error fetching page:", err)
		return
	}
	defer resp.Body.Close()

	buffer := []struct {
		url   string
		depth int
	}{}
	links := extractLinks(resp.Body, pageURL)
	for _, link := range links {
		linkURL, err := url.Parse(link)
		if err != nil || linkURL.Host != startDomain.Host || depth+1 > recurseLevel {
			continue
		}
		mutex.Lock()
		if visited[link] {
			mutex.Unlock()
			continue
		}
		visited[link] = true
		mutex.Unlock()

		buffer = append(buffer, struct {
			url   string
			depth int
		}{link, depth + 1})

		job <- struct {
			url   string
			depth int
		}{link, depth + 1}

		atomic.AddInt32(activeJob, 1)

		if verbose {
			outputMutex.Lock()
			fmt.Printf("New URL added link '%s'\n", link)
			outputMutex.Unlock()
		}
	}
	if verbose {
		outputMutex.Lock()
		fmt.Printf("Size of buffer %d'\n", len(buffer))
		outputMutex.Unlock()
	}
	for len(buffer) > 0 {
		select {
		case queue <- buffer[0]: // Add first item in buffer
			buffer = buffer[1:] // Remove from buffer
			atomic.AddInt32(activeWorker, 1)
		case <-time.After(250 * time.Millisecond):
			if verbose {
				outputMutex.Lock()
				fmt.Println("Queue full, retrying... Size of buffer", len(buffer))
				outputMutex.Unlock()
			}
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./spider [-rlp] URL")
		os.Exit(1)
	}
	url, recurseLevel, outputFolder, verbose, err := parser(os.Args[1:])
	if err != nil {
		ErrorExit(err)
	}
	fmt.Println("Spider is starting: depth-level", recurseLevel, "url", url, "outputFolder", outputFolder)
	crawl(url, recurseLevel, outputFolder, verbose)

	os.Exit(0)
}
