package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/drone/drone-go/drone"
	"github.com/drone/drone-go/plugin"
)

const maxConcurrent = 100

type job struct {
	local  string
	remote string
	action string
}

type result struct {
	j   job
	err error
}

var (
	buildCommit string
)

func main() {
	fmt.Printf("Drone S3 Sync Plugin built from %s\n", buildCommit)

	vargs := PluginArgs{}
	workspace := drone.Workspace{}

	plugin.Param("vargs", &vargs)
	plugin.Param("workspace", &workspace)
	if err := plugin.Parse(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if len(vargs.Key) == 0 || len(vargs.Secret) == 0 || len(vargs.Bucket) == 0 {
		return
	}

	if len(vargs.Region) == 0 {
		vargs.Region = "us-east-1"
	}

	if len(vargs.Source) == 0 {
		vargs.Source = "."
	}
	vargs.Source = filepath.Join(workspace.Path, vargs.Source)

	if strings.HasPrefix(vargs.Target, "/") {
		vargs.Target = vargs.Target[1:]
	}

	client := NewAWS(vargs)
	remote, err := client.List(vargs.Target)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	local := make([]string, 1, 1)
	jobs := make([]job, 1, 1)
	err = filepath.Walk(vargs.Source, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		localPath := path
		if vargs.Source != "." {
			localPath = strings.TrimPrefix(path, vargs.Source)
			if strings.HasPrefix(localPath, "/") {
				localPath = localPath[1:]
			}
		}
		local = append(local, localPath)
		jobs = append(jobs, job{
			local:  filepath.Join(vargs.Source, localPath),
			remote: filepath.Join(vargs.Target, localPath),
			action: "upload",
		})

		return nil
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for path, location := range vargs.Redirects {
		path = strings.TrimPrefix(path, "/")
		local = append(local, path)
		jobs = append(jobs, job{
			local:  path,
			remote: location,
			action: "redirect",
		})
	}

	for _, r := range remote {
		found := false
		for _, l := range local {
			if l == r {
				found = true
				break
			}
		}

		if !found {
			jobs = append(jobs, job{
				local:  "",
				remote: r,
				action: "delete",
			})
		}
	}

	jobChan := make(chan struct{}, maxConcurrent)
	results := make(chan *result, len(jobs))

	fmt.Printf("Synchronizing with bucket \"%s\"\n", vargs.Bucket)
	for _, j := range jobs {
		jobChan <- struct{}{}
		go func(j job) {
			if j.action == "upload" {
				err = client.Upload(j.local, j.remote)
			} else if j.action == "redirect" {
				err = client.Redirect(j.local, j.remote)
			} else if j.action == "delete" && vargs.Delete {
				err = client.Delete(j.remote)
			} else {
				err = nil
			}
			results <- &result{j, err}
			<-jobChan
		}(j)
	}

	for _ = range jobs {
		r := <-results
		if r.err != nil {
			fmt.Printf("ERROR: failed to %s %s to %s: %+v\n", r.j.action, r.j.local, r.j.remote, r.err)
			os.Exit(1)
		}
	}

	fmt.Println("done!")
}

func debug(format string, args ...interface{}) {
	if os.Getenv("DEBUG") != "" {
		fmt.Printf(format+"\n", args...)
	} else {
		fmt.Printf(".")
	}
}
