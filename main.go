package main

import (
	"errors"
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

type app struct {
	vargs     *PluginArgs
	workspace *drone.Workspace
	client    AWS
	jobs      []job
}

var (
	buildCommit string
)

var MissingAwsValuesMessage = "Must set access_key, secret_key, and bucket"

func main() {
	fmt.Printf("Drone S3 Sync Plugin built from %s\n", buildCommit)

	a := newApp()

	err := a.loadVargs()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = a.sanitizeInputs()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

    a.createClient()

	a.createSyncJobs()
	a.createInvalidateJob()
	a.runJobs()

	fmt.Println("done!")
}

func newApp() *app {
	return &app{
		vargs:     &PluginArgs{},
		workspace: &drone.Workspace{},
		jobs:      make([]job, 1, 1),
	}
}

func (a *app) loadVargs() error {
	plugin.Param("vargs", a.vargs)
	plugin.Param("workspace", a.workspace)
    
	err := plugin.Parse()
	return err

}

func (a *app) createClient() {
	a.client = NewAWS(*a.vargs)
}

func (a *app) sanitizeInputs() error {
	vargs := a.vargs
	workspace := a.workspace

	if len(a.vargs.Key) == 0 || len(a.vargs.Secret) == 0 || len(a.vargs.Bucket) == 0 {
		return errors.New(MissingAwsValuesMessage)
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

	return nil
}

func (a *app) createSyncJobs() {
	vargs := a.vargs
	remote, err := a.client.List(vargs.Target)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	local := make([]string, 1, 1)

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
		a.jobs = append(a.jobs, job{
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
		a.jobs = append(a.jobs, job{
			local:  path,
			remote: location,
			action: "redirect",
		})
	}
    if (a.vargs.Delete) {
        for _, r := range remote {
            found := false
            for _, l := range local {
                if l == r {
                    found = true
                    break
                }
            }

            if !found {
                a.jobs = append(a.jobs, job{
                    local:  "",
                    remote: r,
                    action: "delete",
                })
            }
        }
    }
}

func (a *app) createInvalidateJob() {
	if len(a.vargs.CloudFrontDistribution) > 0 {
		a.jobs = append(a.jobs, job{
			local:  "",
			remote: filepath.Join("/", a.vargs.Target, "*"),
			action: "invalidateCloudFront",
		})
	}
}

func (a *app) runJobs() {
	vargs := a.vargs
	client := a.client
	jobChan := make(chan struct{}, maxConcurrent)
	results := make(chan *result, len(a.jobs))

	fmt.Printf("Synchronizing with bucket \"%s\"\n", vargs.Bucket)
	for _, j := range a.jobs {
		jobChan <- struct{}{}
		go func(j job) {
			var err error
			if j.action == "upload" {
				err = client.Upload(j.local, j.remote)
			} else if j.action == "redirect" {
				err = client.Redirect(j.local, j.remote)
			} else if j.action == "delete" {
				err = client.Delete(j.remote)
			} else if j.action == "invalidateCloudFront" {
				client.Invalidate(j.remote)
			} else {
				err = nil
			}
			results <- &result{j, err}
			<-jobChan
		}(j)
	}

	for _ = range a.jobs {
		r := <-results
		if r.err != nil {
			fmt.Printf("ERROR: failed to %s %s to %s: %+v\n", r.j.action, r.j.local, r.j.remote, r.err)
			os.Exit(1)
		}
	}
}

func debug(format string, args ...interface{}) {
	if os.Getenv("DEBUG") != "" {
		fmt.Printf(format+"\n", args...)
	} else {
		fmt.Printf(".")
	}
}
