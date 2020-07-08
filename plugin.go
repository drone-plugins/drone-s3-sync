package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Plugin struct {
	Endpoint               string
	Key                    string
	Secret                 string
	Bucket                 string
	Region                 string
	Source                 string
	Target                 string
	Delete                 bool
	Access                 map[string]string
	CacheControl           map[string]string
	ContentType            map[string]string
	ContentEncoding        map[string]string
	Metadata               map[string]map[string]string
	Redirects              map[string]string
	CloudFrontDistribution string
	DryRun                 bool
	PathStyle              bool
	client                 AWS
	jobs                   []job
	MaxConcurrency         int
}

type job struct {
	local  string
	remote string
	action string
}

type result struct {
	j   job
	err error
}

var MissingAwsValuesMessage = "Must set 'bucket'"

func (p *Plugin) Exec() error {
	err := p.sanitizeInputs()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	p.jobs = make([]job, 1, 1)
	p.client = NewAWS(p)

	p.createSyncJobs()
	p.createInvalidateJob()
	p.runJobs()
	return nil
}

func (p *Plugin) sanitizeInputs() error {
	if len(p.Bucket) == 0 {
		return errors.New(MissingAwsValuesMessage)
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	p.Source = filepath.Join(wd, p.Source)
	p.Target = filepath.Join(p.Target)

	return nil
}

func (p *Plugin) createSyncJobs() {
	remote, err := p.client.List(p.Target)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	local := make([]string, 1, 1)

	err = filepath.Walk(p.Source, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		localPath := path
		if p.Source != "." {
			localPath = strings.TrimPrefix(path, p.Source)
			localPath = strings.TrimPrefix(localPath, string(os.PathSeparator))
		}
		local = append(local, localPath)
		p.jobs = append(p.jobs, job{
			local:  filepath.Join(p.Source, localPath),
			remote: filepath.Join(p.Target, localPath),
			action: "upload",
		})

		return nil
	})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for path, location := range p.Redirects {
		path = strings.TrimPrefix(path, string(os.PathSeparator))
		local = append(local, path)
		p.jobs = append(p.jobs, job{
			local:  path,
			remote: location,
			action: "redirect",
		})
	}
	if p.Delete {
		for _, r := range remote {
			found := false
			matcher := strings.TrimPrefix(r, p.Target)
			matcher = strings.TrimPrefix(matcher, string(os.PathSeparator))
			for _, l := range local {
				if l == matcher {
					found = true
					break
				}
			}

			if !found {
				p.jobs = append(p.jobs, job{
					local:  "",
					remote: r,
					action: "delete",
				})
			}
		}
	}
}

func (p *Plugin) createInvalidateJob() {
	if len(p.CloudFrontDistribution) > 0 {
		p.jobs = append(p.jobs, job{
			local:  "",
			remote: filepath.Join(string(os.PathSeparator), p.Target, "*"),
			action: "invalidateCloudFront",
		})
	}
}

func (p *Plugin) runJobs() {
	client := p.client
	jobChan := make(chan struct{}, p.MaxConcurrency)
	results := make(chan *result, len(p.jobs))
	var invalidateJob *job

	fmt.Printf("Synchronizing with bucket \"%s\"\n", p.Bucket)
	for _, j := range p.jobs {
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
				invalidateJob = &j
			} else {
				err = nil
			}
			results <- &result{j, err}
			<-jobChan
		}(j)
	}

	for range p.jobs {
		r := <-results
		if r.err != nil {
			fmt.Printf("ERROR: failed to %s %s to %s: %+v\n", r.j.action, r.j.local, r.j.remote, r.err)
			os.Exit(1)
		}
	}

	if invalidateJob != nil {
		err := client.Invalidate(invalidateJob.remote)
		if err != nil {
			fmt.Printf("ERROR: failed to %s %s to %s: %+v\n", invalidateJob.action, invalidateJob.local, invalidateJob.remote, err)
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
